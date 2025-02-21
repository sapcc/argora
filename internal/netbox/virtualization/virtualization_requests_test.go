package virtualization_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sapcc/argora/internal/netbox/virtualization"
)

func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Virtualization Suite")
}

var _ = Describe("ListClusterRequest", func() {
	Context("When creating a new ListClusterRequest", func() {
		It("should create a request with the provided name", func() {
			req := virtualization.NewListClusterRequest(virtualization.WithName("test-name"))
			Expect(req).NotTo(BeNil())
			Expect(req.BuildRequest().Name).To(Equal("test-name"))
		})

		It("should create a request with the provided role", func() {
			req := virtualization.NewListClusterRequest(virtualization.WithType("test-type"))
			Expect(req).NotTo(BeNil())
			Expect(req.BuildRequest().Type).To(Equal("test-type"))
		})

		It("should create a request with the provided region", func() {
			req := virtualization.NewListClusterRequest(virtualization.WithRegion("test-region"))
			Expect(req).NotTo(BeNil())
			Expect(req.BuildRequest().Region).To(Equal("test-region"))
		})

		It("should create a request with all provided options", func() {
			req := virtualization.NewListClusterRequest(
				virtualization.WithName("test-name"),
				virtualization.WithType("test-type"),
				virtualization.WithRegion("test-region"),
			)
			Expect(req).NotTo(BeNil())

			builtReq := req.BuildRequest()
			Expect(builtReq.Name).To(Equal("test-name"))
			Expect(builtReq.Type).To(Equal("test-type"))
			Expect(builtReq.Region).To(Equal("test-region"))
		})
	})
})
