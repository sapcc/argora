// Copyright 2025 SAP SE
// SPDX-License-Identifier: Apache-2.0

package dcim_test

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sapcc/go-netbox-go/common"
	"github.com/sapcc/go-netbox-go/models"

	"github.com/sapcc/argora/internal/netbox/dcim"
)

func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DCIM Suite")
}

type MockDCIMClient struct {
	// http connectable
	HTTPClientFunc    func() *http.Client
	SetHTTPClientFunc func(httpClient *http.Client)
	BaseURLFunc       func() *url.URL
	SetBaseURLFunc    func(url *url.URL)
	AuthTokenFunc     func() string
	SetAuthTokenFunc  func(authToken string)

	// cables
	GetCableFunc    func(id int) (*models.Cable, error)
	CreateCableFunc func(cable models.WriteableCable) (*models.Cable, error)
	DeleteCableFunc func(id int) error
	UpdateCableFunc func(cable models.WriteableCable) (*models.Cable, error)

	// device-roles
	ListDeviceRolesFunc func(opts models.ListDeviceRolesRequest) (*models.ListDeviceRolesResponse, error)

	// device-types
	ListDeviceTypesFunc func(opts models.ListDeviceTypesRequest) (*models.ListDeviceTypesResponse, error)

	// devices
	ListDevicesFunc          func(opts models.ListDevicesRequest) (*models.ListDevicesResponse, error)
	GetDeviceFunc            func(id int) (*models.Device, error)
	GetDeviceWithContextFunc func(id int) (*models.Device, error)
	CreateDeviceFunc         func(dev models.WritableDeviceWithConfigContext) (*models.Device, error)
	DeleteDeviceFunc         func(id int) error
	UpdateDeviceFunc         func(dev models.WritableDeviceWithConfigContext) (*models.Device, error)

	// interfaces
	ListInterfacesFunc  func(opts models.ListInterfacesRequest) (*models.ListInterfacesResponse, error)
	UpdateInterfaceFunc func(iface models.WritableInterface, id int) (*models.Interface, error)
	CreateInterfaceFunc func(interf models.WritableInterface) (*models.Interface, error)
	DeleteInterfaceFunc func(id int) error

	// platforms
	ListPlatformsFunc func(opts models.ListPlatformsRequest) (*models.ListPlatformsResponse, error)

	// racks
	ListRacksFunc func(opts models.ListRacksRequest) (*models.ListRacksResponse, error)

	// regions
	ListRegionsFunc func(opts models.ListRegionsRequest) (*models.ListRegionsResponse, error)
	GetRegionFunc   func(id int) (*models.Region, error)

	// site groups
	ListSiteGroupsFunc func(opts models.ListSiteGroupsRequest) (*models.ListSiteGroupsResponse, error)
	GetSiteGroupFunc   func(id int) (*models.SiteGroup, error)

	// sites
	ListSitesFunc func(opts models.ListSitesRequest) (*models.ListSitesResponse, error)
	GetSiteFunc   func(id int) (*models.Site, error)
}

func (m *MockDCIMClient) HTTPClient() *http.Client {
	return m.HTTPClientFunc()
}

func (m *MockDCIMClient) SetHTTPClient(httpClient *http.Client) {
	m.SetHTTPClientFunc(httpClient)
}

func (m *MockDCIMClient) BaseURL() *url.URL {
	return m.BaseURLFunc()
}

func (m *MockDCIMClient) SetBaseURL(url *url.URL) {
	m.SetBaseURLFunc(url)
}

func (m *MockDCIMClient) AuthToken() string {
	return m.AuthTokenFunc()
}

func (m *MockDCIMClient) SetAuthToken(authToken string) {
	m.SetAuthTokenFunc(authToken)
}

func (m *MockDCIMClient) GetCable(id int) (*models.Cable, error) {
	return m.GetCableFunc(id)
}

