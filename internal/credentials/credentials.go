// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

// Package credentials provides functionality to deal with operator credentials
package credentials

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

type FileReader interface {
	ReadFile(fileName string) ([]byte, error)
}

type Reader struct{}

func (f *Reader) ReadFile(fileName string) ([]byte, error) {
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

type Credentials struct {
	reader FileReader

	// /etc/credentials/credentials.json
	BMCUser     string `json:"bmcUser,omitempty"`
	BMCPassword string `json:"bmcPassword,omitempty"`
	NetboxToken string `json:"netboxToken,omitempty"`
}

func NewDefaultCredentials(fileReader FileReader) *Credentials {
	return &Credentials{
		reader:      fileReader,
		BMCUser:     "",
		BMCPassword: "",
		NetboxToken: "",
	}
}

func (c *Credentials) String() string {
	return fmt.Sprintf("bmcUser: %s, bmcPassword: ****, netboxToken: ****", c.BMCUser)
}

func (c *Credentials) Validate() error {
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

func (c *Credentials) Reload() error {
	if err := c.readJSONAndUnmarshal("/etc/credentials/credentials.json"); err != nil {
		return fmt.Errorf("unable to read credentials.json: %w", err)
	}
	return c.Validate()
}

func (c *Credentials) readJSONAndUnmarshal(fileName string) error {
	byteValue, err := c.reader.ReadFile(fileName)
	if err != nil {
		return err
	}

	if err = json.Unmarshal(byteValue, &c); err != nil {
		return err
	}

	return nil
}
