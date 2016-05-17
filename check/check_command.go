package check

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pivotal-cf-experimental/pivnet-resource/concourse"
	"github.com/pivotal-cf-experimental/pivnet-resource/filter"
	"github.com/pivotal-cf-experimental/pivnet-resource/gp"
	"github.com/pivotal-cf-experimental/pivnet-resource/logger"
	"github.com/pivotal-cf-experimental/pivnet-resource/versions"
)

type CheckCommand struct {
	logger         logger.Logger
	logFilePath    string
	binaryVersion  string
	filter         filter.Filter
	pivnetClient   gp.Client
	extendedClient gp.ExtendedClient
}

func NewCheckCommand(
	binaryVersion string,
	logger logger.Logger,
	logFilePath string,
	filter filter.Filter,
	pivnetClient gp.Client,
	extendedClient gp.ExtendedClient,
) *CheckCommand {
	return &CheckCommand{
		logger:         logger,
		logFilePath:    logFilePath,
		binaryVersion:  binaryVersion,
		filter:         filter,
		pivnetClient:   pivnetClient,
		extendedClient: extendedClient,
	}
}

func (c *CheckCommand) Run(input concourse.CheckRequest) (concourse.CheckResponse, error) {
	logDir := filepath.Dir(c.logFilePath)
	existingLogFiles, err := filepath.Glob(filepath.Join(logDir, "pivnet-resource-check.log*"))
	if err != nil {
		// This is untested because the only error returned by filepath.Glob is a
		// malformed glob, and this glob is hard-coded to be correct.
		return nil, err
	}

	for _, f := range existingLogFiles {
		if filepath.Base(f) != filepath.Base(c.logFilePath) {
			c.logger.Debugf("Removing existing log file: %s\n", f)
			err := os.Remove(f)
			if err != nil {
				// This is untested because it is too hard to force os.Remove to return
				// an error.
				return nil, err
			}
		}
	}

	c.logger.Debugf("Received input: %+v\n", input)

	c.logger.Debugf("Getting all valid release types\n")
	releaseTypes, err := c.pivnetClient.ReleaseTypes()
	if err != nil {
		return nil, err
	}

	releaseTypesPrintable := fmt.Sprintf(
		"['%s']",
		strings.Join(releaseTypes, "', '"),
	)

	c.logger.Debugf("All release types: %s\n", releaseTypesPrintable)

	releaseType := input.Source.ReleaseType
	if releaseType != "" && !containsString(releaseTypes, releaseType) {
		return nil, fmt.Errorf(
			"provided release_type: '%s' must be one of: %s",
			releaseType,
			releaseTypesPrintable,
		)
	}

	c.logger.Debugf("Getting all product versions\n")
	productSlug := input.Source.ProductSlug

	releases, err := c.pivnetClient.ReleasesForProductSlug(productSlug)
	if err != nil {
		return nil, err
	}

	if releaseType != "" {
		c.logger.Debugf("Filtering all releases by release_type: %s\n", releaseType)

		releases, err = c.filter.ReleasesByReleaseType(releases, releaseType)
		if err != nil {
			return nil, err
		}
	}

	productVersion := input.Source.ProductVersion
	if productVersion != "" {
		c.logger.Debugf("Filtering all releases by product_version: %s\n", productVersion)

		releases, err = c.filter.ReleasesByVersion(releases, productVersion)
		if err != nil {
			return nil, err
		}
	}

	// ls := lagershim.NewLagerShim(c.logger)
	// extendedClient := extension.NewExtendedClient(c.pivnetClient, ls)
	filteredVersions, err := versions.ProductVersions(c.extendedClient, productSlug, releases)
	if err != nil {
		return nil, err
	}

	c.logger.Debugf("Filtered versions: %+v\n", filteredVersions)

	if len(filteredVersions) == 0 {
		return concourse.CheckResponse{}, nil
	}

	newVersions, err := versions.Since(filteredVersions, input.Version.ProductVersion)
	if err != nil {
		// Untested because versions.Since cannot be forced to return an error.
		return nil, err
	}

	c.logger.Debugf("New versions: %+v\n", newVersions)

	reversedVersions, err := versions.Reverse(newVersions)
	if err != nil {
		// Untested because versions.Since cannot be forced to return an error.
		return nil, err
	}

	var out concourse.CheckResponse
	for _, v := range reversedVersions {
		out = append(out, concourse.Version{ProductVersion: v})
	}

	if len(out) == 0 {
		out = append(out, concourse.Version{ProductVersion: filteredVersions[0]})
	}

	c.logger.Debugf("Returning output: %+v\n", out)

	return out, nil
}

func containsString(strings []string, str string) bool {
	for _, s := range strings {
		if str == s {
			return true
		}
	}
	return false
}
