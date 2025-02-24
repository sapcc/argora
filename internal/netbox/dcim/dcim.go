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

	UpdateDevice(dev models.WritableDeviceWithConfigContext) (*models.Device, error)
}

type DCIMCLientWrapper struct {
	client DCIMClient
}

func NewDCIMCLientWrapper(client DCIMClient) *DCIMCLientWrapper {
	return &DCIMCLientWrapper{client: client}
}

func (d *DCIMCLientWrapper) ListDevices(opts models.ListDevicesRequest) (*models.ListDevicesResponse, error) {
	return d.client.ListDevices(opts)
}

func (d *DCIMCLientWrapper) ListDeviceRoles(opts models.ListDeviceRolesRequest) (*models.ListDeviceRolesResponse, error) {
	return d.client.ListDeviceRoles(opts)
}

func (d *DCIMCLientWrapper) GetRegion(id int) (*models.Region, error) {
	return d.client.GetRegion(id)
}

func (d *DCIMCLientWrapper) GetSite(id int) (*models.Site, error) {
	return d.client.GetSite(id)
}

func (d *DCIMCLientWrapper) ListInterfaces(opts models.ListInterfacesRequest) (*models.ListInterfacesResponse, error) {
	return d.client.ListInterfaces(opts)
}

func (d *DCIMCLientWrapper) ListPlatforms(opts models.ListPlatformsRequest) (*models.ListPlatformsResponse, error) {
	return d.client.ListPlatforms(opts)
}

type DCIM struct {
	client DCIMClient
}

func NewDCIM(client DCIMClient) *DCIM {
	return &DCIM{client: client}
}

func (d *DCIM) GetDeviceByName(deviceName string) (*models.Device, error) {
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

func (d *DCIM) GetDeviceByID(id int) (*models.Device, error) {
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

func (d *DCIM) GetDevicesByClusterID(clusterID int) ([]models.Device, error) {
	listDevicesRequest := NewListDevicesRequest(
		DeviceWithClusterID(clusterID),
	).BuildRequest()

	res, err := d.client.ListDevices(listDevicesRequest)
	if err != nil {
		return nil, fmt.Errorf("unable to liste devices by cluster ID %d: %w", clusterID, err)
	}
	return res.Results, nil
}

func (d *DCIM) GetRoleByName(roleName string) (*models.DeviceRole, error) {
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

func (d *DCIM) GetRegionForDevice(device *models.Device) (string, error) {
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

func (d *DCIM) GetInterfaceByID(id int) (*models.Interface, error) {
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

func (d *DCIM) GetInterfacesForDevice(device *models.Device) ([]models.Interface, error) {
	listInterfacesRequest := NewListInterfacesRequest(
		InterfaceWithDeviceID(device.ID),
	).BuildRequest()

	rir, err := d.client.ListInterfaces(listInterfacesRequest)
	if err != nil {
		return nil, fmt.Errorf("unable to list interfaces for device: %s: %w", device.Name, err)
	}
	return rir.Results, nil
}

func (d *DCIM) GetInterfaceForDevice(device *models.Device, ifaceName string) (*models.Interface, error) {
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

func (d *DCIM) GetInterfacesByLagID(lagID int) ([]models.Interface, error) {
	listInterfacesRequest := NewListInterfacesRequest(
		InterfaceWithLagID(lagID),
	).BuildRequest()

	rir, err := d.client.ListInterfaces(listInterfacesRequest)
	if err != nil {
		return nil, fmt.Errorf("unable to list interfaces for LAG ID %d: %w", lagID, err)
	}
	return rir.Results, nil
}

func (d *DCIM) GetPlatformByName(platformName string) (*models.Platform, error) {
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

func (d *DCIM) UpdateDevice(dev models.WritableDeviceWithConfigContext) (*models.Device, error) {
	res, err := d.client.UpdateDevice(dev)
	if err != nil {
		return nil, fmt.Errorf("unable to update device: %w", err)
	}
	return res, nil
}
