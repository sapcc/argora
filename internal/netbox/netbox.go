// Copyright 2025 SAP SE
// SPDX-License-Identifier: Apache-2.0

// Package netbox provides user friendly client API to Netbox
package netbox

import (
	"github.com/go-logr/logr"
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
	Reload(url, token string, logger logr.Logger) error

	Virtualization() _virtualization.Virtualization
	DCIM() _dcim.DCIM
	IPAM() _ipam.IPAM
	Extras() _extras.Extras
}

type NetboxService struct {
	virtualization _virtualization.Virtualization
	dcim           _dcim.DCIM
	ipam           _ipam.IPAM
	extras         _extras.Extras
}

func NewNetbox() Netbox {
	return &NetboxService{
		virtualization: nil,
		dcim:           nil,
		ipam:           nil,
		extras:         nil,
	}
}

func (n *NetboxService) Reload(url, token string, logger logr.Logger) error {
	virtualization, err := virtualization.NewClient(url, token, false)
	if err != nil {
		return err
	}
	dcim, err := dcim.NewClient(url, token, false)
	if err != nil {
		return err
	}
	ipam, err := ipam.NewClient(url, token, false)
	if err != nil {
		return err
	}
	extras, err := extras.NewClient(url, token, false)
	if err != nil {
		return err
	}
	n.virtualization = _virtualization.NewVirtualization(virtualization, logger.WithValues("nbComponent", "virtualization"))
	n.dcim = _dcim.NewDCIM(dcim, logger.WithValues("nbComponent", "dcim"))
	n.ipam = _ipam.NewIPAM(ipam, logger.WithValues("nbComponent", "ipam"))
	n.extras = _extras.NewExtras(extras, logger.WithValues("nbComponent", "extras"))
	return nil
}

func (n *NetboxService) Virtualization() _virtualization.Virtualization {
	return n.virtualization
}

func (n *NetboxService) DCIM() _dcim.DCIM {
	return n.dcim
}

func (n *NetboxService) IPAM() _ipam.IPAM {
	return n.ipam
}

func (n *NetboxService) Extras() _extras.Extras {
	return n.extras
}
