package dcim

import (
	"fmt"

	"github.com/sapcc/argora/internal/netbox"
	"github.com/sapcc/go-netbox-go/models"
)

type ListDevicesRequest struct {
	name      string
	id        int
	clusterID int
}

type ListDevicesRequestOption func(c *ListDevicesRequest)

func NewListDevicesRequest(opts ...ListDevicesRequestOption) *ListDevicesRequest {
	r := &ListDevicesRequest{}
	for _, opt := range opts {
		opt(r)
	}

	return r
}

func DeviceWithName(name string) ListDevicesRequestOption {
	opt := func(r *ListDevicesRequest) {
		r.name = name
	}

	return opt
}

func DeviceWithID(id int) ListDevicesRequestOption {
	opt := func(r *ListDevicesRequest) {
		r.id = id
	}

	return opt
}

func DeviceWithClusterID(clusterID int) ListDevicesRequestOption {
	opt := func(r *ListDevicesRequest) {
		r.clusterID = clusterID
	}

	return opt
}

func (r *ListDevicesRequest) BuildRequest() models.ListDevicesRequest {
	listDevicesRequest := models.ListDevicesRequest{}
	if r.name != "" {
		listDevicesRequest.Name = r.name
	}
	if r.id != 0 {
		listDevicesRequest.ID = r.id
	}
	if r.clusterID != 0 {
		listDevicesRequest.ClusterID = r.clusterID
	}
	return listDevicesRequest
}

type ListDeviceRolesRequest struct {
	name string
}

type ListDeviceRolesRequestOption func(c *ListDeviceRolesRequest)

func NewListDeviceRolesRequest(opts ...ListDeviceRolesRequestOption) *ListDeviceRolesRequest {
	r := &ListDeviceRolesRequest{}
	for _, opt := range opts {
		opt(r)
	}

	return r
}

func RoleWithName(name string) ListDeviceRolesRequestOption {
	opt := func(r *ListDeviceRolesRequest) {
		r.name = name
	}

	return opt
}

func (r *ListDeviceRolesRequest) BuildRequest() models.ListDeviceRolesRequest {
	listDeviceRolesRequest := models.ListDeviceRolesRequest{}
	if r.name != "" {
		listDeviceRolesRequest.Name = r.name
	}
	return listDeviceRolesRequest
}

type ListInterfacesRequest struct {
	name     string
	id       int
	deviceID int
	lagID    int
}

type ListInterfacesRequestOption func(c *ListInterfacesRequest)

func NewListInterfacesRequest(opts ...ListInterfacesRequestOption) *ListInterfacesRequest {
	r := &ListInterfacesRequest{}
	for _, opt := range opts {
		opt(r)
	}

	return r
}

func InterfaceWithName(name string) ListInterfacesRequestOption {
	opt := func(r *ListInterfacesRequest) {
		r.name = name
	}

	return opt
}

func InterfaceWithID(id int) ListInterfacesRequestOption {
	opt := func(r *ListInterfacesRequest) {
		r.id = id
	}

	return opt
}

func InterfaceWithDeviceID(deviceID int) ListInterfacesRequestOption {
	opt := func(r *ListInterfacesRequest) {
		r.deviceID = deviceID
	}

	return opt
}

func InterfaceWithLagID(lagID int) ListInterfacesRequestOption {
	opt := func(r *ListInterfacesRequest) {
		r.lagID = lagID
	}

	return opt
}

func (r *ListInterfacesRequest) BuildRequest() models.ListInterfacesRequest {
	listInterfacesRequest := models.ListInterfacesRequest{}
	if r.name != "" {
		listInterfacesRequest.Name = r.name
	}
	if r.id != 0 {
		listInterfacesRequest.ID = r.id
	}
	if r.deviceID != 0 {
		listInterfacesRequest.DeviceID = r.deviceID
	}
	if r.lagID != 0 {
		listInterfacesRequest.LagID = r.lagID
	}
	return listInterfacesRequest
}

