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

var _ = Describe("Config", func() {
	var cfg *Config

	BeforeEach(func() {
		client := fake.NewClientBuilder().Build()
		cfg = NewDefaultConfiguration(client, &ConfigReader{})

		cfg.IronCoreRoles = "role1"
		cfg.IronCoreRegion = "region1"
		cfg.ServerController = "controller1"
		cfg.NetboxUrl = "aHR0cDovL25ldGJveA=="
		cfg.NetboxToken = "InRva2VuIg=="
		cfg.BMCUser = "dXNlcg=="
		cfg.BMCPassword = "cGFzc3dvcmQ="
	})

	Describe("Validate", func() {
		Context("when all fields are valid", func() {
			It("should not return an error", func() {
				// when
				err := cfg.Validate()

				// then
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when IronCoreRoles is empty", func() {
			It("should return an error", func() {
				// given
				cfg.IronCoreRoles = ""

				// when
				err := cfg.Validate()

				// then
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("ironcore roles are required"))
			})
		})

		Context("when IronCoreRegion is empty", func() {
			It("should return an error", func() {
				// given
				cfg.IronCoreRegion = ""

				// when
				err := cfg.Validate()

				// then
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("ironcore region is required"))
			})
		})

		Context("when ServerController is empty", func() {
			It("should return an error", func() {
				// given
				cfg.ServerController = ""

				// when
				err := cfg.Validate()

				// then
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("server controller name is required"))
			})
		})

		Context("when NetboxUrl is empty", func() {
			It("should return an error", func() {
				// given
				cfg.NetboxUrl = ""

				// when
				err := cfg.Validate()

				// then
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("netbox URL is required"))
			})
		})

		Context("when NetboxToken is empty", func() {
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

		Context("when BMCUser is empty", func() {
			It("should return an error", func() {
				// given
				cfg.BMCUser = ""

				// when
				err := cfg.Validate()

				// then
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("bmc user is required"))
			})
		})

		Context("when BMCPassword is empty", func() {
			It("should return an error", func() {
				// given
				cfg.BMCPassword = ""

				// when
				err := cfg.Validate()

				// then
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("bmc password is required"))
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
		cfg = &Config{client, fileReaderMock, "", "", "", "", "", "", ""}
	})

	Context("when all fields are valid", func() {
		It("should not return an error", func() {
			// given
			configJson := `{
				"ironCoreRoles": "role1",
				"ironCoreRegion": "region1",
				"serverController": "controller1"
			}`
			credentialsJson := `{
				"netboxUrl": "aHR0cDovL25ldGJveA==",
				"netboxToken": "dG9rZW4=",
				"bmcUser": "dXNlcg==",
				"bmcPassword": "cGFzc3dvcmQ="
			}`

			fileReaderMock.fileContent["/etc/config/config.json"] = configJson
			fileReaderMock.fileContent["/etc/credentials/credentials.json"] = credentialsJson

			// when
			err := cfg.Reload()

			// then
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg.IronCoreRoles).To(Equal("role1"))
			Expect(cfg.IronCoreRegion).To(Equal("region1"))
			Expect(cfg.ServerController).To(Equal("controller1"))
			Expect(cfg.NetboxUrl).To(Equal("http://netbox"))
			Expect(cfg.NetboxToken).To(Equal("token"))
			Expect(cfg.BMCUser).To(Equal("user"))
			Expect(cfg.BMCPassword).To(Equal("password"))
		})
	})

	Context("when config file is missing", func() {
		It("should return an error", func() {
			// given
			credentialsJson := `{
				"netboxUrl": "aHR0cDovL25ldGJveA==",
				"netboxToken": "dG9rZW4=",
				"bmcUser": "user",
				"bmcPassword": "cGFzsword	}`

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
				"ironCoreRoles": "role1",
				"ironCoreRegion": "region1",
				"serverController": "controller1"
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
				"netboxUrl": "aHR0cDovL25ldGJveA==",
				"netboxToken": "InRva2VuIg==",
				"bmcUser": "dXNlcg==",
				"bmcPassword": "cGFzc3dvcmQ="
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
				"ironCoreRoles": "role1",
				"ironCoreRegion": "region1",
				"serverController": "controller1"
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

	Context("when NetboxUrl is not base64 encoded", func() {
		It("should return an error", func() {
			// given
			configJson := `{
				"ironCoreRoles": "role1",
				"ironCoreRegion": "region1",
				"serverController": "controller1"
			}`
			credentialsJson := `{
				"netboxUrl": "not_base64",
				"netboxToken": "dG9rZW4=",
				"bmcUser": "dXNlcg==",
				"bmcPassword": "cGFzc3dvcmQ="
			}`
			fileReaderMock.fileContent["/etc/config/config.json"] = configJson
			fileReaderMock.fileContent["/etc/credentials/credentials.json"] = credentialsJson

			// when
			err := cfg.Reload()

			// then
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("illegal base64 data at input byte 3"))
		})
	})

	Context("when NetboxToken is not base64 encoded", func() {
		It("should return an error", func() {
			// given
			configJson := `{
				"ironCoreRoles": "role1",
				"ironCoreRegion": "region1",
				"serverController": "controller1"
			}`
			credentialsJson := `{
				"netboxUrl": "aHR0cDovL25ldGJveA==",
				"netboxToken": "not_base64",
				"bmcUser": "dXNlcg==",
				"bmcPassword": "cGFzc3dvcmQ="
			}`
			fileReaderMock.fileContent["/etc/config/config.json"] = configJson
			fileReaderMock.fileContent["/etc/credentials/credentials.json"] = credentialsJson

			// when
			err := cfg.Reload()

			// then
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("illegal base64 data at input byte 3"))
		})
	})

	Context("when BMCUser is not base64 encoded", func() {
		It("should return an error", func() {
			// given
			configJson := `{
				"ironCoreRoles": "role1",
				"ironCoreRegion": "region1",
				"serverController": "controller1"
			}`
			credentialsJson := `{
				"netboxUrl": "aHR0cDovL25ldGJveA==",
				"netboxToken": "dG9rZW4=",
				"bmcUser": "not_base64",
				"bmcPassword": "cGFzc3dvcmQ="
			}`
			fileReaderMock.fileContent["/etc/config/config.json"] = configJson
			fileReaderMock.fileContent["/etc/credentials/credentials.json"] = credentialsJson

			// when
			err := cfg.Reload()

			// then
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("illegal base64 data at input byte 3"))
		})
	})

	Context("when BMCPassword is not base64 encoded", func() {
		It("should return an error", func() {
			// given
			configJson := `{
				"ironCoreRoles": "role1",
				"ironCoreRegion": "region1",
				"serverController": "controller1"
			}`
			credentialsJson := `{
				"netboxUrl": "aHR0cDovL25ldGJveA==",
				"netboxToken": "dG9rZW4=",
				"bmcUser": "dXNlcg==",
				"bmcPassword": "not_base64"
			}`
			fileReaderMock.fileContent["/etc/config/config.json"] = configJson
			fileReaderMock.fileContent["/etc/credentials/credentials.json"] = credentialsJson

			// when
			err := cfg.Reload()

			// then
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("illegal base64 data at input byte 3"))
		})
	})
})

var _ = Describe("readJsonAndUnmarshal", func() {
	var cfg *Config
	var fileReaderMock *FileReaderMock

	BeforeEach(func() {
		client := fake.NewClientBuilder().Build()
		fileReaderMock = &FileReaderMock{
			fileContent: make(map[string]string),
			returnError: false,
		}
		cfg = &Config{client, fileReaderMock, "", "", "", "", "", "", ""}
	})

	Context("when the file is read successfully", func() {
		It("should unmarshal the JSON and decode base64 fields", func() {
			// given
			configJsonContent := `{
				"ironCoreRoles": "role1",
				"ironCoreRegion": "region1",
				"serverController": "controller1"
			}`
			credentialsJsonContent := `{
				"netboxUrl": "aHR0cDovL25ldGJveA==",
				"netboxToken": "dG9rZW4=",
				"bmcUser": "dXNlcg==",
				"bmcPassword": "cGFzc3dvcmQ="
			}`
			fileReaderMock.fileContent["/etc/config/config.json"] = configJsonContent
			fileReaderMock.fileContent["/etc/config/credentials.json"] = credentialsJsonContent

			// when
			err := cfg.readJsonAndUnmarshal("/etc/config/config.json")

			// then
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg.IronCoreRoles).To(Equal("role1"))
			Expect(cfg.IronCoreRegion).To(Equal("region1"))
			Expect(cfg.ServerController).To(Equal("controller1"))

			// when
			err = cfg.readJsonAndUnmarshal("/etc/config/credentials.json")

			// then
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg.NetboxUrl).To(Equal("aHR0cDovL25ldGJveA=="))
			Expect(cfg.NetboxToken).To(Equal("dG9rZW4="))
			Expect(cfg.BMCUser).To(Equal("dXNlcg=="))
			Expect(cfg.BMCPassword).To(Equal("cGFzc3dvcmQ="))
		})
	})

	Context("when the file cannot be read", func() {
		It("should return an error", func() {
			// given
			fileReaderMock.returnError = true

			// when
			err := cfg.readJsonAndUnmarshal("/etc/config/config.json")

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
			err := cfg.readJsonAndUnmarshal("/etc/config/config.json")

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
			err := cfg.readJsonAndUnmarshal("/etc/config/credentials.json")

			// then
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("invalid character 'i' looking for beginning of value"))
		})
	})
})
