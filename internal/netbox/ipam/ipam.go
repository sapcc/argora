// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

// Package ipam provides the functionality to interact with the IPAM API of Netbox.
package ipam

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/sapcc/go-netbox-go/ipam"
	"github.com/sapcc/go-netbox-go/models"
)

type IPAM interface {
	GetVlanByName(vlanName string) (*models.Vlan, error)
	GetIPAddressByAddress(address string) (*models.IPAddress, error)
	GetIPAddressesForInterface(interfaceID int) ([]models.IPAddress, error)
	GetIPAddressForInterface(interfaceID int) (*models.IPAddress, error)
	GetPrefixesContaining(contains string) ([]models.Prefix, error)

	DeleteIPAddress(id int) error
}

type IPAMService struct {
	netboxAPI ipam.NetboxAPI
	logger    logr.Logger
}

func NewIPAM(netboxAPI ipam.NetboxAPI, logger logr.Logger) IPAM {
	return &IPAMService{netboxAPI, logger}
}

func (i *IPAMService) GetVlanByName(vlanName string) (*models.Vlan, error) {
	ListVlanRequest := NewListVlanRequest(
		VlanWithName(vlanName),
	).BuildRequest()
	i.logger.V(1).Info("list VLANs", "request", ListVlanRequest)
	res, err := i.netboxAPI.ListVlans(ListVlanRequest)
	if err != nil {
		return nil, fmt.Errorf("unable to list VLANs by name %s: %w", vlanName, err)
	}
	if res.Count != 1 {
		return nil, fmt.Errorf("unexpected number of VLANs found by name %s: %d", vlanName, res.Count)
	}
	return &res.Results[0], nil
}

func (i *IPAMService) GetIPAddressByAddress(address string) (*models.IPAddress, error) {
	ListIPAddressesRequest := NewListIPAddressesRequest(
		IPAddressesWithAddress(address),
	).BuildRequest()
	i.logger.V(1).Info("list IP addresses", "request", ListIPAddressesRequest)
	res, err := i.netboxAPI.ListIPAddresses(ListIPAddressesRequest)
	if err != nil {
		return nil, fmt.Errorf("unable to list IP addresses with address %s: %w", address, err)
	}
	if len(res.Results) != 1 {
		return nil, fmt.Errorf("unexpected number of IP addresses found with address %s: %d", address, len(res.Results))
	}
	return &res.Results[0], nil
}

func (i *IPAMService) GetIPAddressesForInterface(interfaceID int) ([]models.IPAddress, error) {
	ListIPAddressesRequest := NewListIPAddressesRequest(
		IPAddressesWithInterfaceID(interfaceID),
	).BuildRequest()
	i.logger.V(1).Info("list IP addresses", "request", ListIPAddressesRequest)
	res, err := i.netboxAPI.ListIPAddresses(ListIPAddressesRequest)
	if err != nil {
		return nil, fmt.Errorf("unable to list IP addresses for interface ID %d: %w", interfaceID, err)
	}
	return res.Results, nil
}

func (i *IPAMService) GetIPAddressForInterface(interfaceID int) (*models.IPAddress, error) {
	i.logger.V(1).Info("get IP addresses for interface", "ID", interfaceID)
	ifaces, err := i.GetIPAddressesForInterface(interfaceID)
	if err != nil {
		return nil, err
	}
	if len(ifaces) != 1 {
		return nil, fmt.Errorf("unexpected number of IP addresses found for interface ID %d: %d", interfaceID, len(ifaces))
	}
	return &ifaces[0], nil
}

func (i *IPAMService) GetPrefixesContaining(contains string) ([]models.Prefix, error) {
	ListPrefixesRequest := NewListPrefixesRequest(
		PrefixWithContains(contains),
	).BuildRequest()
	i.logger.V(1).Info("list prefixes", "request", ListPrefixesRequest)
	res, err := i.netboxAPI.ListPrefixes(ListPrefixesRequest)
	if err != nil {
		return nil, fmt.Errorf("unable to list prefixes containing %s: %w", contains, err)
	}
	if len(res.Results) == 0 {
		return nil, fmt.Errorf("prefixes containing %s not found", contains)
	}
	return res.Results, nil
}

func (i *IPAMService) DeleteIPAddress(id int) error {
	i.logger.V(1).Info("delete IP address", "ID", id)
	err := i.netboxAPI.DeleteIPAddress(id)
	if err != nil {
		return fmt.Errorf("unable to delete IP address (%d): %w", id, err)
	}
	return nil
}
