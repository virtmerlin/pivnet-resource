package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/pivotal-cf-experimental/pivnet-resource/concourse"
	"github.com/pivotal-cf-experimental/pivnet-resource/downloader"
	"github.com/pivotal-cf-experimental/pivnet-resource/filter"
	"github.com/pivotal-cf-experimental/pivnet-resource/logger"
	"github.com/pivotal-cf-experimental/pivnet-resource/pivnet"
	"github.com/pivotal-cf-experimental/pivnet-resource/sanitizer"
)

const (
	url = "https://network.pivotal.io/api/v2"
)

func main() {
	var input concourse.CheckRequest
	if len(os.Args) < 2 {
		panic("Not enough args")
	}

	downloadDir := os.Args[1]

	err := json.NewDecoder(os.Stdin).Decode(&input)
	if err != nil {
		log.Fatalln(err)
	}

	sanitized := make(map[string]string)
	logFile, err := ioutil.TempFile("", "pivnet-resource-in.log")
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Fprintf(os.Stderr, "logging to %s\n", logFile.Name())
	sanitizer := sanitizer.NewSanitizer(sanitized, logFile)
	logger := logger.NewLogger(sanitizer)

	token := input.Source.APIToken
	mustBeNonEmpty(token, "api_token")
	sanitized[input.Source.APIToken] = "***REDACTED-PIVNET_API_TOKEN***"

	client := pivnet.NewClient(
		url,
		token,
		logger,
	)
	if err != nil {
		log.Fatalf("Failed to create client: %s", err)
	}

	productVersion := input.Version.ProductVersion

	release, err := client.GetRelease(input.Source.ProductName, productVersion)
	if err != nil {
		log.Fatalf("Failed to get Release: %s", err)
	}

	productFiles, err := client.GetProductFiles(release)
	if err != nil {
		log.Fatalf("Failed to get Product Files: %s", err)
	}

	downloadLinks := filter.DownloadLinks(productFiles)

	err = downloader.Download(downloadDir, downloadLinks, token)
	if err != nil {
		log.Fatalf("Failed to Download Files: %s", err)
	}

	versionFilepath := filepath.Join(downloadDir, "version")

	err = ioutil.WriteFile(versionFilepath, []byte(productVersion), os.ModePerm)
	if err != nil {
		log.Fatalln(err)
	}

	out := concourse.InResponse{
		Version: concourse.Version{
			ProductVersion: productVersion,
		},
		Metadata: []concourse.Metadata{},
	}

	err = json.NewEncoder(os.Stdout).Encode(out)
	if err != nil {
		log.Fatalln(err)
	}
}

func mustBeNonEmpty(input string, key string) {
	if input == "" {
		log.Fatalf("%s must be provided\n", key)
	}
}
