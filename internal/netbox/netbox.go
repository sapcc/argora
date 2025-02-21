// Copyright 2024 SAP SE
// SPDX-License-Identifier: Apache-2.0

package netbox

import (
	"github.com/sapcc/go-netbox-go/dcim"
	"github.com/sapcc/go-netbox-go/extras"
	"github.com/sapcc/go-netbox-go/ipam"
	"github.com/sapcc/go-netbox-go/virtualization"
)

type Netbox struct {
	Virtualization *virtualization.Client
	DCIM           *dcim.Client
	IPAM           *ipam.Client
	Extras         *extras.Client
}

func NewNetbox(url, token string) (*Netbox, error) {
	virtualization, err := virtualization.New(url, token, false)
	if err != nil {
		return nil, err
	}
	dcim, err := dcim.New(url, token, false)
	if err != nil {
		return nil, err
	}
	ipam, err := ipam.New(url, token, false)
	if err != nil {
		return nil, err
	}
	extras, err := extras.New(url, token, false)
	if err != nil {
		return nil, err
	}
	return &Netbox{
		Virtualization: virtualization,
		DCIM:           dcim,
		IPAM:           ipam,
		Extras:         extras,
	}, nil
}
