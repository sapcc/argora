// Copyright 2024 SAP SE
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"encoding/json"
	"errors"
	"io"
	"os"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Config struct {
	client client.Client

	// /etc/config/config.json
	IronCoreRoles        string `json:"ironCoreRoles"`
	IronCoreRegion       string `json:"ironCoreRegion"`
	IronCoreClusterTypes string `json:"ironCoreClusterTypes"`
	ServerController     string `json:"serverController"`
	K8sServiceHost       string `json:"k8sServiceHost"`

	// /etc/credentials/credentials.json
	NetboxUrl   string `json:"netboxUrl"`
	NetboxToken string `json:"netboxToken"`
	BMCUser     string `json:"bmcUsername"`
	BMCPassword string `json:"bmcPassword"`
}

func NewDefaultConfiguration(client client.Client) *Config {
	return &Config{client, "", "", "", "", "", "", "", "", ""}
}

func (c *Config) Validate() error {
	if c.IronCoreRoles == "" {
		return errors.New("ironcore roles are required")
	}
	if c.IronCoreRegion == "" {
		return errors.New("ironcore region is required")
	}
	if c.IronCoreClusterTypes == "" {
		return errors.New("ironcore cluster types are required")
	}
	if c.ServerController == "" {
		return errors.New("server controller name is required")
	}
	if c.K8sServiceHost == "" {
		return errors.New("k8s service host is required")
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
		return err
	}
	if err := c.readJsonAndUnmarshal("/etc/credentials/credentials.json"); err != nil {
		return err
	}
	return c.Validate()
}

func (c *Config) readJsonAndUnmarshal(fileName string) error {
	file, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer file.Close()

	byteValue, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	err = json.Unmarshal(byteValue, &c)
	if err != nil {
		return err
	}

	return nil
}
