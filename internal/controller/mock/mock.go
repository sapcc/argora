// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

// Package mock provides mock implementations for the netbox client and its services.
package mock

import (
	"errors"

	"github.com/go-logr/logr"
	"github.com/sapcc/go-netbox-go/models"

	"github.com/sapcc/argora/internal/netbox/dcim"
	"github.com/sapcc/argora/internal/netbox/extras"
	"github.com/sapcc/argora/internal/netbox/ipam"
	"github.com/sapcc/argora/internal/netbox/virtualization"
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

func (n *NetBoxMock) Reload(_ string, _ logr.Logger) error {
	if n.ReturnError {
		return errors.New("unable to reload netbox")
	}
	return nil
}

func (n *NetBoxMock) Virtualization() virtualization.Virtualization {
	return n.VirtualizationMock
}

func (n *NetBoxMock) SetVirtualization(_ virtualization.Virtualization) {
}

func (n *NetBoxMock) DCIM() dcim.DCIM {
	return n.DCIMMock
}

func (n *NetBoxMock) SetDCIM(_ dcim.DCIM) {
}

func (n *NetBoxMock) IPAM() ipam.IPAM {
	return n.IPAMMock
}

func (n *NetBoxMock) SetIPAM(_ ipam.IPAM) {
}

func (n *NetBoxMock) Extras() extras.Extras {
	return n.ExtrasMock
}

func (n *NetBoxMock) SetExtras(_ extras.Extras) {
}

type VirtualizationMock struct {
	GetClustersByNameRegionTypeFunc  func(name, region, clusterType string) ([]models.Cluster, error)
	GetClustersByNameRegionTypeCalls int
}

func (v *VirtualizationMock) GetClustersByNameRegionType(name, region, clusterType string) ([]models.Cluster, error) {
	v.GetClustersByNameRegionTypeCalls++
	return v.GetClustersByNameRegionTypeFunc(name, region, clusterType)
}

type DCIMMock struct {
	GetDeviceByNameFunc         func(deviceName string) (*models.Device, error)
	GetDeviceByNameCalls        int
	GetDeviceByIDFunc           func(id int) (*models.Device, error)
	GetDeviceByIDCalls          int
	GetDevicesByClusterIDFunc   func(clusterID int) ([]models.Device, error)
	GetDevicesByClusterIDCalls  int
	GetRoleByNameFunc           func(roleName string) (*models.DeviceRole, error)
	GetRoleByNameCalls          int
	GetRegionForDeviceFunc      func(device *models.Device) (string, error)
	GetRegionForDeviceCalls     int
	GetInterfaceByIDFunc        func(id int) (*models.Interface, error)
	GetInterfaceByIDCalls       int
	GetInterfacesForDeviceFunc  func(device *models.Device) ([]models.Interface, error)
	GetInterfacesForDeviceCalls int
	GetInterfaceForDeviceFunc   func(device *models.Device, ifaceName string) (*models.Interface, error)
	GetInterfaceForDeviceCalls  int
	GetInterfacesByLagIDFunc    func(lagID int) ([]models.Interface, error)
	GetInterfacesByLagIDCalls   int
	GetPlatformByNameFunc       func(platformName string) (*models.Platform, error)
	GetPlatformByNameCalls      int

	UpdateDeviceFunc     func(device models.WritableDeviceWithConfigContext) (*models.Device, error)
	UpdateDeviceCalls    int
	UpdateInterfaceFunc  func(iface models.WritableInterface, id int) (*models.Interface, error)
	UpdateInterfaceCalls int

	DeleteInterfaceFunc  func(id int) error
	DeleteInterfaceCalls int
}

func (d *DCIMMock) GetDeviceByName(deviceName string) (*models.Device, error) {
	d.GetDeviceByNameCalls++
	return d.GetDeviceByNameFunc(deviceName)
}

func (d *DCIMMock) GetDeviceByID(id int) (*models.Device, error) {
	d.GetDeviceByIDCalls++
	return d.GetDeviceByIDFunc(id)
}

func (d *DCIMMock) GetDevicesByClusterID(clusterID int) ([]models.Device, error) {
	d.GetDevicesByClusterIDCalls++
	return d.GetDevicesByClusterIDFunc(clusterID)
}

func (d *DCIMMock) GetRoleByName(roleName string) (*models.DeviceRole, error) {
	d.GetRoleByNameCalls++
	return d.GetRoleByNameFunc(roleName)
}

func (d *DCIMMock) GetRegionForDevice(device *models.Device) (string, error) {
	d.GetRegionForDeviceCalls++
	return d.GetRegionForDeviceFunc(device)
}

func (d *DCIMMock) GetInterfaceByID(id int) (*models.Interface, error) {
	d.GetInterfaceByIDCalls++
	return d.GetInterfaceByIDFunc(id)
}

func (d *DCIMMock) GetInterfacesForDevice(device *models.Device) ([]models.Interface, error) {
	d.GetInterfacesForDeviceCalls++
	return d.GetInterfacesForDeviceFunc(device)
}

func (d *DCIMMock) GetInterfaceForDevice(device *models.Device, ifaceName string) (*models.Interface, error) {
	d.GetInterfaceForDeviceCalls++
	return d.GetInterfaceForDeviceFunc(device, ifaceName)
}

func (d *DCIMMock) GetInterfacesByLagID(lagID int) ([]models.Interface, error) {
	d.GetInterfacesByLagIDCalls++
	return d.GetInterfacesByLagIDFunc(lagID)
}

func (d *DCIMMock) GetPlatformByName(platformName string) (*models.Platform, error) {
	d.GetPlatformByNameCalls++
	return d.GetPlatformByNameFunc(platformName)
}

func (d *DCIMMock) UpdateDevice(device models.WritableDeviceWithConfigContext) (*models.Device, error) {
	d.UpdateDeviceCalls++
	return d.UpdateDeviceFunc(device)
}

func (d *DCIMMock) UpdateInterface(iface models.WritableInterface, id int) (*models.Interface, error) {
	d.UpdateInterfaceCalls++
	return d.UpdateInterfaceFunc(iface, id)
}

func (d *DCIMMock) DeleteInterface(id int) error {
	d.DeleteInterfaceCalls++
	return d.DeleteInterfaceFunc(id)
}

type IPAMMock struct {
	GetVlanByNameFunc               func(vlanName string) (*models.Vlan, error)
	GetVlanByNameCalls              int
	GetIPAddressByAddressFunc       func(address string) (*models.IPAddress, error)
	GetIPAddressByAddressCalls      int
	GetIPAddressesForInterfaceFunc  func(interfaceID int) ([]models.IPAddress, error)
	GetIPAddressesForInterfaceCalls int
	GetIPAddressForInterfaceFunc    func(interfaceID int) (*models.IPAddress, error)
	GetIPAddressForInterfaceCalls   int
	GetPrefixesContainingFunc       func(contains string) ([]models.Prefix, error)
	GetPrefixesContainingCalls      int

	DeleteIPAddressFunc  func(id int) error
	DeleteIPAddressCalls int
}

func (i *IPAMMock) GetVlanByName(vlanName string) (*models.Vlan, error) {
	i.GetVlanByNameCalls++
	return i.GetVlanByNameFunc(vlanName)
}

func (i *IPAMMock) GetIPAddressByAddress(address string) (*models.IPAddress, error) {
	i.GetIPAddressByAddressCalls++
	return i.GetIPAddressByAddressFunc(address)
}

func (i *IPAMMock) GetIPAddressesForInterface(interfaceID int) ([]models.IPAddress, error) {
	i.GetIPAddressesForInterfaceCalls++
	return i.GetIPAddressesForInterfaceFunc(interfaceID)
}

func (i *IPAMMock) GetIPAddressForInterface(interfaceID int) (*models.IPAddress, error) {
	i.GetIPAddressForInterfaceCalls++
	return i.GetIPAddressForInterfaceFunc(interfaceID)
}

func (i *IPAMMock) GetPrefixesContaining(contains string) ([]models.Prefix, error) {
	i.GetPrefixesContainingCalls++
	return i.GetPrefixesContainingFunc(contains)
}

func (i *IPAMMock) DeleteIPAddress(id int) error {
	i.DeleteIPAddressCalls++
	return i.DeleteIPAddressFunc(id)
}

type ExtrasMock struct {
	GetTagByNameFunc  func(tagName string) (*models.Tag, error)
	GetTagByNameCalls int
}

func (e *ExtrasMock) GetTagByName(tagName string) (*models.Tag, error) {
	e.GetTagByNameCalls++
	return e.GetTagByNameFunc(tagName)
}