func (m *MockDCIMClient) CreateCable(cable models.WriteableCable) (*models.Cable, error) {
	return m.CreateCableFunc(cable)
}

func (m *MockDCIMClient) DeleteCable(id int) error {
	return m.DeleteCableFunc(id)
}

func (m *MockDCIMClient) UpdateCable(cable models.WriteableCable) (*models.Cable, error) {
	return m.UpdateCableFunc(cable)
}

func (m *MockDCIMClient) ListDeviceRoles(opts models.ListDeviceRolesRequest) (*models.ListDeviceRolesResponse, error) {
	return m.ListDeviceRolesFunc(opts)
}

func (m *MockDCIMClient) ListDeviceTypes(opts models.ListDeviceTypesRequest) (*models.ListDeviceTypesResponse, error) {
	return m.ListDeviceTypesFunc(opts)
}

func (m *MockDCIMClient) ListDevices(opts models.ListDevicesRequest) (*models.ListDevicesResponse, error) {
	return m.ListDevicesFunc(opts)
}

func (m *MockDCIMClient) GetDevice(id int) (*models.Device, error) {
	return m.GetDeviceFunc(id)
}

func (m *MockDCIMClient) GetDeviceWithContext(id int) (*models.Device, error) {
	return m.GetDeviceWithContextFunc(id)
}

func (m *MockDCIMClient) CreateDevice(dev models.WritableDeviceWithConfigContext) (*models.Device, error) {
	return m.CreateDeviceFunc(dev)
}

func (m *MockDCIMClient) DeleteDevice(id int) error {
	return m.DeleteDeviceFunc(id)
}

func (m *MockDCIMClient) UpdateDevice(dev models.WritableDeviceWithConfigContext) (*models.Device, error) {
	return m.UpdateDeviceFunc(dev)
}

func (m *MockDCIMClient) ListInterfaces(opts models.ListInterfacesRequest) (*models.ListInterfacesResponse, error) {
	return m.ListInterfacesFunc(opts)
}

func (m *MockDCIMClient) UpdateInterface(iface models.WritableInterface, id int) (*models.Interface, error) {
	return m.UpdateInterfaceFunc(iface, id)
}

func (m *MockDCIMClient) CreateInterface(interf models.WritableInterface) (*models.Interface, error) {
	return m.CreateInterfaceFunc(interf)
}

func (m *MockDCIMClient) DeleteInterface(id int) error {
	return m.DeleteInterfaceFunc(id)
}

func (m *MockDCIMClient) ListPlatforms(opts models.ListPlatformsRequest) (*models.ListPlatformsResponse, error) {
	return m.ListPlatformsFunc(opts)
}

func (m *MockDCIMClient) ListRacks(opts models.ListRacksRequest) (*models.ListRacksResponse, error) {
	return m.ListRacksFunc(opts)
}

func (m *MockDCIMClient) ListRegions(opts models.ListRegionsRequest) (*models.ListRegionsResponse, error) {
	return m.ListRegionsFunc(opts)
}

func (m *MockDCIMClient) GetRegion(id int) (*models.Region, error) {
	return m.GetRegionFunc(id)
}

func (m *MockDCIMClient) ListSiteGroups(opts models.ListSiteGroupsRequest) (*models.ListSiteGroupsResponse, error) {
	return m.ListSiteGroupsFunc(opts)
}

func (m *MockDCIMClient) GetSiteGroup(id int) (*models.SiteGroup, error) {
	return m.GetSiteGroupFunc(id)
}

func (m *MockDCIMClient) ListSites(opts models.ListSitesRequest) (*models.ListSitesResponse, error) {
	return m.ListSitesFunc(opts)
}

func (m *MockDCIMClient) GetSite(id int) (*models.Site, error) {
	return m.GetSiteFunc(id)
}

