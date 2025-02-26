package dcim_test

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sapcc/argora/internal/netbox/dcim"
	"github.com/sapcc/go-netbox-go/common"
	"github.com/sapcc/go-netbox-go/models"
)

func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DCIM Suite")
}

type MockDCIMClient struct {
	GetRegionFunc func(id int) (*models.Region, error)
	GetSiteFunc   func(id int) (*models.Site, error)

	ListDevicesFunc     func(opts models.ListDevicesRequest) (*models.ListDevicesResponse, error)
	ListDeviceRolesFunc func(opts models.ListDeviceRolesRequest) (*models.ListDeviceRolesResponse, error)
	ListInterfacesFunc  func(opts models.ListInterfacesRequest) (*models.ListInterfacesResponse, error)
	ListPlatformsFunc   func(opts models.ListPlatformsRequest) (*models.ListPlatformsResponse, error)

	UpdateDeviceFunc    func(dev models.WritableDeviceWithConfigContext) (*models.Device, error)
	UpdateInterfaceFunc func(iface models.WritableInterface, id int) (*models.Interface, error)

	DeleteInterfaceFunc func(id int) error
}

func (d *MockDCIMClient) GetRegion(id int) (*models.Region, error) {
	return d.GetRegionFunc(id)
}

func (d *MockDCIMClient) GetSite(id int) (*models.Site, error) {
	return d.GetSiteFunc(id)
}

func (d *MockDCIMClient) ListDevices(opts models.ListDevicesRequest) (*models.ListDevicesResponse, error) {
	return d.ListDevicesFunc(opts)
}

func (d *MockDCIMClient) ListDeviceRoles(opts models.ListDeviceRolesRequest) (*models.ListDeviceRolesResponse, error) {
	return d.ListDeviceRolesFunc(opts)
}

func (d *MockDCIMClient) ListInterfaces(opts models.ListInterfacesRequest) (*models.ListInterfacesResponse, error) {
	return d.ListInterfacesFunc(opts)
}

func (d *MockDCIMClient) ListPlatforms(opts models.ListPlatformsRequest) (*models.ListPlatformsResponse, error) {
	return d.ListPlatformsFunc(opts)
}

func (d *MockDCIMClient) UpdateDevice(device models.WritableDeviceWithConfigContext) (*models.Device, error) {
	return d.UpdateDeviceFunc(device)
}

func (d *MockDCIMClient) UpdateInterface(iface models.WritableInterface, id int) (*models.Interface, error) {
	return d.UpdateInterfaceFunc(iface, id)
}

func (d *MockDCIMClient) DeleteInterface(id int) error {
	return d.DeleteInterfaceFunc(id)
}

