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
		cfg = NewDefaultConfiguration(client)

		cfg.IronCoreRoles = "role1"
		cfg.IronCoreRegion = "region1"
		cfg.IronCoreClusterTypes = "type1"
		cfg.ServerController = "controller1"
		cfg.K8sServiceHost = "host1"
		cfg.NetboxUrl = "http://netbox"
		cfg.NetboxToken = "token"
		cfg.BMCUser = "user"
		cfg.BMCPassword = "password"
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
				Expect(err.Error()).To(Equal("ironcore roles are required"))
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
				Expect(err.Error()).To(Equal("ironcore region is required"))
			})
		})

		Context("when IronCoreClusterTypes is empty", func() {
			It("should return an error", func() {
				// given
				cfg.IronCoreClusterTypes = ""

				// when
				err := cfg.Validate()

				// then
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("ironcore cluster types are required"))
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
				Expect(err.Error()).To(Equal("server controller name is required"))
			})
		})

		Context("when K8sServiceHost is empty", func() {
			It("should return an error", func() {
				// given
				cfg.K8sServiceHost = ""

				// when
				err := cfg.Validate()

				// then
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("k8s service host is required"))
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
				Expect(err.Error()).To(Equal("netbox URL is required"))
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
				Expect(err.Error()).To(Equal("netbox token is required"))
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
				Expect(err.Error()).To(Equal("bmc user is required"))
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
				Expect(err.Error()).To(Equal("bmc password is required"))
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
		cfg = &Config{client, fileReaderMock, "", "", "", "", "", "", "", "", ""}
	})

	Context("when all fields are valid", func() {
		It("should not return an error", func() {
			// given
			configJson := `{
				"ironCoreRoles": "role1",
				"ironCoreRegion": "region1",
				"ironCoreClusterTypes": "type1",
				"serverController": "controller1",
				"k8sServiceHost": "host1"
			}`
			credentialsJson := `{
				"netboxUrl": "http://netbox",
				"netboxToken": "token",
				"bmcUsername": "user",
				"bmcPassword": "password"
			}`

			fileReaderMock.fileContent["/etc/config/config.json"] = configJson
			fileReaderMock.fileContent["/etc/credentials/credentials.json"] = credentialsJson

			// when
			err := cfg.Reload()

			// then
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("when config file is missing", func() {
		It("should return an error", func() {
			// given
			credentialsJson := `{
				"netboxUrl": "http://netbox",
				"netboxToken": "token",
				"bmcUsername": "user",
				"bmcPassword": "password"
			}`

			fileReaderMock.fileContent["/etc/credentials/credentials.json"] = credentialsJson

			// when
			err := cfg.Reload()

			// then
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when credentials file is missing", func() {
		It("should return an error", func() {
			// given
			configJson := `{
				"ironCoreRoles": "role1",
				"ironCoreRegion": "region1",
				"ironCoreClusterTypes": "type1",
				"serverController": "controller1",
				"k8sServiceHost": "host1"
			}`

			fileReaderMock.fileContent["/etc/config/config.json"] = configJson

			// when
			err := cfg.Reload()

			// then
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("unexpected end of JSON input"))
		})
	})

	Context("when config file contains invalid JSON", func() {
		It("should return an error", func() {
			// given
			configJson := `a`
			credentialsJson := `{
				"netboxUrl": "http://netbox",
				"netboxToken": "token",
				"bmcUsername": "user",
				"bmcPassword": "password"
			}`

			fileReaderMock.fileContent["/etc/config/config.json"] = configJson
			fileReaderMock.fileContent["/etc/credentials/credentials.json"] = credentialsJson

			// when
			err := cfg.Reload()

			// then
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("invalid character 'a' looking for beginning of value"))
		})
	})

	Context("when credentials file contains invalid JSON", func() {
		It("should return an error", func() {
			// given
			configJson := `{
				"ironCoreRoles": "role1",
				"ironCoreRegion": "region1",
				"ironCoreClusterTypes": "type1",
				"serverController": "controller1",
				"k8sServiceHost": "host1"
			}`
			credentialsJson := `b`

			fileReaderMock.fileContent["/etc/config/config.json"] = configJson
			fileReaderMock.fileContent["/etc/credentials/credentials.json"] = credentialsJson
			// when
			err := cfg.Reload()

			// then
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("invalid character 'b' looking for beginning of value"))
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
