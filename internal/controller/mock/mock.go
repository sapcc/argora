package mock

import (
	"errors"

	"github.com/sapcc/argora/internal/netbox/dcim"
	"github.com/sapcc/argora/internal/netbox/extras"
	"github.com/sapcc/argora/internal/netbox/ipam"
	"github.com/sapcc/argora/internal/netbox/virtualization"
	"github.com/sapcc/go-netbox-go/models"
)

type FileReaderMock struct {
	FileContent map[string]string
	ReturnError bool
}

func (f *FileReaderMock) ReadFile(fileName string) ([]byte, error) {
	if f.ReturnError {
		return nil, errors.New("error")
	}
	return []byte(f.FileContent[fileName]), nil
}

type NetBoxMock struct {
	ReturnError        bool
	VirtualizationMock virtualization.Virtualization
	DCIMMock           dcim.DCIM
	IPAMMock           ipam.IPAM
	ExtrasMock         extras.Extras
}

func (n *NetBoxMock) Reload(_, _ string) error {
	if n.ReturnError {
		return errors.New("error")
	}
	return nil
}

func (n *NetBoxMock) Virtualization() virtualization.Virtualization {
	return n.VirtualizationMock
}

func (n *NetBoxMock) SetVirtualization(virtualization virtualization.Virtualization) {
}

func (n *NetBoxMock) DCIM() dcim.DCIM {
	return n.DCIMMock
}

func (n *NetBoxMock) SetDCIM(dcim dcim.DCIM) {
}

func (n *NetBoxMock) IPAM() ipam.IPAM {
	return n.IPAMMock
}

func (n *NetBoxMock) SetIPAM(ipam ipam.IPAM) {
}

func (n *NetBoxMock) Extras() extras.Extras {
	return n.ExtrasMock
}

func (n *NetBoxMock) SetExtras(extras extras.Extras) {
}

type VirtualizationMock struct {
	GetClusterByNameFunc           func(clusterName string) (*models.Cluster, error)
	GetClusterByNameRegionTypeFunc func(name, region, clusterType string) (*models.Cluster, error)
}

func (v *VirtualizationMock) GetClusterByName(clusterName string) (*models.Cluster, error) {
	return v.GetClusterByNameFunc(clusterName)
}

func (v *VirtualizationMock) GetClusterByNameRegionType(name, region, clusterType string) (*models.Cluster, error) {
	return v.GetClusterByNameRegionTypeFunc(name, region, clusterType)
}

type DCIMMock struct {
	GetDeviceByNameFunc        func(deviceName string) (*models.Device, error)
	GetDeviceByIDFunc          func(id int) (*models.Device, error)
	GetDevicesByClusterIDFunc  func(clusterID int) ([]models.Device, error)
	GetRoleByNameFunc          func(roleName string) (*models.DeviceRole, error)
	GetRegionForDeviceFunc     func(device *models.Device) (string, error)
	GetInterfaceByIDFunc       func(id int) (*models.Interface, error)
	GetInterfacesForDeviceFunc func(device *models.Device) ([]models.Interface, error)
	GetInterfaceForDeviceFunc  func(device *models.Device, ifaceName string) (*models.Interface, error)
	GetInterfacesByLagIDFunc   func(lagID int) ([]models.Interface, error)
	GetPlatformByNameFunc      func(platformName string) (*models.Platform, error)

	UpdateDeviceFunc    func(device models.WritableDeviceWithConfigContext) (*models.Device, error)
	UpdateInterfaceFunc func(iface models.WritableInterface, id int) (*models.Interface, error)

	DeleteInterfaceFunc func(id int) error
}

func (d *DCIMMock) GetDeviceByName(deviceName string) (*models.Device, error) {
	return d.GetDeviceByNameFunc(deviceName)
}

func (d *DCIMMock) GetDeviceByID(id int) (*models.Device, error) {
	return d.GetDeviceByIDFunc(id)
}

func (d *DCIMMock) GetDevicesByClusterID(clusterID int) ([]models.Device, error) {
	return d.GetDevicesByClusterIDFunc(clusterID)
}

func (d *DCIMMock) GetRoleByName(roleName string) (*models.DeviceRole, error) {
	return d.GetRoleByNameFunc(roleName)
}

func (d *DCIMMock) GetRegionForDevice(device *models.Device) (string, error) {
	return d.GetRegionForDeviceFunc(device)
}

func (d *DCIMMock) GetInterfaceByID(id int) (*models.Interface, error) {
	return d.GetInterfaceByIDFunc(id)
}

func (d *DCIMMock) GetInterfacesForDevice(device *models.Device) ([]models.Interface, error) {
	return d.GetInterfacesForDeviceFunc(device)
}

func (d *DCIMMock) GetInterfaceForDevice(device *models.Device, ifaceName string) (*models.Interface, error) {
	return d.GetInterfaceForDeviceFunc(device, ifaceName)
}

func (d *DCIMMock) GetInterfacesByLagID(lagID int) ([]models.Interface, error) {
	return d.GetInterfacesByLagIDFunc(lagID)
}

func (d *DCIMMock) GetPlatformByName(platformName string) (*models.Platform, error) {
	return d.GetPlatformByNameFunc(platformName)
}

func (d *DCIMMock) UpdateDevice(device models.WritableDeviceWithConfigContext) (*models.Device, error) {
	return d.UpdateDeviceFunc(device)
}

func (d *DCIMMock) UpdateInterface(iface models.WritableInterface, id int) (*models.Interface, error) {
	return d.UpdateInterfaceFunc(iface, id)
}

func (d *DCIMMock) DeleteInterface(id int) error {
	return d.DeleteInterfaceFunc(id)
}

type IPAMMock struct {
	GetVlanByNameFunc              func(vlanName string) (*models.Vlan, error)
	GetIPAddressByAddressFunc      func(address string) (*models.IPAddress, error)
	GetIPAddressesForInterfaceFunc func(interfaceID int) ([]models.IPAddress, error)
	GetIPAddressForInterfaceFunc   func(interfaceID int) (*models.IPAddress, error)
	GetPrefixesContainingFunc      func(contains string) ([]models.Prefix, error)

	DeleteIPAddressFunc func(id int) error
}

func (i *IPAMMock) GetVlanByName(vlanName string) (*models.Vlan, error) {
	return i.GetVlanByNameFunc(vlanName)
}

func (i *IPAMMock) GetIPAddressByAddress(address string) (*models.IPAddress, error) {
	return i.GetIPAddressByAddressFunc(address)
}

func (i *IPAMMock) GetIPAddressesForInterface(interfaceID int) ([]models.IPAddress, error) {
	return i.GetIPAddressesForInterfaceFunc(interfaceID)
}

func (i *IPAMMock) GetIPAddressForInterface(interfaceID int) (*models.IPAddress, error) {
	return i.GetIPAddressForInterfaceFunc(interfaceID)
}

func (i *IPAMMock) GetPrefixesContaining(contains string) ([]models.Prefix, error) {
	return i.GetPrefixesContainingFunc(contains)
}

func (i *IPAMMock) DeleteIPAddress(id int) error {
	return i.DeleteIPAddressFunc(id)
}

type ExtrasMock struct {
	GetTagByNameFunc func(tagName string) (*models.Tag, error)
}

func (e *ExtrasMock) GetTagByName(tagName string) (*models.Tag, error) {
	return e.GetTagByNameFunc(tagName)
}
