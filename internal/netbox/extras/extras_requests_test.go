package extras_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sapcc/argora/internal/netbox/extras"
)

func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Extras Suite")
}

var _ = Describe("ListTagsRequest", func() {
	Context("BuildRequest", func() {
		It("should build a request with the specified name", func() {
			req := extras.NewListTagsRequest(
				extras.WithName("test-tag"),
			)
			builtRequest := req.BuildRequest()
			Expect(builtRequest.Name).To(Equal("test-tag"))
		})

		It("should build a request with an empty name if not specified", func() {
			req := extras.NewListTagsRequest()
			builtRequest := req.BuildRequest()
			Expect(builtRequest.Name).To(BeEmpty())
		})
	})
})
