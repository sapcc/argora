package netbox

import (
	"fmt"
	"github.com/sapcc/go-netbox-go/dcim"
	"github.com/sapcc/go-netbox-go/models"
	"github.com/sapcc/go-netbox-go/virtualization"
)

type NetboxClient struct {
	virt *virtualization.Client
	dcim *dcim.Client
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
	return &NetboxClient{
		virt: virt,
		dcim: dcim,
	}, nil
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
