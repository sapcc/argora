// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

// Package virtualization provides the functionality to interact with the Virtualization API of Netbox.
package virtualization

import (
	"errors"

	"github.com/go-logr/logr"
	"github.com/sapcc/go-netbox-go/models"
	"github.com/sapcc/go-netbox-go/virtualization"
)

type Virtualization interface {
	GetClustersByNameRegionType(name, region, clusterType string) ([]models.Cluster, error)
}

type VirtualizationService struct {
	netboxAPI virtualization.NetboxAPI
	logger    logr.Logger
}

func NewVirtualization(netboxAPI virtualization.NetboxAPI, logger logr.Logger) Virtualization {
	return &VirtualizationService{netboxAPI, logger}
}

func (v *VirtualizationService) GetClustersByNameRegionType(name, region, clusterType string) ([]models.Cluster, error) {
	listClusterRequest := NewListClusterRequest(
		WithName(name),
		WithRegion(region),
		WithType(clusterType),
	).BuildRequest()
	v.logger.V(1).Info("list clusters", "request", listClusterRequest)
	res, err := v.netboxAPI.ListClusters(listClusterRequest)
	if err != nil {
		return nil, err
	}
	if res.Count == 0 {
		return nil, errors.New("no clusters found")
	}
	return res.Results, nil
}
