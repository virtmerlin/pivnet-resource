package s3

import (
	"encoding/json"
	"fmt"
	"io"
	"os/exec"

	"github.com/pivotal-golang/lager"
)

type Client interface {
	Upload(fileGlob string, to string, sourcesDir string) error
}

type client struct {
	accessKeyID     string
	secretAccessKey string
	regionName      string
	bucket          string

	logger lager.Logger

	stdout io.Writer
	stderr io.Writer

	outBinaryPath string
}

type NewClientConfig struct {
	AccessKeyID     string
	SecretAccessKey string
	RegionName      string
	Bucket          string

	Logger lager.Logger

	Stdout io.Writer
	Stderr io.Writer

	OutBinaryPath string
}

func NewClient(config NewClientConfig) Client {
	return &client{
		accessKeyID:     config.AccessKeyID,
		secretAccessKey: config.SecretAccessKey,
		regionName:      config.RegionName,
		bucket:          config.Bucket,
		stdout:          config.Stdout,
		stderr:          config.Stderr,
		outBinaryPath:   config.OutBinaryPath,
		logger:          config.Logger,
	}
}

func (c client) Upload(fileGlob string, to string, sourcesDir string) error {
	s3Input := Request{
		Source: Source{
			AccessKeyID:     c.accessKeyID,
			SecretAccessKey: c.secretAccessKey,
			Bucket:          c.bucket,
			RegionName:      c.regionName,
		},
		Params: Params{
			File: fileGlob,
			To:   to,
		},
	}

	c.logger.Debug("Input to s3out", lager.Data{"input": s3Input, "sources dir": sourcesDir})

	cmd := exec.Command(c.outBinaryPath, sourcesDir)

	cmdIn, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	cmd.Stdout = c.stderr
	cmd.Stderr = c.stderr

	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("Error starting %s: %s", c.outBinaryPath, err.Error())
	}

	err = json.NewEncoder(cmdIn).Encode(s3Input)
	if err != nil {
		return err
	}

	err = cmd.Wait()
	if err != nil {
		return fmt.Errorf("Error running %s: %s", c.outBinaryPath, err.Error())
	}

	return nil
}
