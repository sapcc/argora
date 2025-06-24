// Copyright 2024 SAP SE
// SPDX-License-Identifier: Apache-2.0

// Package config provides functionality to deal with operator configuration
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ControllerType string

const (
	ControllerTypeIroncore ControllerType = "ironcore"
	ControllerTypeMetal3   ControllerType = "metal3"
)

func (c ControllerType) String() string {
	switch c {
	case ControllerTypeIroncore:
		return "ironcore"
	case ControllerTypeMetal3:
		return "metal3"
	}
	panic(fmt.Errorf("%#v has unimplemented String() method", c))
}

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
	k8sClient client.Client
	reader    FileReader

	// /etc/config/config.json
	ServerController ControllerType `json:"serverController"`
	IronCore         IronCore       `json:"ironCore"`
	NetboxURL        string         `json:"netboxURL"`

	// /etc/credentials/credentials.json
	BMCUser     string `json:"bmcUser"`
	BMCPassword string `json:"bmcPassword"`
	NetboxToken string `json:"netboxToken"`
}

type IronCore struct {
	Names  string `json:"name"`
	Region string `json:"region"`
	Types  string `json:"types"`
}

func NewDefaultConfiguration(k8sClient client.Client, configReader FileReader) *Config {
	return &Config{
		k8sClient:        k8sClient,
		reader:           configReader,
		ServerController: "",
		IronCore:         IronCore{},
		NetboxURL:        "",
		BMCUser:          "",
		BMCPassword:      "",
		NetboxToken:      "",
	}
}

func (c *Config) String() string {
	return fmt.Sprintf("ironCore.Names:%s,ironCore.Region:%s,ironCore.Types:%s,serverController:%s,netboxURL:%s", c.IronCore.Names, c.IronCore.Region, c.IronCore.Types, c.ServerController, c.NetboxURL)
}

func (c *Config) Validate() error {
	// /etc/config/config.json
	if c.ServerController == "" {
		return errors.New("server controller name is required")
	}
	if c.ServerController == ControllerTypeIroncore && c.IronCore.Names == "" && c.IronCore.Region == "" && c.IronCore.Types == "" {
		return errors.New("ironcore configuration is required")
	}
	if c.NetboxURL == "" {
		return errors.New("netbox URL is required")
	}
	// /etc/credentials/credentials.json
	if c.BMCUser == "" {
		return errors.New("bmc user is required")
	}
	if c.BMCPassword == "" {
		return errors.New("bmc password is required")
	}
	if c.NetboxToken == "" {
		return errors.New("netbox token is required")
	}
	return nil
}

func (c *Config) Reload() error {
	if err := c.readJSONAndUnmarshal("/etc/config/config.json"); err != nil {
		return fmt.Errorf("unable to read config.json: %w", err)
	}
	if err := c.readJSONAndUnmarshal("/etc/credentials/credentials.json"); err != nil {
		return fmt.Errorf("unable to read credentials.json: %w", err)
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
