// Copyright 2025 SAP SE
// SPDX-License-Identifier: Apache-2.0

// Package dcim provides the functionality to interact with the DCIM API of Netbox.
package dcim

import (
	"fmt"

	"github.com/sapcc/go-netbox-go/dcim"
	"github.com/sapcc/go-netbox-go/models"
)

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
	netboxAPI dcim.NetboxAPI
}

func NewDCIM(netboxAPI dcim.NetboxAPI) DCIM {
	return &DCIMService{netboxAPI: netboxAPI}
}

func (d *DCIMService) GetDeviceByName(deviceName string) (*models.Device, error) {
	listDevicesRequest := NewListDevicesRequest(
		DeviceWithName(deviceName),
	).BuildRequest()

	res, err := d.netboxAPI.ListDevices(listDevicesRequest)
	if err != nil {
		return nil, fmt.Errorf("unable to list devices by name %s: %w", deviceName, err)
	}
	if res.Count != 1 {
		return nil, fmt.Errorf("unexpected number of devices found by name %s: %d", deviceName, res.Count)
	}
	return &res.Results[0], nil
}

func (d *DCIMService) GetDeviceByID(id int) (*models.Device, error) {
	listDevicesRequest := NewListDevicesRequest(
		DeviceWithID(id),
	).BuildRequest()

	res, err := d.netboxAPI.ListDevices(listDevicesRequest)
	if err != nil {
		return nil, fmt.Errorf("unable to list devices for ID %d: %w", id, err)
	}
	if res.Count != 1 {
		return nil, fmt.Errorf("unexpected number of devices found for ID %d: %d", id, res.Count)
	}
	return &res.Results[0], nil
}

func (d *DCIMService) GetDevicesByClusterID(clusterID int) ([]models.Device, error) {
	listDevicesRequest := NewListDevicesRequest(
		DeviceWithClusterID(clusterID),
	).BuildRequest()

	res, err := d.netboxAPI.ListDevices(listDevicesRequest)
	if err != nil {
		return nil, fmt.Errorf("unable to liste devices by cluster ID %d: %w", clusterID, err)
	}
	return res.Results, nil
}

func (d *DCIMService) GetRoleByName(roleName string) (*models.DeviceRole, error) {
	listDeviceRolesRequest := NewListDeviceRolesRequest(
		RoleWithName(roleName),
	).BuildRequest()

	res, err := d.netboxAPI.ListDeviceRoles(listDeviceRolesRequest)
	if err != nil {
		return nil, fmt.Errorf("unable to list roles by name %s: %w", roleName, err)
	}
	if res.Count != 1 {
		return nil, fmt.Errorf("unexpected number of roles found by name %s: %d", roleName, res.Count)
	}
	return &res.Results[0], nil
}

func (d *DCIMService) GetRegionForDevice(device *models.Device) (string, error) {
	site, err := d.netboxAPI.GetSite(device.Site.ID)
	if err != nil {
		return "", fmt.Errorf("unable to get site for ID %d: %w", device.Site.ID, err)
	}
	region, err := d.netboxAPI.GetRegion(site.Region.ID)
	if err != nil {
		return "", fmt.Errorf("unable to get region for ID %d: %w", site.Region.ID, err)
	}
	return region.Slug, nil
}

func (d *DCIMService) GetInterfaceByID(id int) (*models.Interface, error) {
	listInterfacesRequest := NewListInterfacesRequest(
		InterfaceWithID(id),
	).BuildRequest()

	rir, err := d.netboxAPI.ListInterfaces(listInterfacesRequest)
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

	rir, err := d.netboxAPI.ListInterfaces(listInterfacesRequest)
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

	rir, err := d.netboxAPI.ListInterfaces(listInterfacesRequest)
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

	rir, err := d.netboxAPI.ListInterfaces(listInterfacesRequest)
	if err != nil {
		return nil, fmt.Errorf("unable to list interfaces for LAG ID %d: %w", lagID, err)
	}
	return rir.Results, nil
}

func (d *DCIMService) GetPlatformByName(platformName string) (*models.Platform, error) {
	listPlatformsRequest := NewListPlatformsRequest(
		PlatformWithName(platformName),
	).BuildRequest()

	res, err := d.netboxAPI.ListPlatforms(listPlatformsRequest)
	if err != nil {
		return nil, fmt.Errorf("unable to list platforms by name %s: %w", platformName, err)
	}
	if res.Count != 1 {
		return nil, fmt.Errorf("unexpected number of platforms found by name %s: %d", platformName, res.Count)
	}
	return &res.Results[0], nil
}

func (d *DCIMService) UpdateDevice(device models.WritableDeviceWithConfigContext) (*models.Device, error) {
	res, err := d.netboxAPI.UpdateDevice(device)
	if err != nil {
		return nil, fmt.Errorf("unable to update device: %w", err)
	}
	return res, nil
}

func (d *DCIMService) UpdateInterface(iface models.WritableInterface, id int) (*models.Interface, error) {
	res, err := d.netboxAPI.UpdateInterface(iface, id)
	if err != nil {
		return nil, fmt.Errorf("unable to update interface: %w", err)
	}
	return res, nil
}

func (d *DCIMService) DeleteInterface(id int) error {
	err := d.netboxAPI.DeleteInterface(id)
	if err != nil {
		return fmt.Errorf("unable to delete interface (%d): %w", id, err)
	}
	return nil
}
