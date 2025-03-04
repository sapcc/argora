// Copyright 2025 SAP SE
// SPDX-License-Identifier: Apache-2.0

// Package dcim provides the functionality to interact with the DCIM API of Netbox.
package dcim

import (
	"fmt"

	"github.com/sapcc/go-netbox-go/models"
)

type DCIMClient interface {
	GetRegion(id int) (*models.Region, error)
	GetSite(id int) (*models.Site, error)

	ListDevices(opts models.ListDevicesRequest) (*models.ListDevicesResponse, error)
	ListDeviceRoles(opts models.ListDeviceRolesRequest) (*models.ListDeviceRolesResponse, error)
	ListInterfaces(opts models.ListInterfacesRequest) (*models.ListInterfacesResponse, error)
	ListPlatforms(opts models.ListPlatformsRequest) (*models.ListPlatformsResponse, error)

	UpdateDevice(device models.WritableDeviceWithConfigContext) (*models.Device, error)
	UpdateInterface(iface models.WritableInterface, id int) (*models.Interface, error)

	DeleteInterface(id int) error
}

type DCIMCLientWrapper struct {
	client DCIMClient
}

func NewDCIMCLientWrapper(client DCIMClient) *DCIMCLientWrapper {
	return &DCIMCLientWrapper{client: client}
}

func (d *DCIMCLientWrapper) GetRegion(id int) (*models.Region, error) {
	return d.client.GetRegion(id)
}

func (d *DCIMCLientWrapper) GetSite(id int) (*models.Site, error) {
	return d.client.GetSite(id)
}

func (d *DCIMCLientWrapper) ListDevices(opts models.ListDevicesRequest) (*models.ListDevicesResponse, error) {
	return d.client.ListDevices(opts)
}

func (d *DCIMCLientWrapper) ListDeviceRoles(opts models.ListDeviceRolesRequest) (*models.ListDeviceRolesResponse, error) {
	return d.client.ListDeviceRoles(opts)
}

func (d *DCIMCLientWrapper) ListInterfaces(opts models.ListInterfacesRequest) (*models.ListInterfacesResponse, error) {
	return d.client.ListInterfaces(opts)
}

func (d *DCIMCLientWrapper) ListPlatforms(opts models.ListPlatformsRequest) (*models.ListPlatformsResponse, error) {
	return d.client.ListPlatforms(opts)
}

func (d *DCIMCLientWrapper) UpdateDevice(device models.WritableDeviceWithConfigContext) (*models.Device, error) {
	return d.client.UpdateDevice(device)
}

func (d *DCIMCLientWrapper) UpdateInterface(iface models.WritableInterface, id int) (*models.Interface, error) {
	return d.client.UpdateInterface(iface, id)
}

func (d *DCIMCLientWrapper) DeleteInterface(id int) error {
	return d.client.DeleteInterface(id)
}

type DCIM interface {
	GetDeviceByName(deviceName string) (*models.Device, error)
	GetDeviceByID(id int) (*models.Device, error)
	GetDevicesByClusterID(clusterID int) ([]models.Device, error)
	GetRoleByName(roleName string) (*models.DeviceRole, error)
	GetRegionForDevice(device *models.Device) (string, error)
	GetInterfaceByID(id int) (*models.Interface, error)
	GetInterfacesForDevice(device *models.Device) ([]models.Interface, error)
	GetInterfaceForDevice(device *models.Device, ifaceName string) (*models.Interface, error)
	GetInterfacesByLagID(lagID int) ([]models.Interface, error)
	GetPlatformByName(platformName string) (*models.Platform, error)

	UpdateDevice(device models.WritableDeviceWithConfigContext) (*models.Device, error)
	UpdateInterface(iface models.WritableInterface, id int) (*models.Interface, error)

	DeleteInterface(id int) error
}

type DCIMService struct {
	client DCIMClient
}

func NewDCIM(client DCIMClient) DCIM {
	return &DCIMService{client: client}
}

func (d *DCIMService) GetDeviceByName(deviceName string) (*models.Device, error) {
	listDevicesRequest := NewListDevicesRequest(
		DeviceWithName(deviceName),
	).BuildRequest()

	res, err := d.client.ListDevices(listDevicesRequest)
	if err != nil {
		return nil, fmt.Errorf("unable to list devices by name %s: %w", deviceName, err)
	}
	if res.Count != 1 {
		return nil, fmt.Errorf("unexpected number of devices found (%d)", res.Count)
	}
	return &res.Results[0], nil
}

func (d *DCIMService) GetDeviceByID(id int) (*models.Device, error) {
	listDevicesRequest := NewListDevicesRequest(
		DeviceWithID(id),
	).BuildRequest()

	res, err := d.client.ListDevices(listDevicesRequest)
	if err != nil {
		return nil, fmt.Errorf("unable to list devices for id %d: %w", id, err)
	}
	if res.Count != 1 {
		return nil, fmt.Errorf("unexpected number of devices found (%d)", res.Count)
	}
	return &res.Results[0], nil
}

