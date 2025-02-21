package virtualization

import (
	"github.com/sapcc/go-netbox-go/models"
)

type ListClusterRequest struct {
	role, region, name string
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

func WithRole(role string) ListClusterOption {
	opt := func(r *ListClusterRequest) {
		r.role = role
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
	if r.role != "" {
		listClusterRequest.Type = r.role
	}
	return listClusterRequest
}
