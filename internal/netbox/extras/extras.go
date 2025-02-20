package extras

import (
	"fmt"

	"github.com/sapcc/argora/internal/netbox"
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

type Extras struct {
	client *netbox.NetboxClient
}

func NewExtras(client *netbox.NetboxClient) *Extras {
	return &Extras{client: client}
}

func (e *Extras) GetTagByName(tagName string) (*models.Tag, error) {
	listTagsRequest := NewListTagsRequest(
		WithName(tagName),
	).BuildRequest()

	res, err := e.client.Extras.ListTags(listTagsRequest)
	if err != nil {
		return nil, fmt.Errorf("unable to list tags by name %s: %w", tagName, err)
	}
	if res.Count != 1 {
		return nil, fmt.Errorf("unexpected number of tags found (%d)", res.Count)
	}
	return &res.Results[0], nil
}
