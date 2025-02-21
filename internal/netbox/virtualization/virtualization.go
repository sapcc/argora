package virtualization

import (
	"fmt"

	"github.com/sapcc/go-netbox-go/models"
)

type VirtualizationClient interface {
	ListClusters(opts models.ListClusterRequest) (*models.ListClusterResponse, error)
}

func (vcw *VirtualizationCLientWrapper) ListClusters(opts models.ListClusterRequest) (*models.ListClusterResponse, error) {
	return vcw.client.ListClusters(opts)
}

type VirtualizationCLientWrapper struct {
	client VirtualizationClient
}

func NewVirtualizationCLientWrapper(client VirtualizationClient) *VirtualizationCLientWrapper {
	return &VirtualizationCLientWrapper{client: client}
}

type Virtualization struct {
	client VirtualizationClient
}

func NewVirtualization(client VirtualizationClient) *Virtualization {
	return &Virtualization{client: client}
}

func (v *Virtualization) GetClusterByName(clusterName string) (*models.Cluster, error) {
	listClusterRequest := NewListClusterRequest(
		WithName(clusterName),
	).BuildRequest()

	res, err := v.client.ListClusters(listClusterRequest)
	if err != nil {
		return nil, fmt.Errorf("unable to list clusters by name %s: %w", clusterName, err)
	}

	if res.Count != 1 {
		return nil, fmt.Errorf("unexpected number of clusters found (%d)", res.Count)
	}
	return &res.Results[0], nil
}

func (v *Virtualization) GetClusterByNameRegionRole(name, region, role string) (*models.Cluster, error) {
	listClusterRequest := NewListClusterRequest(
		WithName(name),
		WithRegion(region),
		WithRole(role),
	).BuildRequest()
	res, err := v.client.ListClusters(listClusterRequest)
	if err != nil {
		return nil, err
	}
	if res.Count != 1 {
		return nil, fmt.Errorf("unexpected number of clusters found (%d)", res.Count)
	}
	return &res.Results[0], nil
}