func (d *DCIMService) GetDevicesByClusterID(clusterID int) ([]models.Device, error) {
	listDevicesRequest := NewListDevicesRequest(
		DeviceWithClusterID(clusterID),
	).BuildRequest()

	res, err := d.client.ListDevices(listDevicesRequest)
	if err != nil {
		return nil, fmt.Errorf("unable to liste devices by cluster ID %d: %w", clusterID, err)
	}
	return res.Results, nil
}

func (d *DCIMService) GetRoleByName(roleName string) (*models.DeviceRole, error) {
	listDeviceRolesRequest := NewListDeviceRolesRequest(
		RoleWithName(roleName),
	).BuildRequest()

	res, err := d.client.ListDeviceRoles(listDeviceRolesRequest)
	if err != nil {
		return nil, fmt.Errorf("unable to list roles by name %s: %w", roleName, err)
	}
	if res.Count != 1 {
		return nil, fmt.Errorf("unexpected number of roles found (%d)", res.Count)
	}
	return &res.Results[0], nil
}

func (d *DCIMService) GetRegionForDevice(device *models.Device) (string, error) {
	site, err := d.client.GetSite(device.Site.ID)
	if err != nil {
		return "", fmt.Errorf("unable to get site for ID %d: %w", device.Site.ID, err)
	}
	region, err := d.client.GetRegion(site.Region.ID)
	if err != nil {
		return "", fmt.Errorf("unable to get region for ID %d: %w", site.Region.ID, err)
	}
	return region.Slug, nil
}

func (d *DCIMService) GetInterfaceByID(id int) (*models.Interface, error) {
	listInterfacesRequest := NewListInterfacesRequest(
		InterfaceWithID(id),
	).BuildRequest()

	rir, err := d.client.ListInterfaces(listInterfacesRequest)
	if err != nil {
		return nil, fmt.Errorf("unable to list interface for ID %d: %w", id, err)
	}
	if len(rir.Results) == 0 {
		return nil, fmt.Errorf("interface with ID %d not found", id)
	}
	return &rir.Results[0], nil
}

func (d *DCIMService) GetInterfacesForDevice(device *models.Device) ([]models.Interface, error) {
	listInterfacesRequest := NewListInterfacesRequest(
		InterfaceWithDeviceID(device.ID),
	).BuildRequest()

	rir, err := d.client.ListInterfaces(listInterfacesRequest)
	if err != nil {
		return nil, fmt.Errorf("unable to list interfaces for device: %s: %w", device.Name, err)
	}
	return rir.Results, nil
}

func (d *DCIMService) GetInterfaceForDevice(device *models.Device, ifaceName string) (*models.Interface, error) {
	listInterfacesRequest := NewListInterfacesRequest(
		InterfaceWithName(ifaceName),
		InterfaceWithDeviceID(device.ID),
	).BuildRequest()

	rir, err := d.client.ListInterfaces(listInterfacesRequest)
	if err != nil {
		return nil, fmt.Errorf("unable to list interfaces by name %s (device ID: %d): %w", ifaceName, device.ID, err)
	}
	if len(rir.Results) == 0 {
		return nil, fmt.Errorf("%s interface not found", ifaceName)
	}
	return &rir.Results[0], nil
}

func (d *DCIMService) GetInterfacesByLagID(lagID int) ([]models.Interface, error) {
	listInterfacesRequest := NewListInterfacesRequest(
		InterfaceWithLagID(lagID),
	).BuildRequest()

	rir, err := d.client.ListInterfaces(listInterfacesRequest)
	if err != nil {
		return nil, fmt.Errorf("unable to list interfaces for LAG ID %d: %w", lagID, err)
	}
	return rir.Results, nil
}

func (d *DCIMService) GetPlatformByName(platformName string) (*models.Platform, error) {
	listPlatformsRequest := NewListPlatformsRequest(
		PlatformWithName(platformName),
	).BuildRequest()

	res, err := d.client.ListPlatforms(listPlatformsRequest)
	if err != nil {
		return nil, fmt.Errorf("unable to list platforms by name %s: %w", platformName, err)
	}
	if res.Count != 1 {
		return nil, fmt.Errorf("unexpected number of platforms found (%d)", res.Count)
	}
	return &res.Results[0], nil
}

func (d *DCIMService) UpdateDevice(device models.WritableDeviceWithConfigContext) (*models.Device, error) {
	res, err := d.client.UpdateDevice(device)
	if err != nil {
		return nil, fmt.Errorf("unable to update device: %w", err)
	}
	return res, nil
}

func (d *DCIMService) UpdateInterface(iface models.WritableInterface, id int) (*models.Interface, error) {
	res, err := d.client.UpdateInterface(iface, id)
	if err != nil {
		return nil, fmt.Errorf("unable to update interface: %w", err)
	}
	return res, nil
}

func (d *DCIMService) DeleteInterface(id int) error {
	err := d.client.DeleteInterface(id)
	if err != nil {
		return fmt.Errorf("unable to delete interface (%d): %w", id, err)
	}
	return nil
}
