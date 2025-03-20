// Copyright 2025 SAP SE
// SPDX-License-Identifier: Apache-2.0

package virtualization_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sapcc/argora/internal/netbox/virtualization"
)

var _ = Describe("ListClusterRequest", func() {
	Context("When creating a new ListClusterRequest", func() {
		It("should create a request with the provided name", func() {
			req := virtualization.NewListClusterRequest(virtualization.WithName("test-name"))
			Expect(req).ToNot(BeNil())
			Expect(req.BuildRequest().Name).To(Equal("test-name"))
		})

		It("should create a request with the provided role", func() {
			req := virtualization.NewListClusterRequest(virtualization.WithType("test-type"))
			Expect(req).ToNot(BeNil())
			Expect(req.BuildRequest().Type).To(Equal("test-type"))
		})

		It("should create a request with the provided region", func() {
			req := virtualization.NewListClusterRequest(virtualization.WithRegion("test-region"))
			Expect(req).ToNot(BeNil())
			Expect(req.BuildRequest().Region).To(Equal("test-region"))
		})

		It("should create a request with all provided options", func() {
			req := virtualization.NewListClusterRequest(
				virtualization.WithName("test-name"),
				virtualization.WithType("test-type"),
				virtualization.WithRegion("test-region"),
			)
			Expect(req).ToNot(BeNil())

			builtReq := req.BuildRequest()
			Expect(builtReq.Name).To(Equal("test-name"))
			Expect(builtReq.Type).To(Equal("test-type"))
			Expect(builtReq.Region).To(Equal("test-region"))
		})
	})
})
