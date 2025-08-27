// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package ipam_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sapcc/argora/internal/netbox/ipam"
)

var _ = Describe("BuildRequest", func() {
	Context("ListVlanRequest", func() {
		It("should build request with name", func() {
			req := ipam.NewListVlanRequest(
				ipam.VlanWithName("test-vlan"),
			).BuildRequest()

			Expect(req.Name).To(Equal("test-vlan"))
		})

		It("should build request without name", func() {
			req := ipam.NewListVlanRequest().BuildRequest()

			Expect(req.Name).To(BeEmpty())
		})
	})

	Context("ListIPAddressesRequest", func() {
		It("should build request with interface ID", func() {
			req := ipam.NewListIPAddressesRequest(
				ipam.IPAddressesWithInterfaceID(123),
			).BuildRequest()

			Expect(req.InterfaceID).To(Equal(123))
		})

		It("should build request without interface ID", func() {
			req := ipam.NewListIPAddressesRequest().BuildRequest()

			Expect(req.InterfaceID).To(BeZero())
		})
	})
})
