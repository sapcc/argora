// Copyright 2025 SAP SE
// SPDX-License-Identifier: Apache-2.0

package extras

import (
	"github.com/sapcc/go-netbox-go/models"
)

type ListTagsRequest struct {
	name string
}

type ListClusterOption func(c *ListTagsRequest)

func NewListTagsRequest(opts ...ListClusterOption) *ListTagsRequest {
	r := &ListTagsRequest{}
	for _, opt := range opts {
		opt(r)
	}

	return r
}

func WithName(name string) ListClusterOption {
	opt := func(r *ListTagsRequest) {
		r.name = name
	}

	return opt
}

func (r *ListTagsRequest) BuildRequest() models.ListTagsRequest {
	listTagsRequest := models.ListTagsRequest{}
	if r.name != "" {
		listTagsRequest.Name = r.name
	}
	return listTagsRequest
}
