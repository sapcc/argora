// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
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
	virtClient, err := virtualization.NewClient(url, token, false)
	if err != nil {
		return err
	}
	dcimClient, err := dcim.NewClient(url, token, false)
	if err != nil {
		return err
	}
	ipamClient, err := ipam.NewClient(url, token, false)
	if err != nil {
		return err
	}
	extrasClient, err := extras.NewClient(url, token, false)
	if err != nil {
		return err
	}
	n.virtualization = _virtualization.NewVirtualization(virtClient, logger.WithValues("nbComponent", "virtualization"))
	n.dcim = _dcim.NewDCIM(dcimClient, logger.WithValues("nbComponent", "dcim"))
	n.ipam = _ipam.NewIPAM(ipamClient, logger.WithValues("nbComponent", "ipam"))
	n.extras = _extras.NewExtras(extrasClient, logger.WithValues("nbComponent", "extras"))
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
