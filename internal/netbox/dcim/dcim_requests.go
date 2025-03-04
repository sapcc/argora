// Copyright 2025 SAP SE
// SPDX-License-Identifier: Apache-2.0

package dcim

import (
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