type ListPlatformsRequest struct {
	name string
}

type ListPlatformsRequestOption func(c *ListPlatformsRequest)

func NewListPlatformsRequest(opts ...ListPlatformsRequestOption) *ListPlatformsRequest {
	r := &ListPlatformsRequest{}
	for _, opt := range opts {
		opt(r)
	}

	return r
}

func PlatformWithName(name string) ListPlatformsRequestOption {
	opt := func(r *ListPlatformsRequest) {
		r.name = name
	}

	return opt
}

func (r *ListPlatformsRequest) BuildRequest() models.ListPlatformsRequest {
	listPlatformRequest := models.ListPlatformsRequest{}
	if r.name != "" {
		listPlatformRequest.Name = r.name
	}
	return listPlatformRequest
}

type DCIM struct {
	client *netbox.NetboxClient
}

func NewDCIM(client *netbox.NetboxClient) *DCIM {
	return &DCIM{client: client}
}

func (d *DCIM) GetDeviceByName(deviceName string) (*models.Device, error) {
	listDevicesRequest := NewListDevicesRequest(
		DeviceWithName(deviceName),
	).BuildRequest()

	res, err := d.client.DCIM.ListDevices(listDevicesRequest)
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

	res, err := d.client.DCIM.ListDevices(listDevicesRequest)
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

	res, err := d.client.DCIM.ListDevices(listDevicesRequest)
	if err != nil {
		return nil, fmt.Errorf("unable to liste devices by cluster ID %d: %w", clusterID, err)
	}
	return res.Results, nil
}

func (d *DCIM) GetRoleByName(roleName string) (*models.DeviceRole, error) {
	listDeviceRolesRequest := NewListDeviceRolesRequest(
		RoleWithName(roleName),
	).BuildRequest()

	res, err := d.client.DCIM.ListDeviceRoles(listDeviceRolesRequest)
	if err != nil {
		return nil, fmt.Errorf("unable to list roles by name %s: %w", roleName, err)
	}
	if res.Count != 1 {
		return nil, fmt.Errorf("unexpected number of roles found (%d)", res.Count)
	}
	return &res.Results[0], nil
}

func (d *DCIM) GetRegionForDevice(device *models.Device) (string, error) {
	site, err := d.client.DCIM.GetSite(device.Site.ID)
	if err != nil {
		return "", fmt.Errorf("unable to get site for ID %d: %w", device.Site.ID, err)
	}
	region, err := d.client.DCIM.GetRegion(site.Region.ID)
	if err != nil {
		return "", fmt.Errorf("unable to get region for ID %d: %w", site.Region.ID, err)
	}
	return region.Slug, nil
}

func (d *DCIM) GetInterfaceByID(id int) (*models.Interface, error) {
	listInterfacesRequest := NewListInterfacesRequest(
		InterfaceWithID(id),
	).BuildRequest()

	rir, err := d.client.DCIM.ListInterfaces(listInterfacesRequest)
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

	rir, err := d.client.DCIM.ListInterfaces(listInterfacesRequest)
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

	rir, err := d.client.DCIM.ListInterfaces(listInterfacesRequest)
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

	rir, err := d.client.DCIM.ListInterfaces(listInterfacesRequest)
	if err != nil {
		return nil, fmt.Errorf("unable to list interfaces for LAG ID %d: %w", lagID, err)
	}
	return rir.Results, nil
}

func (d *DCIM) GetPlatformByName(platformName string) (*models.Platform, error) {
	listPlatformsRequest := NewListPlatformsRequest(
		PlatformWithName(platformName),
	).BuildRequest()

	res, err := d.client.DCIM.ListPlatforms(listPlatformsRequest)
	if err != nil {
		return nil, fmt.Errorf("unable to list platforms by name %s: %w", platformName, err)
	}
	if res.Count != 1 {
		return nil, fmt.Errorf("unexpected number of platforms found (%d)", res.Count)
	}
	return &res.Results[0], nil
}
