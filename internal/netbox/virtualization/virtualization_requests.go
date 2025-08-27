// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package virtualization

import (
	"github.com/sapcc/go-netbox-go/models"
)

type ListClusterRequest struct {
	clusterType, region, name string
}

type ListClusterOption func(c *ListClusterRequest)

func NewListClusterRequest(opts ...ListClusterOption) *ListClusterRequest {
	r := &ListClusterRequest{}
	for _, opt := range opts {
		opt(r)
	}

	return r
}

func WithName(name string) ListClusterOption {
	opt := func(r *ListClusterRequest) {
		r.name = name
	}

	return opt
}

func WithType(clusterType string) ListClusterOption {
	opt := func(r *ListClusterRequest) {
		r.clusterType = clusterType
	}

	return opt
}

func WithRegion(region string) ListClusterOption {
	opt := func(r *ListClusterRequest) {
		r.region = region
	}

	return opt
}

func (r *ListClusterRequest) BuildRequest() models.ListClusterRequest {
	listClusterRequest := models.ListClusterRequest{}
	if r.name != "" {
		listClusterRequest.Name = r.name
	}
	if r.region != "" {
		listClusterRequest.Region = r.region
	}
	if r.clusterType != "" {
		listClusterRequest.Type = r.clusterType
	}
	return listClusterRequest
}