var _ = Describe("DCIM", func() {
	var (
		mockClient  *MockDCIMClient
		dcimService dcim.DCIM
	)

	BeforeEach(func() {
		mockClient = &MockDCIMClient{}
		dcimService = dcim.NewDCIM(mockClient)
	})

	Describe("GetDeviceByName", func() {
		It("should return the device when found", func() {
			mockClient.ListDevicesFunc = func(opts models.ListDevicesRequest) (*models.ListDevicesResponse, error) {
				return &models.ListDevicesResponse{
					ReturnValues: common.ReturnValues{
						Count: 1,
					},
					Results: []models.Device{{Name: "device1"}},
				}, nil
			}

			device, err := dcimService.GetDeviceByName("device1")
			Expect(err).ToNot(HaveOccurred())
			Expect(device.Name).To(Equal("device1"))
		})

		It("should return an error when device is not found", func() {
			mockClient.ListDevicesFunc = func(opts models.ListDevicesRequest) (*models.ListDevicesResponse, error) {
				return &models.ListDevicesResponse{
					ReturnValues: common.ReturnValues{
						Count: 0,
					},
					Results: []models.Device{},
				}, nil
			}

			_, err := dcimService.GetDeviceByName("device1")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unexpected number of devices found (0)"))
		})
	})

	Describe("GetDeviceByID", func() {
		It("should return the device when found", func() {
			mockClient.ListDevicesFunc = func(opts models.ListDevicesRequest) (*models.ListDevicesResponse, error) {
				return &models.ListDevicesResponse{
					ReturnValues: common.ReturnValues{
						Count: 1,
					},
					Results: []models.Device{{ID: 1}},
				}, nil
			}

			device, err := dcimService.GetDeviceByID(1)
			Expect(err).ToNot(HaveOccurred())
			Expect(device.ID).To(Equal(1))
		})

		It("should return an error when device is not found", func() {
			mockClient.ListDevicesFunc = func(opts models.ListDevicesRequest) (*models.ListDevicesResponse, error) {
				return &models.ListDevicesResponse{
					ReturnValues: common.ReturnValues{
						Count: 0,
					},
					Results: []models.Device{},
				}, nil
			}

			_, err := dcimService.GetDeviceByID(1)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unexpected number of devices found (0)"))
		})
	})

	Describe("GetDevicesByClusterID", func() {
		It("should return the devices when found", func() {
			mockClient.ListDevicesFunc = func(opts models.ListDevicesRequest) (*models.ListDevicesResponse, error) {
				return &models.ListDevicesResponse{
					ReturnValues: common.ReturnValues{
						Count: 2,
					},
					Results: []models.Device{{ID: 1}, {ID: 2}},
				}, nil
			}

			devices, err := dcimService.GetDevicesByClusterID(1)
			Expect(err).ToNot(HaveOccurred())
			Expect(devices).To(HaveLen(2))
		})

		It("should return an error when no devices are found", func() {
			mockClient.ListDevicesFunc = func(opts models.ListDevicesRequest) (*models.ListDevicesResponse, error) {
				return &models.ListDevicesResponse{
					ReturnValues: common.ReturnValues{
						Count: 0,
					},
					Results: []models.Device{},
				}, nil
			}

			devices, err := dcimService.GetDevicesByClusterID(1)
			Expect(err).ToNot(HaveOccurred())
			Expect(devices).To(HaveLen(0))
		})
	})

	Describe("GetRoleByName", func() {
		It("should return the role when found", func() {
			mockClient.ListDeviceRolesFunc = func(opts models.ListDeviceRolesRequest) (*models.ListDeviceRolesResponse, error) {
				return &models.ListDeviceRolesResponse{
					ReturnValues: common.ReturnValues{
						Count: 1,
					},
					Results: []models.DeviceRole{
						{
							NestedDeviceRole: models.NestedDeviceRole{
								Name: "role1",
							},
						},
					},
				}, nil
			}

			role, err := dcimService.GetRoleByName("role1")
			Expect(err).ToNot(HaveOccurred())
			Expect(role.Name).To(Equal("role1"))
		})

		It("should return an error when role is not found", func() {
			mockClient.ListDeviceRolesFunc = func(opts models.ListDeviceRolesRequest) (*models.ListDeviceRolesResponse, error) {
				return &models.ListDeviceRolesResponse{
					ReturnValues: common.ReturnValues{
						Count: 0,
					},
					Results: []models.DeviceRole{},
				}, nil
			}

			_, err := dcimService.GetRoleByName("role1")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unexpected number of roles found (0)"))
		})
	})

	Describe("GetRegionForDevice", func() {
		It("should return the region slug when found", func() {
			mockClient.GetSiteFunc = func(id int) (*models.Site, error) {
				return &models.Site{
					Region: models.NestedRegion{ID: 1},
				}, nil
			}
			mockClient.GetRegionFunc = func(id int) (*models.Region, error) {
				return &models.Region{Slug: "region1"}, nil
			}

			region, err := dcimService.GetRegionForDevice(&models.Device{Site: models.NestedSite{ID: 1}})
			Expect(err).ToNot(HaveOccurred())
			Expect(region).To(Equal("region1"))
		})

		It("should return an error when site is not found", func() {
			mockClient.GetSiteFunc = func(id int) (*models.Site, error) {
				return nil, fmt.Errorf("site not found")
			}

			_, err := dcimService.GetRegionForDevice(&models.Device{Site: models.NestedSite{ID: 1}})
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unable to get site for ID 1: site not found"))
		})

		It("should return an error when region is not found", func() {
			mockClient.GetSiteFunc = func(id int) (*models.Site, error) {
				return &models.Site{Region: models.NestedRegion{ID: 1}}, nil
			}
			mockClient.GetRegionFunc = func(id int) (*models.Region, error) {
				return nil, fmt.Errorf("region not found")
			}

			_, err := dcimService.GetRegionForDevice(&models.Device{Site: models.NestedSite{ID: 1}})
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unable to get region for ID 1: region not found"))
		})
	})

	Describe("GetInterfaceByID", func() {
		It("should return the interface when found", func() {
			mockClient.ListInterfacesFunc = func(opts models.ListInterfacesRequest) (*models.ListInterfacesResponse, error) {
				return &models.ListInterfacesResponse{
					Results: []models.Interface{
						{
							NestedInterface: models.NestedInterface{ID: 1},
						},
					},
				}, nil
			}

			iface, err := dcimService.GetInterfaceByID(1)
			Expect(err).ToNot(HaveOccurred())
			Expect(iface.ID).To(Equal(1))
		})

		It("should return an error when interface is not found", func() {
			mockClient.ListInterfacesFunc = func(opts models.ListInterfacesRequest) (*models.ListInterfacesResponse, error) {
				return &models.ListInterfacesResponse{
					Results: []models.Interface{},
				}, nil
			}

			_, err := dcimService.GetInterfaceByID(1)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("interface with ID 1 not found"))
		})
	})

	Describe("GetInterfacesForDevice", func() {
		It("should return the interfaces when found", func() {
			mockClient.ListInterfacesFunc = func(opts models.ListInterfacesRequest) (*models.ListInterfacesResponse, error) {
				return &models.ListInterfacesResponse{
					Results: []models.Interface{
						{
							NestedInterface: models.NestedInterface{ID: 1},
						},
						{
							NestedInterface: models.NestedInterface{ID: 2},
						},
					},
				}, nil
			}

			ifaces, err := dcimService.GetInterfacesForDevice(&models.Device{ID: 1})
			Expect(err).ToNot(HaveOccurred())
			Expect(ifaces).To(HaveLen(2))
		})

		It("should return an error when no interfaces are found", func() {
			mockClient.ListInterfacesFunc = func(opts models.ListInterfacesRequest) (*models.ListInterfacesResponse, error) {
				return &models.ListInterfacesResponse{
					Results: []models.Interface{},
				}, nil
			}

			ifaces, err := dcimService.GetInterfacesForDevice(&models.Device{ID: 1})
			Expect(err).ToNot(HaveOccurred())
			Expect(ifaces).To(HaveLen(0))
		})
	})

	Describe("GetInterfaceForDevice", func() {
		It("should return the interface when found", func() {
			mockClient.ListInterfacesFunc = func(opts models.ListInterfacesRequest) (*models.ListInterfacesResponse, error) {
				return &models.ListInterfacesResponse{
					Results: []models.Interface{
						{
							NestedInterface: models.NestedInterface{ID: 1},
							Name:            "eth0",
						},
					},
				}, nil
			}

			iface, err := dcimService.GetInterfaceForDevice(&models.Device{ID: 1}, "eth0")
			Expect(err).ToNot(HaveOccurred())
			Expect(iface.Name).To(Equal("eth0"))
		})

		It("should return an error when interface is not found", func() {
			mockClient.ListInterfacesFunc = func(opts models.ListInterfacesRequest) (*models.ListInterfacesResponse, error) {
				return &models.ListInterfacesResponse{
					Results: []models.Interface{},
				}, nil
			}

			_, err := dcimService.GetInterfaceForDevice(&models.Device{ID: 1}, "eth0")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("eth0 interface not found"))
		})
	})

	Describe("GetInterfacesByLagID", func() {
		It("should return the interfaces when found", func() {
			mockClient.ListInterfacesFunc = func(opts models.ListInterfacesRequest) (*models.ListInterfacesResponse, error) {
				return &models.ListInterfacesResponse{
					Results: []models.Interface{
						{
							NestedInterface: models.NestedInterface{ID: 1},
						},
						{
							NestedInterface: models.NestedInterface{ID: 2},
						},
					},
				}, nil
			}

			ifaces, err := dcimService.GetInterfacesByLagID(1)
			Expect(err).ToNot(HaveOccurred())
			Expect(ifaces).To(HaveLen(2))
		})

		It("should return an error when no interfaces are found", func() {
			mockClient.ListInterfacesFunc = func(opts models.ListInterfacesRequest) (*models.ListInterfacesResponse, error) {
				return &models.ListInterfacesResponse{
					Results: []models.Interface{},
				}, nil
			}

			ifaces, err := dcimService.GetInterfacesByLagID(1)
			Expect(err).ToNot(HaveOccurred())
			Expect(ifaces).To(HaveLen(0))
		})
	})

	Describe("GetPlatformByName", func() {
		It("should return the platform when found", func() {
			mockClient.ListPlatformsFunc = func(opts models.ListPlatformsRequest) (*models.ListPlatformsResponse, error) {
				return &models.ListPlatformsResponse{
					ReturnValues: common.ReturnValues{
						Count: 1,
					},
					Results: []models.Platform{
						{
							NestedPlatform: models.NestedPlatform{Name: "platform1"},
						},
					},
				}, nil
			}

			platform, err := dcimService.GetPlatformByName("platform1")
			Expect(err).ToNot(HaveOccurred())
			Expect(platform.Name).To(Equal("platform1"))
		})

		It("should return an error when platform is not found", func() {
			mockClient.ListPlatformsFunc = func(opts models.ListPlatformsRequest) (*models.ListPlatformsResponse, error) {
				return &models.ListPlatformsResponse{
					ReturnValues: common.ReturnValues{
						Count: 0,
					},
					Results: []models.Platform{},
				}, nil
			}

			_, err := dcimService.GetPlatformByName("platform1")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unexpected number of platforms found (0)"))
		})
	})

	Describe("UpdateDevice", func() {
		It("should update the device successfully", func() {
			mockClient.UpdateDeviceFunc = func(dev models.WritableDeviceWithConfigContext) (*models.Device, error) {
				return &models.Device{ID: dev.ID, Name: dev.Name}, nil
			}

			device, err := dcimService.UpdateDevice(models.WritableDeviceWithConfigContext{ID: 1, Name: "updatedDevice"})
			Expect(err).ToNot(HaveOccurred())
			Expect(device.ID).To(Equal(1))
			Expect(device.Name).To(Equal("updatedDevice"))
		})

		It("should return an error when update fails", func() {
			mockClient.UpdateDeviceFunc = func(dev models.WritableDeviceWithConfigContext) (*models.Device, error) {
				return nil, fmt.Errorf("update failed")
			}

			_, err := dcimService.UpdateDevice(models.WritableDeviceWithConfigContext{ID: 1, Name: "updatedDevice"})
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unable to update device: update failed"))
		})
	})

	Describe("UpdateInterface", func() {
		It("should update the interface successfully", func() {
			mockClient.UpdateInterfaceFunc = func(iface models.WritableInterface, id int) (*models.Interface, error) {
				return &models.Interface{
					NestedInterface: models.NestedInterface{
						ID: id,
					},
					Name: iface.Name}, nil
			}

			iface, err := dcimService.UpdateInterface(models.WritableInterface{Name: "updatedInterface"}, 1)
			Expect(err).ToNot(HaveOccurred())
			Expect(iface.ID).To(Equal(1))
			Expect(iface.Name).To(Equal("updatedInterface"))
		})

		It("should return an error when update fails", func() {
			mockClient.UpdateInterfaceFunc = func(iface models.WritableInterface, id int) (*models.Interface, error) {
				return nil, fmt.Errorf("update failed")
			}

			_, err := dcimService.UpdateInterface(models.WritableInterface{Name: "updatedInterface"}, 1)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unable to update interface: update failed"))
		})
	})

	Describe("DeleteInterface", func() {
		It("should delete the interface successfully", func() {
			mockClient.DeleteInterfaceFunc = func(id int) error {
				return nil
			}

			err := dcimService.DeleteInterface(1)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return an error when delete fails", func() {
			mockClient.DeleteInterfaceFunc = func(id int) error {
				return fmt.Errorf("delete failed")
			}

			err := dcimService.DeleteInterface(1)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unable to delete interface (1): delete failed"))
		})
	})
})