var _ = Describe("DCIM", func() {
	var (
		mockClient  *MockDCIMClient
		dcimService dcim.DCIM
	)

	BeforeEach(func() {
		mockClient = &MockDCIMClient{}
		dcimService = dcim.NewDCIM(mockClient, logr.Discard())
	})

	Describe("GetDeviceByName", func() {
		It("should return the device when found", func() {
			mockClient.ListDevicesFunc = func(opts models.ListDevicesRequest) (*models.ListDevicesResponse, error) {
				Expect(opts.Name).To(Equal("device1"))
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
				Expect(opts.Name).To(Equal("device1"))
				return &models.ListDevicesResponse{
					ReturnValues: common.ReturnValues{
						Count: 0,
					},
					Results: []models.Device{},
				}, nil
			}

			_, err := dcimService.GetDeviceByName("device1")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unexpected number of devices found by name device1: 0"))
		})
	})

	Describe("GetDeviceByID", func() {
		It("should return the device when found", func() {
			mockClient.ListDevicesFunc = func(opts models.ListDevicesRequest) (*models.ListDevicesResponse, error) {
				Expect(opts.ID).To(Equal(1))
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
				Expect(opts.ID).To(Equal(1))
				return &models.ListDevicesResponse{
					ReturnValues: common.ReturnValues{
						Count: 0,
					},
					Results: []models.Device{},
				}, nil
			}

			_, err := dcimService.GetDeviceByID(1)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unexpected number of devices found for ID 1: 0"))
		})
	})

	Describe("GetDevicesByClusterID", func() {
		It("should return the devices when found", func() {
			mockClient.ListDevicesFunc = func(opts models.ListDevicesRequest) (*models.ListDevicesResponse, error) {
				Expect(opts.ClusterID).To(Equal(1))
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
				Expect(opts.ClusterID).To(Equal(1))
				return &models.ListDevicesResponse{
					ReturnValues: common.ReturnValues{
						Count: 0,
					},
					Results: []models.Device{},
				}, nil
			}

			devices, err := dcimService.GetDevicesByClusterID(1)
			Expect(err).ToNot(HaveOccurred())
			Expect(devices).To(BeEmpty())
		})
	})

	Describe("GetRoleByName", func() {
		It("should return the role when found", func() {
			mockClient.ListDeviceRolesFunc = func(opts models.ListDeviceRolesRequest) (*models.ListDeviceRolesResponse, error) {
				Expect(opts.Name).To(Equal("role1"))
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
				Expect(opts.Name).To(Equal("role1"))
				return &models.ListDeviceRolesResponse{
					ReturnValues: common.ReturnValues{
						Count: 0,
					},
					Results: []models.DeviceRole{},
				}, nil
			}

			_, err := dcimService.GetRoleByName("role1")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unexpected number of roles found by name role1: 0"))
		})
	})

	Describe("GetRegionForDevice", func() {
		It("should return the region slug when found", func() {
			mockClient.GetSiteFunc = func(id int) (*models.Site, error) {
				Expect(id).To(Equal(1))
				return &models.Site{
					Region: models.NestedRegion{ID: 1},
				}, nil
			}
			mockClient.GetRegionFunc = func(id int) (*models.Region, error) {
				Expect(id).To(Equal(1))
				return &models.Region{Slug: "region1"}, nil
			}

			region, err := dcimService.GetRegionForDevice(&models.Device{Site: models.NestedSite{ID: 1}})
			Expect(err).ToNot(HaveOccurred())
			Expect(region).To(Equal("region1"))
		})

		It("should return an error when site is not found", func() {
			mockClient.GetSiteFunc = func(id int) (*models.Site, error) {
				Expect(id).To(Equal(1))
				return nil, fmt.Errorf("site not found")
			}

			_, err := dcimService.GetRegionForDevice(&models.Device{Site: models.NestedSite{ID: 1}})
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unable to get site for ID 1: site not found"))
		})

		It("should return an error when region is not found", func() {
			mockClient.GetSiteFunc = func(id int) (*models.Site, error) {
				Expect(id).To(Equal(1))
				return &models.Site{Region: models.NestedRegion{ID: 1}}, nil
			}
			mockClient.GetRegionFunc = func(id int) (*models.Region, error) {
				Expect(id).To(Equal(1))
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
				Expect(opts.ID).To(Equal(1))
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
				Expect(opts.ID).To(Equal(1))
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
				Expect(opts.DeviceID).To(Equal(1))
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
				Expect(opts.DeviceID).To(Equal(1))
				return &models.ListInterfacesResponse{
					Results: []models.Interface{},
				}, nil
			}

			ifaces, err := dcimService.GetInterfacesForDevice(&models.Device{ID: 1})
			Expect(err).ToNot(HaveOccurred())
			Expect(ifaces).To(BeEmpty())
		})
	})

	Describe("GetInterfaceForDevice", func() {
		It("should return the interface when found", func() {
			mockClient.ListInterfacesFunc = func(opts models.ListInterfacesRequest) (*models.ListInterfacesResponse, error) {
				Expect(opts.DeviceID).To(Equal(1))
				Expect(opts.Name).To(Equal("eth0"))
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
				Expect(opts.DeviceID).To(Equal(1))
				Expect(opts.Name).To(Equal("eth0"))
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
				Expect(opts.LagID).To(Equal(1))
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
				Expect(opts.LagID).To(Equal(1))
				return &models.ListInterfacesResponse{
					Results: []models.Interface{},
				}, nil
			}

			ifaces, err := dcimService.GetInterfacesByLagID(1)
			Expect(err).ToNot(HaveOccurred())
			Expect(ifaces).To(BeEmpty())
		})
	})

	Describe("GetPlatformByName", func() {
		It("should return the platform when found", func() {
			mockClient.ListPlatformsFunc = func(opts models.ListPlatformsRequest) (*models.ListPlatformsResponse, error) {
				Expect(opts.Name).To(Equal("platform1"))
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
				Expect(opts.Name).To(Equal("platform1"))
				return &models.ListPlatformsResponse{
					ReturnValues: common.ReturnValues{
						Count: 0,
					},
					Results: []models.Platform{},
				}, nil
			}

			_, err := dcimService.GetPlatformByName("platform1")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unexpected number of platforms found by name platform1: 0"))
		})
	})

	Describe("UpdateDevice", func() {
		It("should update the device successfully", func() {
			mockClient.UpdateDeviceFunc = func(dev models.WritableDeviceWithConfigContext) (*models.Device, error) {
				Expect(dev.ID).To(Equal(1))
				Expect(dev.Name).To(Equal("updatedDevice"))
				return &models.Device{ID: dev.ID, Name: dev.Name}, nil
			}

			device, err := dcimService.UpdateDevice(models.WritableDeviceWithConfigContext{ID: 1, Name: "updatedDevice"})
			Expect(err).ToNot(HaveOccurred())
			Expect(device.ID).To(Equal(1))
			Expect(device.Name).To(Equal("updatedDevice"))
		})

		It("should return an error when update fails", func() {
			mockClient.UpdateDeviceFunc = func(dev models.WritableDeviceWithConfigContext) (*models.Device, error) {
				Expect(dev.ID).To(Equal(1))
				Expect(dev.Name).To(Equal("updatedDevice"))
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
				Expect(iface.Name).To(Equal("updatedInterface"))
				Expect(id).To(Equal(1))
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
				Expect(iface.Name).To(Equal("updatedInterface"))
				Expect(id).To(Equal(1))
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
				Expect(id).To(Equal(1))
				return nil
			}

			err := dcimService.DeleteInterface(1)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return an error when delete fails", func() {
			mockClient.DeleteInterfaceFunc = func(id int) error {
				Expect(id).To(Equal(1))
				return fmt.Errorf("delete failed")
			}

			err := dcimService.DeleteInterface(1)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unable to delete interface (1): delete failed"))
		})
	})
})
