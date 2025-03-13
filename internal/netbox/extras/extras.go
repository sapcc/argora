// Copyright 2025 SAP SE
// SPDX-License-Identifier: Apache-2.0

// Package extras provides the functionality to interact with the Extras API of Netbox.
package extras

import (
	"fmt"

	"github.com/sapcc/go-netbox-go/extras"
	"github.com/sapcc/go-netbox-go/models"
)

type Extras interface {
	GetTagByName(tagName string) (*models.Tag, error)
}

type ExtrasService struct {
	netboxAPI extras.NetboxAPI
}

func NewExtras(netboxAPI extras.NetboxAPI) Extras {
	return &ExtrasService{netboxAPI: netboxAPI}
}

func (e *ExtrasService) GetTagByName(tagName string) (*models.Tag, error) {
	listTagsRequest := NewListTagsRequest(
		WithName(tagName),
	).BuildRequest()

	res, err := e.netboxAPI.ListTags(listTagsRequest)
	if err != nil {
		return nil, fmt.Errorf("unable to list tags by name %s: %w", tagName, err)
	}
	if res.Count != 1 {
		return nil, fmt.Errorf("unexpected number of tags found (%d)", res.Count)
	}
	return &res.Results[0], nil
}
