// Copyright 2024 SAP SE
// SPDX-License-Identifier: Apache-2.0

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
	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	byteValue, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	return byteValue, nil
}

type Config struct {
	client client.Client
	reader FileReader

	// /etc/config/config.json
	IronCoreRoles    string `json:"ironCoreRoles"`
	IronCoreRegion   string `json:"ironCoreRegion"`
	ServerController string `json:"serverController"`

	// /etc/credentials/credentials.json
	NetboxUrl   string `json:"netboxUrl"`
	NetboxToken string `json:"netboxToken"`
	BMCUser     string `json:"bmcUsername"`
	BMCPassword string `json:"bmcPassword"`
}

func NewDefaultConfiguration(client client.Client) *Config {
	return &Config{client, &ConfigReader{}, "", "", "", "", "", "", ""}
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
	if c.NetboxUrl == "" {
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
	if err := c.readJsonAndUnmarshal("/etc/config/config.json"); err != nil {
		return fmt.Errorf("unable to read config.json: %w", err)
	}
	if err := c.readJsonAndUnmarshal("/etc/credentials/credentials.json"); err != nil {
		return fmt.Errorf("unable to read credentials.json: %w", err)
	}
	return c.Validate()
}

func (c *Config) readJsonAndUnmarshal(fileName string) error {
	byteValue, err := c.reader.ReadFile(fileName)
	if err != nil {
		return err
	}

	if err = json.Unmarshal(byteValue, &c); err != nil {
		return err
	}

	if c.NetboxUrl, err = decodeBase64(c.NetboxUrl); err != nil {
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

	return nil
}

func decodeBase64(message string) (string, error) {
	bytes, err := base64.StdEncoding.DecodeString(message)
	return string(bytes), err
}
