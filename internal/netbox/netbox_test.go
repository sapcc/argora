package netbox_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sapcc/argora/internal/netbox"
	"github.com/sapcc/go-netbox-go/models"
)

func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Netbox Suite")
}

type MockVirtualization struct{}
type MockDCIM struct{}
type MockIPAM struct{}
type MockExtras struct{}

func (m *MockVirtualization) GetClusterByName(clusterName string) (*models.Cluster, error) {
	return nil, nil
}

func (m *MockVirtualization) GetClusterByNameRegionType(name, region, clusterType string) (*models.Cluster, error) {
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
		netboxService      netbox.Netbox
	)

	BeforeEach(func() {
		mockVirtualization = &MockVirtualization{}
		mockDCIM = &MockDCIM{}
		mockIPAM = &MockIPAM{}
		mockExtras = &MockExtras{}

		netboxService = netbox.NewNetbox()
		netboxService.SetVirtualization(mockVirtualization)
		netboxService.SetDCIM(mockDCIM)
		netboxService.SetIPAM(mockIPAM)
		netboxService.SetExtras(mockExtras)
	})

	Describe("Virtualization", func() {
		It("should return the correct virtualization service", func() {
			Expect(netboxService.Virtualization()).To(Equal(mockVirtualization))
		})

		It("should set the correct virtualization service", func() {
			newMockVirtualization := &MockVirtualization{}
			netboxService.SetVirtualization(newMockVirtualization)
			Expect(netboxService.Virtualization()).To(Equal(newMockVirtualization))
		})
	})

	Describe("DCIM", func() {
		It("should return the correct DCIM service", func() {
			Expect(netboxService.DCIM()).To(Equal(mockDCIM))
		})

		It("should set the correct DCIM service", func() {
			newMockDCIM := &MockDCIM{}
			netboxService.SetDCIM(newMockDCIM)
			Expect(netboxService.DCIM()).To(Equal(newMockDCIM))
		})
	})

	Describe("IPAM", func() {
		It("should return the correct IPAM service", func() {
			Expect(netboxService.IPAM()).To(Equal(mockIPAM))
		})

		It("should set the correct IPAM service", func() {
			newMockIPAM := &MockIPAM{}
			netboxService.SetIPAM(newMockIPAM)
			Expect(netboxService.IPAM()).To(Equal(newMockIPAM))
		})
	})

	Describe("Extras", func() {
		It("should return the correct Extras service", func() {
			Expect(netboxService.Extras()).To(Equal(mockExtras))
		})

		It("should set the correct Extras service", func() {
			newMockExtras := &MockExtras{}
			netboxService.SetExtras(newMockExtras)
			Expect(netboxService.Extras()).To(Equal(newMockExtras))
		})
	})

	Describe("NewDefaultNetbox", func() {
		It("should return a new NetboxService with the correct services", func() {
			url := "http://example.com"
			token := "test-token"

			netboxService, err := netbox.NewDefaultNetbox(url, token)
			Expect(err).NotTo(HaveOccurred())
			Expect(netboxService).NotTo(BeNil())
			Expect(netboxService.Virtualization()).NotTo(BeNil())
			Expect(netboxService.DCIM()).NotTo(BeNil())
			Expect(netboxService.IPAM()).NotTo(BeNil())
			Expect(netboxService.Extras()).NotTo(BeNil())
		})
	})

	Describe("Reload", func() {
		It("should reload the services with new configurations", func() {
			url := "http://example.com"
			token := "test-token"

			err := netboxService.Reload(url, token)
			Expect(err).NotTo(HaveOccurred())
			Expect(netboxService.Virtualization()).NotTo(BeNil())
			Expect(netboxService.DCIM()).NotTo(BeNil())
			Expect(netboxService.IPAM()).NotTo(BeNil())
			Expect(netboxService.Extras()).NotTo(BeNil())
		})
	})
})
