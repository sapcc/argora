// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Config Suite")
}

var _ = Describe("ControllerType", func() {
	Context("String method", func() {
		It("should return 'ironcore' for ControllerTypeIroncore", func() {
			// given
			controllerType := ControllerTypeIroncore

			// when
			result := controllerType.String()

			// then
			Expect(result).To(Equal("ironcore"))
		})

		It("should return 'metal3' for ControllerTypeMetal3", func() {
			// given
			controllerType := ControllerTypeMetal3

			// when
			result := controllerType.String()

			// then
			Expect(result).To(Equal("metal3"))
		})

		It("should return 'unsupported' for an unimplemented ControllerType", func() {
			// given
			controllerType := ControllerType("_")

			// when
			result := controllerType.String()

			// when/then
			Expect(result).To(Equal("unsupported"))
		})
	})
})

var _ = Describe("Config", func() {
	var cfg *Config

	BeforeEach(func() {
		client := fake.NewClientBuilder().Build()
		cfg = NewDefaultConfiguration(client, &ConfigReader{})

		cfg.ServerController = "controller1"
		cfg.IronCore = []IronCore{
			{
				Name:   "name1",
				Region: "region1",
				Type:   "type1",
			},
		}
		cfg.NetboxURL = "http://netbox"

		cfg.BMCUser = "user"
		cfg.BMCPassword = "password"
		cfg.NetboxToken = "token"
	})

	Describe("Validate", func() {
		Context("should succeed when all fields are valid", func() {
			It("should not return an error", func() {
				// when
				err := cfg.Validate()

				// then
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("should not return an error when ServerController is empty", func() {
			It("should return an error", func() {
				// given
				cfg.ServerController = ""

				// when
				err := cfg.Validate()

				// then
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("should return an error when serverController is ironcore and configuration is missing", func() {
			It("should return an error", func() {
				// given
				cfg.ServerController = ControllerTypeIroncore
				cfg.IronCore = []IronCore{
					{
						Name:   "",
						Region: "",
						Type:   "",
					},
				}

				// when
				err := cfg.Validate()

				// then
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("ironcore configuration is required"))
			})
		})

		Context("should succeed when serverController is metal3 and configuration is missing", func() {
			It("should return an error", func() {
				// given
				cfg.ServerController = ControllerTypeMetal3
				cfg.IronCore = []IronCore{
					{
						Name:   "",
						Region: "",
						Type:   "",
					},
				}

				// when
				err := cfg.Validate()

				// then
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("should return an error when NetboxURL is empty", func() {
			It("should return an error", func() {
				// given
				cfg.NetboxURL = ""

				// when
				err := cfg.Validate()

				// then
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("netbox URL is required"))
			})
		})

		Context("should not return an error when BMCUser and BMCPassword are empty and serverController neither IronCore nor Metal3", func() {
			It("should return an error", func() {
				// given
				cfg.ServerController = ""
				cfg.BMCUser = ""
				cfg.BMCPassword = ""

				// when
				err := cfg.Validate()

				// then
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("should return an error when BMCUser is empty if serverController is IronCore", func() {
			It("should return an error", func() {
				// given
				cfg.ServerController = ControllerTypeIroncore
				cfg.BMCUser = ""

				// when
				err := cfg.Validate()

				// then
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("bmc user is required"))
			})
		})

		Context("should return an error when BMCPassword is empty if serverController is Metal3", func() {
			It("should return an error", func() {
				// given
				cfg.ServerController = ControllerTypeMetal3
				cfg.BMCPassword = ""

				// when
				err := cfg.Validate()

				// then
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("bmc password is required"))
			})
		})

		Context("should return an error when NetboxToken is empty", func() {
			It("should return an error", func() {
				// given
				cfg.NetboxToken = ""

				// when
				err := cfg.Validate()

				// then
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("netbox token is required"))
			})
		})
	})
})

type FileReaderMock struct {
	fileContent map[string]string
	returnError bool
}

func (f *FileReaderMock) ReadFile(fileName string) ([]byte, error) {
	if f.returnError {
		return nil, errors.New("error")
	}
	return []byte(f.fileContent[fileName]), nil
}

var _ = Describe("Reload", func() {
	var cfg *Config
	var fileReaderMock *FileReaderMock

	BeforeEach(func() {
		client := fake.NewClientBuilder().Build()
		fileReaderMock = &FileReaderMock{
			fileContent: make(map[string]string),
			returnError: false,
		}
		cfg = NewDefaultConfiguration(client, fileReaderMock)
	})

	Context("when all fields are valid", func() {
		It("should not return an error", func() {
			// given
			configJson := `{
				"serverController": "ironcore",
				"ironCore": [{
				  "name": "name1",
				  "region": "region1",
					"type": "type1"
				}],
				"netboxUrl": "http://netbox"
			}`
			credentialsJson := `{
				"bmcUser": "user",
				"bmcPassword": "password",
				"netboxToken": "token"
			}`

			fileReaderMock.fileContent["/etc/config/config.json"] = configJson
			fileReaderMock.fileContent["/etc/credentials/credentials.json"] = credentialsJson

			// when
			err := cfg.Reload()

			// then
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg.ServerController).To(Equal(ControllerTypeIroncore))
			Expect(cfg.IronCore).To(ConsistOf(IronCore{Name: "name1", Region: "region1", Type: "type1"}))
			Expect(cfg.NetboxURL).To(Equal("http://netbox"))

			Expect(cfg.BMCUser).To(Equal("user"))
			Expect(cfg.BMCPassword).To(Equal("password"))
			Expect(cfg.NetboxToken).To(Equal("token"))
		})

		It("should not return an error if ironcore config is empty when serverController is metal3", func() {
			// given
			configJson := `{
				"serverController": "metal3",
				"ironCore": [],
				"netboxUrl": "http://netbox"
			}`
			credentialsJson := `{
				"bmcUser": "user",
				"bmcPassword": "password",
				"netboxToken": "token"
			}`

			fileReaderMock.fileContent["/etc/config/config.json"] = configJson
			fileReaderMock.fileContent["/etc/credentials/credentials.json"] = credentialsJson

			// when
			err := cfg.Reload()

			// then
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg.ServerController).To(Equal(ControllerTypeMetal3))
			Expect(cfg.IronCore).To(BeEmpty())
		})

		It("should return an error if ironcore config is empty and serverController is ironcore", func() {
			// given
			configJson := `{
				"serverController": "ironcore",
				"ironCore": [],
				"netboxUrl": "http://netbox"
			}`
			credentialsJson := `{
				"bmcUser": "user",
				"bmcPassword": "password",
				"netboxToken": "token"
			}`

			fileReaderMock.fileContent["/etc/config/config.json"] = configJson
			fileReaderMock.fileContent["/etc/credentials/credentials.json"] = credentialsJson

			// when
			err := cfg.Reload()

			// then
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("ironcore configuration is required"))
		})

		It("should return an error if ironcore config has one entry with empty fields and serverController is ironcore", func() {
			// given
			configJson := `{
				"serverController": "ironcore",
				"ironCore": [{
					"name": "",
					"region": "",
					"type": ""
				}],
				"netboxUrl": "http://netbox"
			}`
			credentialsJson := `{
				"bmcUser": "user",
				"bmcPassword": "password",
				"netboxToken": "token"
			}`

			fileReaderMock.fileContent["/etc/config/config.json"] = configJson
			fileReaderMock.fileContent["/etc/credentials/credentials.json"] = credentialsJson

			// when
			err := cfg.Reload()

			// then
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("ironcore configuration is required"))
		})
	})

	Context("when config file is missing", func() {
		It("should return an error", func() {
			// given
			credentialsJson := `{
				"bmcUser": "user",
				"bmcPassword": "password",
				"netboxToken": "token"
			}`

			fileReaderMock.fileContent["/etc/credentials/credentials.json"] = credentialsJson

			// when
			err := cfg.Reload()

			// then
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unable to read config.json: unexpected end of JSON input"))
		})
	})

	Context("when credentials file is missing", func() {
		It("should return an error", func() {
			// given
			configJson := `{
				"serverController": "controller1",
				"ironCore": [{
					"name": "name1",
					"region": "region1",
				  "type": "type1"
				}],
				"netboxUrl": "http://netbox"
			}`

			fileReaderMock.fileContent["/etc/config/config.json"] = configJson

			// when
			err := cfg.Reload()

			// then
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unable to read credentials.json: unexpected end of JSON input"))
		})
	})

	Context("when config file contains invalid JSON", func() {
		It("should return an error", func() {
			// given
			configJson := `a`
			credentialsJson := `{
				"bmcUser": "user",
				"bmcPassword": "password",
				"netboxToken": "token"
			}`

			fileReaderMock.fileContent["/etc/config/config.json"] = configJson
			fileReaderMock.fileContent["/etc/credentials/credentials.json"] = credentialsJson

			// when
			err := cfg.Reload()

			// then
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unable to read config.json: invalid character 'a' looking for beginning of value"))
		})
	})

	Context("when credentials file contains invalid JSON", func() {
		It("should return an error", func() {
			// given
			configJson := `{
				"serverController": "controller1",
				"ironCore": [{
				  "name": "name1",
				  "region": "region1",
					"type": "type1"
				}],
				"netboxUrl": "http://netbox"
			}`
			credentialsJson := `b`

			fileReaderMock.fileContent["/etc/config/config.json"] = configJson
			fileReaderMock.fileContent["/etc/credentials/credentials.json"] = credentialsJson
			// when
			err := cfg.Reload()

			// then
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unable to read credentials.json: invalid character 'b' looking for beginning of value"))
		})
	})

	Context("when reading file fails", func() {
		It("should return an error", func() {
			// given
			fileReaderMock.returnError = true

			// when
			err := cfg.Reload()

			// then
			Expect(err).To(HaveOccurred())
		})
	})
})

var _ = Describe("readJSONAndUnmarshal", func() {
	var cfg *Config
	var fileReaderMock *FileReaderMock

	BeforeEach(func() {
		client := fake.NewClientBuilder().Build()
		fileReaderMock = &FileReaderMock{
			fileContent: make(map[string]string),
			returnError: false,
		}
		cfg = NewDefaultConfiguration(client, fileReaderMock)
	})

	Context("when the file is read successfully", func() {
		It("should unmarshal the JSON content", func() {
			// given
			configJsonContent := `{
				"serverController": "ironcore",
				"ironCore": [{
				  "name": "name1",
				  "region": "region1",
					"type": "type1"
				}],
				"netboxUrl": "http://netbox"
			}`
			credentialsJsonContent := `{
				"bmcUser": "user",
				"bmcPassword": "password",
				"netboxToken": "token"
			}`
			fileReaderMock.fileContent["/etc/config/config.json"] = configJsonContent
			fileReaderMock.fileContent["/etc/config/credentials.json"] = credentialsJsonContent

			// when
			err := cfg.readJSONAndUnmarshal("/etc/config/config.json")

			// then
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg.ServerController).To(Equal(ControllerTypeIroncore))
			Expect(cfg.IronCore).To(HaveLen(1))
			Expect(cfg.IronCore[0].Name).To(Equal("name1"))
			Expect(cfg.IronCore[0].Region).To(Equal("region1"))
			Expect(cfg.IronCore[0].Type).To(Equal("type1"))
			Expect(cfg.NetboxURL).To(Equal("http://netbox"))

			// when
			err = cfg.readJSONAndUnmarshal("/etc/config/credentials.json")

			// then
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg.BMCUser).To(Equal("user"))
			Expect(cfg.BMCPassword).To(Equal("password"))
			Expect(cfg.NetboxToken).To(Equal("token"))
		})
	})

	It("should unmarshal the config.json content when ironcore config is missing", func() {
		// given
		configJsonContent := `{
			"serverController": "metal3",
			"ironCore": [],
			"netboxUrl": "http://netbox"
		}`
		fileReaderMock.fileContent["/etc/config/config.json"] = configJsonContent

		// when
		err := cfg.readJSONAndUnmarshal("/etc/config/config.json")

		// then
		Expect(err).ToNot(HaveOccurred())
		Expect(cfg.ServerController).To(Equal(ControllerTypeMetal3))
		Expect(cfg.IronCore).To(BeEmpty())
		Expect(cfg.NetboxURL).To(Equal("http://netbox"))
	})

	It("should unmarshal the config.json content when ironcore config is missing #2", func() {
		// given
		configJsonContent := `{
			"serverController": "metal3",
			"ironCore": [{
				"name": "",
				"region": "",
				"type": ""
			}],
			"netboxUrl": "http://netbox"
		}`
		fileReaderMock.fileContent["/etc/config/config.json"] = configJsonContent

		// when
		err := cfg.readJSONAndUnmarshal("/etc/config/config.json")

		// then
		Expect(err).ToNot(HaveOccurred())
		Expect(cfg.ServerController).To(Equal(ControllerTypeMetal3))
		Expect(cfg.IronCore).To(HaveLen(1))
		Expect(cfg.IronCore[0].Name).To(BeEmpty())
		Expect(cfg.IronCore[0].Region).To(BeEmpty())
		Expect(cfg.IronCore[0].Type).To(BeEmpty())
		Expect(cfg.NetboxURL).To(Equal("http://netbox"))
	})

	Context("when the file cannot be read", func() {
		It("should return an error", func() {
			// given
			fileReaderMock.returnError = true

			// when
			err := cfg.readJSONAndUnmarshal("/etc/config/config.json")

			// then
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when the config.json is invalid", func() {
		It("should return an error", func() {
			// given
			invalidJsonContent := `invalid json`
			fileReaderMock.fileContent["/etc/config/config.json"] = invalidJsonContent

			// when
			err := cfg.readJSONAndUnmarshal("/etc/config/config.json")

			// then
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("invalid character 'i' looking for beginning of value"))
		})
	})

	Context("when the credentials.json is invalid", func() {
		It("should return an error", func() {
			// given
			invalidJsonContent := `invalid json`
			fileReaderMock.fileContent["/etc/config/credentials.json"] = invalidJsonContent

			// when
			err := cfg.readJSONAndUnmarshal("/etc/config/credentials.json")

			// then
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("invalid character 'i' looking for beginning of value"))
		})
	})
})
