// Copyright 2024 SAP SE
// SPDX-License-Identifier: Apache-2.0

// Package config provides functionality to deal with operator configuration
package config

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type FileReader interface {
	ReadFile(fileName string) ([]byte, error)
}

type ConfigReader struct{}

func (f *ConfigReader) ReadFile(fileName string) ([]byte, error) {
	file, err := os.Open(fileName) //nolint:gosec
	if err != nil {
		return nil, err
	}
	defer func() {
		err = file.Close()
	}()

	byteValue, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	return byteValue, err
}

type Config struct {
	client client.Client
	reader FileReader

	// /etc/config/config.json
	IronCoreRoles    string `json:"ironCoreRoles"`
	IronCoreRegion   string `json:"ironCoreRegion"`
	ServerController string `json:"serverController"`

	// /etc/credentials/credentials.json
	NetboxURL   string `json:"netboxURL"`
	NetboxToken string `json:"netboxToken"`
	BMCUser     string `json:"bmcUser"`
	BMCPassword string `json:"bmcPassword"`
}

func NewDefaultConfiguration(client client.Client, configReader FileReader) *Config {
	return &Config{client, configReader, "", "", "", "", "", "", ""}
}

func (c *Config) Validate() error {
	if c.IronCoreRoles == "" {
		return errors.New("ironcore roles are required")
	}
	if c.IronCoreRegion == "" {
		return errors.New("ironcore region is required")
	}
	if c.ServerController == "" {
		return errors.New("server controller name is required")
	}
	if c.NetboxURL == "" {
		return errors.New("netbox URL is required")
	}
	if c.NetboxToken == "" {
		return errors.New("netbox token is required")
	}
	if c.BMCUser == "" {
		return errors.New("bmc user is required")
	}
	if c.BMCPassword == "" {
		return errors.New("bmc password is required")
	}
	return nil
}

func (c *Config) Reload() error {
	if err := c.readJSONAndUnmarshal("/etc/config/config.json"); err != nil {
		return fmt.Errorf("unable to read config.json: %w", err)
	}
	if err := c.readJSONAndUnmarshal("/etc/credentials/credentials.json"); err != nil {
		return fmt.Errorf("unable to read credentials.json: %w", err)
	} else {
		if c.NetboxURL, err = decodeBase64(c.NetboxURL); err != nil {
			return err
		}

		if c.NetboxToken, err = decodeBase64(c.NetboxToken); err != nil {
			return err
		}

		if c.BMCUser, err = decodeBase64(c.BMCUser); err != nil {
			return err
		}

		if c.BMCPassword, err = decodeBase64(c.BMCPassword); err != nil {
			return err
		}
	}
	return c.Validate()
}

func (c *Config) readJSONAndUnmarshal(fileName string) error {
	byteValue, err := c.reader.ReadFile(fileName)
	if err != nil {
		return err
	}

	if err = json.Unmarshal(byteValue, &c); err != nil {
		return err
	}

	return nil
}

func decodeBase64(message string) (string, error) {
	bytes, err := base64.StdEncoding.DecodeString(message)
	return string(bytes), err
}
