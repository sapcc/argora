package dcim_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sapcc/argora/internal/netbox/dcim"
)

var _ = Describe("BuildRequest", func() {
	Context("ListDevicesRequest", func() {
		It("should build request with name", func() {
			req := dcim.NewListDevicesRequest(
				dcim.DeviceWithName("test-device"),
			).BuildRequest()

			Expect(req.Name).To(Equal("test-device"))
			Expect(req.ClusterID).To(BeZero())
		})

		It("should build request with cluster ID", func() {
			req := dcim.NewListDevicesRequest(
				dcim.DeviceWithClusterID(123),
			).BuildRequest()

			Expect(req.ClusterID).To(Equal(123))
			Expect(req.Name).To(BeZero())
		})

		It("should build request with name and cluster ID", func() {
			req := dcim.NewListDevicesRequest(
				dcim.DeviceWithName("test-device"),
				dcim.DeviceWithClusterID(123),
			).BuildRequest()

			Expect(req.Name).To(Equal("test-device"))
			Expect(req.ClusterID).To(Equal(123))
		})
	})

	Context("ListDeviceRolesRequest", func() {
		It("should build request with name", func() {
			req := dcim.NewListDeviceRolesRequest(
				dcim.RoleWithName("test-role"),
			).BuildRequest()

			Expect(req.Name).To(Equal("test-role"))
		})
	})

	Context("ListInterfacesRequest", func() {
		It("should build request with name", func() {
			req := dcim.NewListInterfacesRequest(
				dcim.InterfaceWithName("test-interface"),
			).BuildRequest()

			Expect(req.Name).To(Equal("test-interface"))
		})

		It("should build request with interface ID", func() {
			req := dcim.NewListInterfacesRequest(
				dcim.InterfaceWithDeviceID(456),
			).BuildRequest()

			Expect(req.DeviceID).To(Equal(456))
			Expect(req.Name).To(BeZero())
		})

		It("should build request with name and interface ID", func() {
			req := dcim.NewListInterfacesRequest(
				dcim.InterfaceWithName("test-interface"),
				dcim.InterfaceWithDeviceID(456),
			).BuildRequest()

			Expect(req.Name).To(Equal("test-interface"))
			Expect(req.DeviceID).To(Equal(456))
		})
	})

	Context("ListPlatformsRequest", func() {
		It("should build request with name", func() {
			req := dcim.NewListPlatformsRequest(
				dcim.PlatformWithName("test-platform"),
			).BuildRequest()

			Expect(req.Name).To(Equal("test-platform"))
		})
	})
})
