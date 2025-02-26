// Copyright 2024 SAP SE
// SPDX-License-Identifier: Apache-2.0

package netbox

import (
	"github.com/sapcc/go-netbox-go/dcim"
	"github.com/sapcc/go-netbox-go/extras"
	"github.com/sapcc/go-netbox-go/ipam"
	"github.com/sapcc/go-netbox-go/virtualization"

	_dcim "github.com/sapcc/argora/internal/netbox/dcim"
	_extras "github.com/sapcc/argora/internal/netbox/extras"
	_ipam "github.com/sapcc/argora/internal/netbox/ipam"
	_virtualization "github.com/sapcc/argora/internal/netbox/virtualization"
)

type Netbox interface {
	Virtualization() _virtualization.Virtualization
	SetVirtualization(virtualization _virtualization.Virtualization)

	DCIM() _dcim.DCIM
	SetDCIM(dcim _dcim.DCIM)

	IPAM() _ipam.IPAM
	SetIPAM(ipam _ipam.IPAM)

	Extras() _extras.Extras
	SetExtras(extras _extras.Extras)
}

type NetboxService struct {
	virtualization _virtualization.Virtualization
	dcim           _dcim.DCIM
	ipam           _ipam.IPAM
	extras         _extras.Extras
}

func NewNetbox(virtualization _virtualization.Virtualization, dcim _dcim.DCIM, ipam _ipam.IPAM, extras _extras.Extras) Netbox {
	return &NetboxService{
		virtualization: virtualization,
		dcim:           dcim,
		ipam:           ipam,
		extras:         extras,
	}
}

func NewDefaultNetbox(url, token string) (Netbox, error) {
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
	return &NetboxService{
		virtualization: _virtualization.NewVirtualization(virtualization),
		dcim:           _dcim.NewDCIM(dcim),
		ipam:           _ipam.NewIPAM(ipam),
		extras:         _extras.NewExtras(extras),
	}, nil
}

func (n *NetboxService) Virtualization() _virtualization.Virtualization {
	return n.virtualization
}

func (n *NetboxService) SetVirtualization(virtualization _virtualization.Virtualization) {
	n.virtualization = virtualization
}

func (n *NetboxService) DCIM() _dcim.DCIM {
	return n.dcim
}

func (n *NetboxService) SetDCIM(dcim _dcim.DCIM) {
	n.dcim = dcim
}

func (n *NetboxService) IPAM() _ipam.IPAM {
	return n.ipam
}

func (n *NetboxService) SetIPAM(ipam _ipam.IPAM) {
	n.ipam = ipam
}

func (n *NetboxService) Extras() _extras.Extras {
	return n.extras
}

func (n *NetboxService) SetExtras(extras _extras.Extras) {
	n.extras = extras
}
