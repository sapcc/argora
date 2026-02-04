// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package netbox

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/go-logr/logr"
	"github.com/sapcc/go-netbox-go/models"

	"github.com/sapcc/argora/internal/netbox/ipam"
)

func TestNetbox(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Netbox Suite")
}

type MockVirtualization struct{}
type MockDCIM struct{}
type MockIPAM struct{}
type MockExtras struct{}

func (m *MockVirtualization) GetClustersByNameRegionType(name, region, clusterType string) ([]models.Cluster, error) {
	return nil, nil
}

func (m *MockDCIM) GetDeviceByName(deviceName string) (*models.Device, error) {
	return nil, nil
}

func (m *MockDCIM) GetDeviceByID(id int) (*models.Device, error) {
	return nil, nil
}

func (m *MockDCIM) GetDevicesByClusterID(clusterID int) ([]models.Device, error) {
	return nil, nil
}

func (m *MockDCIM) GetRoleByName(roleName string) (*models.DeviceRole, error) {
	return nil, nil
}

func (m *MockDCIM) GetRegionForDevice(device *models.Device) (string, error) {
	return "", nil
}

func (m *MockDCIM) GetInterfaceByID(id int) (*models.Interface, error) {
	return nil, nil
}

func (m *MockDCIM) GetInterfacesForDevice(device *models.Device) ([]models.Interface, error) {
	return nil, nil
}

func (m *MockDCIM) GetInterfaceForDevice(device *models.Device, ifaceName string) (*models.Interface, error) {
	return nil, nil
}

func (m *MockDCIM) GetInterfacesByLagID(lagID int) ([]models.Interface, error) {
	return nil, nil
}

func (m *MockDCIM) GetPlatformByName(platformName string) (*models.Platform, error) {
	return nil, nil
}

func (m *MockDCIM) UpdateDevice(device models.WritableDeviceWithConfigContext) (*models.Device, error) {
	return nil, nil
}

func (m *MockDCIM) UpdateInterface(iface models.WritableInterface, id int) (*models.Interface, error) {
	return nil, nil
}

func (m *MockDCIM) DeleteInterface(id int) error {
	return nil
}

func (m *MockIPAM) GetVlanByName(vlanName string) (*models.Vlan, error) {
	return nil, nil
}

func (m *MockIPAM) GetIPAddressByAddress(address string) (*models.IPAddress, error) {
	return nil, nil
}

func (m *MockIPAM) GetIPAddressesForInterface(interfaceID int) ([]models.IPAddress, error) {
	return nil, nil
}

func (m *MockIPAM) GetIPAddressForInterface(interfaceID int) (*models.IPAddress, error) {
	return nil, nil
}

func (m *MockIPAM) GetPrefixesContaining(contains string) ([]models.Prefix, error) {
	return nil, nil
}

func (m *MockIPAM) GetPrefixesByRegionRole(region, role string) ([]models.Prefix, error) {
	return nil, nil
}

func (m *MockIPAM) GetPrefixesByPrefix(_ string) ([]models.Prefix, error) {
	return nil, nil
}

func (m *MockIPAM) CreateIPAddress(addr ipam.CreateIPAddressParams) (*models.IPAddress, error) {
	return nil, nil
}

func (m *MockIPAM) UpdateIPAddress(addr models.WriteableIPAddress) (*models.IPAddress, error) {
	return nil, nil
}

func (m *MockIPAM) DeleteIPAddress(id int) error {
	return nil
}

func (m *MockExtras) GetTagByName(tagName string) (*models.Tag, error) {
	return nil, nil
}

var _ = Describe("NetboxService", func() {
	var (
		mockVirtualization *MockVirtualization
		mockDCIM           *MockDCIM
		mockIPAM           *MockIPAM
		mockExtras         *MockExtras
		netboxService      Netbox
	)

	BeforeEach(func() {
		mockVirtualization = &MockVirtualization{}
		mockDCIM = &MockDCIM{}
		mockIPAM = &MockIPAM{}
		mockExtras = &MockExtras{}

		netboxService = &NetboxService{"", mockVirtualization, mockDCIM, mockIPAM, mockExtras}
	})

	Describe("Virtualization", func() {
		It("should return the correct virtualization service", func() {
			Expect(netboxService.Virtualization()).To(Equal(mockVirtualization))
		})
	})

	Describe("DCIM", func() {
		It("should return the correct DCIM service", func() {
			Expect(netboxService.DCIM()).To(Equal(mockDCIM))
		})
	})

	Describe("IPAM", func() {
		It("should return the correct IPAM service", func() {
			Expect(netboxService.IPAM()).To(Equal(mockIPAM))
		})
	})

	Describe("Extras", func() {
		It("should return the correct Extras service", func() {
			Expect(netboxService.Extras()).To(Equal(mockExtras))
		})
	})

	Describe("Reload", func() {
		It("should reload the services with new token", func() {
			token := "test-token"

			err := netboxService.Reload(token, logr.Discard())
			Expect(err).ToNot(HaveOccurred())

			Expect(netboxService.Virtualization()).ToNot(BeNil())
			Expect(netboxService.DCIM()).ToNot(BeNil())
			Expect(netboxService.IPAM()).ToNot(BeNil())
			Expect(netboxService.Extras()).ToNot(BeNil())
		})
	})
})
