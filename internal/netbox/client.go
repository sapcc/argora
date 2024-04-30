/*
 * Copyright (c) 2024. Lorem ipsum dolor sit amet, consectetur adipiscing elit.
 * Morbi non lorem porttitor neque feugiat blandit. Ut vitae ipsum eget quam lacinia accumsan.
 * Etiam sed turpis ac ipsum condimentum fringilla. Maecenas magna.
 * Proin dapibus sapien vel ante. Aliquam erat volutpat. Pellentesque sagittis ligula eget metus.
 * Vestibulum commodo. Ut rhoncus gravida arcu.
 */

package netbox

import (
	"fmt"
	"github.com/sapcc/go-netbox-go/dcim"
	"github.com/sapcc/go-netbox-go/ipam"
	"github.com/sapcc/go-netbox-go/models"
	"github.com/sapcc/go-netbox-go/virtualization"
)

type NetboxClient struct {
	virt *virtualization.Client
	dcim *dcim.Client
	ipam *ipam.Client
}

func NewNetboxClient(url string, token string) (*NetboxClient, error) {
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
	return &NetboxClient{
		virt: virt,
		dcim: dcim,
		ipam: ipam,
	}, nil
}

func (n *NetboxClient) LookupVLANForDevice(device *models.Device) (int, string, error) {
	lir := models.ListInterfacesRequest{
		DeviceId: device.Id,
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
	for _, nestedVlan := range interf.TaggedVlans {
		vlan, err := n.ipam.GetVlan(nestedVlan.Id)
		if err != nil {
			return 0, "", err
		}
		if vlan.Role.Slug == "cc-kubernetes-transit" {
			lipr := models.ListIpAddressesRequest{
				InterfaceId: interf.Id,
			}
			res, err := n.ipam.ListIpAddresses(lipr)
			return vlan.VId, "", nil
		}
	}
	return 0, "", fmt.Errorf("no vlan found for device %s", device.Name)
}

func (n *NetboxClient) LookupCluster(role string, name string) ([]models.Device, error) {
	lcp := models.ListClusterRequest{
		Type: role,
	}
	lcp.Name = name
	resp, err := n.virt.ListClusters(lcp)
	if err != nil {
		return nil, err
	}
	if resp.Count == 0 {
		return nil, nil
	}
	if resp.Count > 1 {
		return nil, fmt.Errorf("too many clusters found: %d", resp.Count)
	}
	cluster := resp.Results[0]
	ldp := models.ListDevicesRequest{
		ClusterId: cluster.Id,
	}
	dresp, err := n.dcim.ListDevices(ldp)
	if err != nil {
		return nil, err
	}
	return dresp.Results, nil
}
