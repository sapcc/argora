// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package ipam

import (
	"github.com/sapcc/go-netbox-go/models"
)

type ListVlanRequest struct {
	name string
}

type ListVlanRequestOption func(c *ListVlanRequest)

func NewListVlanRequest(opts ...ListVlanRequestOption) *ListVlanRequest {
	r := &ListVlanRequest{}
	for _, opt := range opts {
		opt(r)
	}

	return r
}

func VlanWithName(name string) ListVlanRequestOption {
	opt := func(r *ListVlanRequest) {
		r.name = name
	}

	return opt
}

func (r *ListVlanRequest) BuildRequest() models.ListVlanRequest {
	listVlanRequest := models.ListVlanRequest{}
	if r.name != "" {
		listVlanRequest.Name = r.name
	}
	return listVlanRequest
}

type ListIPAddressesRequest struct {
	interfaceID int
	address     string
}

type ListIPAddressesRequestOption func(c *ListIPAddressesRequest)

func NewListIPAddressesRequest(opts ...ListIPAddressesRequestOption) *ListIPAddressesRequest {
	r := &ListIPAddressesRequest{}
	for _, opt := range opts {
		opt(r)
	}

	return r
}

func IPAddressesWithInterfaceID(interfaceID int) ListIPAddressesRequestOption {
	opt := func(r *ListIPAddressesRequest) {
		r.interfaceID = interfaceID
	}

	return opt
}

func IPAddressesWithAddress(address string) ListIPAddressesRequestOption {
	opt := func(r *ListIPAddressesRequest) {
		r.address = address
	}

	return opt
}

func (r *ListIPAddressesRequest) BuildRequest() models.ListIPAddressesRequest {
	listIPAddressesRequest := models.ListIPAddressesRequest{}
	if r.interfaceID != 0 {
		listIPAddressesRequest.InterfaceID = r.interfaceID
	}
	if r.address != "" {
		listIPAddressesRequest.Address = r.address
	}
	return listIPAddressesRequest
}

type ListPrefixesRequest struct {
	contains string
}

type ListPrefixesRequestOption func(c *ListPrefixesRequest)

func NewListPrefixesRequest(opts ...ListPrefixesRequestOption) *ListPrefixesRequest {
	r := &ListPrefixesRequest{}
	for _, opt := range opts {
		opt(r)
	}

	return r
}

func PrefixWithContains(contains string) ListPrefixesRequestOption {
	opt := func(r *ListPrefixesRequest) {
		r.contains = contains
	}

	return opt
}

func (r *ListPrefixesRequest) BuildRequest() models.ListPrefixesRequest {
	listPrefixesRequest := models.ListPrefixesRequest{}
	if r.contains != "" {
		listPrefixesRequest.Contains = r.contains
	}
	return listPrefixesRequest
}
