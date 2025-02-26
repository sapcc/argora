package virtualization

import (
	"fmt"

	"github.com/sapcc/go-netbox-go/models"
)

type VirtualizationClient interface {
	ListClusters(opts models.ListClusterRequest) (*models.ListClusterResponse, error)
}

func (vcw *VirtualizationClientWrapper) ListClusters(opts models.ListClusterRequest) (*models.ListClusterResponse, error) {
	return vcw.client.ListClusters(opts)
}

type VirtualizationClientWrapper struct {
	client VirtualizationClient
}

func NewVirtualizationClientWrapper(client VirtualizationClient) *VirtualizationClientWrapper {
	return &VirtualizationClientWrapper{client: client}
}

type Virtualization interface {
	GetClusterByName(clusterName string) (*models.Cluster, error)
	GetClusterByNameRegionType(name, region, clusterType string) (*models.Cluster, error)
}

type VirtualizationService struct {
	client VirtualizationClient
}

func NewVirtualization(client VirtualizationClient) Virtualization {
	return &VirtualizationService{client: client}
}

func (v *VirtualizationService) GetClusterByName(clusterName string) (*models.Cluster, error) {
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

func (v *VirtualizationService) GetClusterByNameRegionType(name, region, clusterType string) (*models.Cluster, error) {
	listClusterRequest := NewListClusterRequest(
		WithName(name),
		WithRegion(region),
		WithType(clusterType),
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
