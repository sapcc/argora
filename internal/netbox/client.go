// Copyright 2024 SAP SE
// SPDX-License-Identifier: Apache-2.0

package netbox

import (
	"fmt"
	"sort"

	"github.com/sapcc/go-netbox-go/dcim"
	"github.com/sapcc/go-netbox-go/ipam"
	"github.com/sapcc/go-netbox-go/models"
	"github.com/sapcc/go-netbox-go/virtualization"
)

type Client struct {
	virt *virtualization.Client
	dcim *dcim.Client
	ipam *ipam.Client
}

func NewNetboxClient(url, token string) (*Client, error) {
	virt, err := virtualization.New(url, token, false)
	if err != nil {
		return nil, err
	}
	dcim, err := dcim.New(url, token, false)
	if err != nil {
		return nil, err
	}
	ipam, err := ipam.New(url, token, false)
	if err != nil {
		return nil, err
	}
	return &Client{
		virt: virt,
		dcim: dcim,
		ipam: ipam,
	}, nil
}

func (n *Client) GetRegionForDevice(device *models.Device) (string, error) {
	site, err := n.dcim.GetSite(device.Site.ID)
	if err != nil {
		return "", err
	}
	region, err := n.dcim.GetRegion(site.Region.ID)
	if err != nil {
		return "", err
	}
	return region.Slug, nil
}

func (n *Client) LookupVLANForDevice(device *models.Device, role string) (vlanid int, address string, err error) {
	lir := models.ListInterfacesRequest{
		DeviceID: device.ID,
	}
	lir.Name = "LAG1"
	resp, err := n.dcim.ListInterfaces(lir)
	if err != nil {
		return 0, "", err
	}
	if resp.Count == 0 {
		return 0, "", fmt.Errorf("no interfaces found for device %s", device.Name)
	}
	if resp.Count > 1 {
		return 0, "", fmt.Errorf("too many interfaces found for device %s", device.Name)
	}
	interf := resp.Results[0]

	lipr := models.ListIPAddressesRequest{
		InterfaceID: interf.ID,
	}
	res, err := n.ipam.ListIPAddresses(lipr)
	if err != nil {
		return 0, "", err
	}
	if res.Count == 0 {
		return 0, "", fmt.Errorf("no ip addresses found for device %s", device.Name)
	}
	if res.Count > 1 {
		return 0, "", fmt.Errorf("too many ip addresses found for device %s", device.Name)
	}
	prefix, err := n.ipam.ListPrefixes(models.ListPrefixesRequest{Contains: res.Results[0].Address})
	if err != nil {
		return 0, "", err
	}
	if prefix.Count == 0 {
		return 0, "", fmt.Errorf("no prefix found for device %s", device.Name)
	}

	for _, p := range prefix.Results {
		if p.Vlan.VID != 0 {
			return p.Vlan.VID, res.Results[0].Address, nil
		}
		// return 0, "", fmt.Fprintf("too many prefixes found for device %s", device.Name)
	}
	return 0, res.Results[0].Address, nil
}

// LookupMacForIP get the first interface of LAG1 and return the mac address
func (n *Client) LookupMacForIP(ipStr string) (string, error) {
	lipr := models.ListIPAddressesRequest{
		Address: ipStr,
	}
	resp, err := n.ipam.ListIPAddresses(lipr)
	if err != nil {
		return "", err
	}
	if resp.Count == 0 {
		return "", fmt.Errorf("no ip address found for ip %s", ipStr)
	}
	if resp.Count > 1 {
		return "", fmt.Errorf("too many ip addresses found for ip %s", ipStr)
	}

	ip := resp.Results[0]
	lir := models.ListInterfacesRequest{}
	lir.ID = ip.AssignedInterface.ID
	res, err := n.dcim.ListInterfaces(lir)
	if err != nil {
		return "", err
	}
	if res.Count == 0 {
		return "", fmt.Errorf("no interfaces found for ip %s", ipStr)
	}
	if res.Count > 1 {
		return "", fmt.Errorf("too many interfaces found for ip %s", ipStr)
	}
	// this is now the LAG interface we need to get the first interface of this LAG
	lir2 := models.ListInterfacesRequest{}
	lir2.LagID = res.Results[0].ID
	res2, err := n.dcim.ListInterfaces(lir2)
	if err != nil {
		return "", err
	}
	if res2.Count == 0 {
		return "", fmt.Errorf("no subinterfaces found for ip %s", ipStr)
	}
	macs := make(map[string]string)
	names := []string{}
	for _, i := range res2.Results {
		macs[i.Name] = i.MacAddress
		names = append(names, i.Name)
	}
	sort.Strings(names)
	return macs[names[0]], nil
}

func (n *Client) LookupCluster(role, region, name string) ([]models.Device, string, error) {
	lcp := models.ListClusterRequest{
		Type: role,
	}
	if region != "" {
		lcp.Region = region
	}
	if name != "" {
		lcp.Name = name
	}
	resp, err := n.virt.ListClusters(lcp)
	if err != nil {
		return nil, "", err
	}
	if resp.Count == 0 {
		return nil, "", nil
	}
	if resp.Count > 1 {
		return nil, "", fmt.Errorf("too many clusters found: %d", resp.Count)
	}
	cluster := resp.Results[0]
	ldp := models.ListDevicesRequest{
		ClusterID: cluster.ID,
	}
	dresp, err := n.dcim.ListDevices(ldp)
	if err != nil {
		return nil, "", err
	}
	return dresp.Results, cluster.Name, nil
}
