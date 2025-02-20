package config_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sapcc/argora/internal/config"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Config Suite")
}

var _ = Describe("Config", func() {
	var cfg *config.Config

	BeforeEach(func() {
		client := fake.NewClientBuilder().Build()
		cfg = config.NewDefaultConfiguration(client)

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
