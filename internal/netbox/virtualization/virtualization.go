// Copyright 2025 SAP SE
// SPDX-License-Identifier: Apache-2.0

// Package virtualization provides the functionality to interact with the Virtualization API of Netbox.
package virtualization

import (
	"fmt"

	"github.com/sapcc/go-netbox-go/models"
	"github.com/sapcc/go-netbox-go/virtualization"
)

type Virtualization interface {
	GetClusterByName(clusterName string) (*models.Cluster, error)
	GetClusterByNameRegionType(name, region, clusterType string) (*models.Cluster, error)
}

type VirtualizationService struct {
	netboxAPI virtualization.NetboxAPI
}

func NewVirtualization(netboxAPI virtualization.NetboxAPI) Virtualization {
	return &VirtualizationService{netboxAPI: netboxAPI}
}

func (v *VirtualizationService) GetClusterByName(clusterName string) (*models.Cluster, error) {
	listClusterRequest := NewListClusterRequest(
		WithName(clusterName),
	).BuildRequest()

	res, err := v.netboxAPI.ListClusters(listClusterRequest)
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

	res, err := v.netboxAPI.ListClusters(listClusterRequest)
	if err != nil {
		return nil, err
	}
	if res.Count != 1 {
		return nil, fmt.Errorf("unexpected number of clusters found (%d)", res.Count)
	}
	return &res.Results[0], nil
}
