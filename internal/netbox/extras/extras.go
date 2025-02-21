package extras

import (
	"fmt"

	"github.com/sapcc/go-netbox-go/models"
)

type ExtrasClient interface {
	ListTags(opts models.ListTagsRequest) (*models.ListTagsResponse, error)
}

type ExtrasCLientWrapper struct {
	client ExtrasClient
}

func NewExtrasCLientWrapper(client ExtrasClient) *ExtrasCLientWrapper {
	return &ExtrasCLientWrapper{client: client}
}

func (d *ExtrasCLientWrapper) ListDevices(opts models.ListTagsRequest) (*models.ListTagsResponse, error) {
	return d.client.ListTags(opts)
}

type Extras struct {
	client ExtrasClient
}

func NewExtras(client ExtrasClient) *Extras {
	return &Extras{client: client}
}

func (e *Extras) GetTagByName(tagName string) (*models.Tag, error) {
	listTagsRequest := NewListTagsRequest(
		WithName(tagName),
	).BuildRequest()

	res, err := e.client.ListTags(listTagsRequest)
	if err != nil {
		return nil, fmt.Errorf("unable to list tags by name %s: %w", tagName, err)
	}
	if res.Count != 1 {
		return nil, fmt.Errorf("unexpected number of tags found (%d)", res.Count)
	}
	return &res.Results[0], nil
}
