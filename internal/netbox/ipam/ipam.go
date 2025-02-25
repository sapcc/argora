package ipam

import (
	"fmt"

	"github.com/sapcc/go-netbox-go/models"
)

type IPAMClient interface {
	ListVlans(opts models.ListVlanRequest) (*models.ListVlanResponse, error)
	ListIPAddresses(opts models.ListIPAddressesRequest) (*models.ListIPAddressesResponse, error)
	ListPrefixes(opts models.ListPrefixesRequest) (*models.ListPrefixesReponse, error)

	DeleteIPAddress(id int) error
}

func (w *IPAMCLientWrapper) ListVlans(opts models.ListVlanRequest) (*models.ListVlanResponse, error) {
	return w.client.ListVlans(opts)
}

func (w *IPAMCLientWrapper) ListIPAddresses(opts models.ListIPAddressesRequest) (*models.ListIPAddressesResponse, error) {
	return w.client.ListIPAddresses(opts)
}

func (w *IPAMCLientWrapper) ListPrefixes(opts models.ListPrefixesRequest) (*models.ListPrefixesReponse, error) {
	return w.client.ListPrefixes(opts)
}

func (w *IPAMCLientWrapper) DeleteIPAddress(id int) error {
	return w.client.DeleteIPAddress(id)
}

type IPAMCLientWrapper struct {
	client IPAMClient
}

func NewIPAMCLientWrapper(client IPAMClient) *IPAMCLientWrapper {
	return &IPAMCLientWrapper{client: client}
}

type IPAM struct {
	client IPAMClient
}

func NewIPAM(client IPAMClient) *IPAM {
	return &IPAM{client: client}
}

func (i *IPAM) GetVlanByName(vlanName string) (*models.Vlan, error) {
	ListVlanRequest := NewListVlanRequest(
		VlanWithName(vlanName),
	).BuildRequest()

	res, err := i.client.ListVlans(ListVlanRequest)
	if err != nil {
		return nil, fmt.Errorf("unable to list VLANs by name %s: %w", vlanName, err)
	}
	if res.Count != 1 {
		return nil, fmt.Errorf("unexpected number of VLANs found (%d)", res.Count)
	}
	return &res.Results[0], nil
}

func (i *IPAM) GetIPAddressByAddress(address string) (*models.IPAddress, error) {
	ListIPAddressesRequest := NewListIPAddressesRequest(
		IPAddressesWithAddress(address),
	).BuildRequest()

	res, err := i.client.ListIPAddresses(ListIPAddressesRequest)
	if err != nil {
		return nil, fmt.Errorf("unable to list IP addresses with address %s: %w", address, err)
	}
	if len(res.Results) != 1 {
		return nil, fmt.Errorf("unexpected number of IP addresses found (%d)", len(res.Results))
	}
	return &res.Results[0], nil
}

func (i *IPAM) GetIPAddressesForInterface(interfaceID int) ([]models.IPAddress, error) {
	ListIPAddressesRequest := NewListIPAddressesRequest(
		IPAddressesWithInterfaceID(interfaceID),
	).BuildRequest()

	res, err := i.client.ListIPAddresses(ListIPAddressesRequest)
	if err != nil {
		return nil, fmt.Errorf("unable to list IP addresses for interface ID %d: %w", interfaceID, err)
	}
	return res.Results, nil
}

func (i *IPAM) GetIPAddressForInterface(interfaceID int) (*models.IPAddress, error) {
	ifaces, err := i.GetIPAddressesForInterface(interfaceID)
	if err != nil {
		return nil, err
	}
	if len(ifaces) != 1 {
		return nil, fmt.Errorf("unexpected number of IP addresses found (%d)", len(ifaces))
	}
	return &ifaces[0], nil
}

func (i *IPAM) GetPrefixesContaining(contains string) ([]models.Prefix, error) {
	ListPrefixesRequest := NewListPrefixesRequest(
		PrefixWithContains(contains),
	).BuildRequest()

	res, err := i.client.ListPrefixes(ListPrefixesRequest)
	if err != nil {
		return nil, fmt.Errorf("unable to list prefixes containing %s: %w", contains, err)
	}
	if len(res.Results) == 0 {
		return nil, fmt.Errorf("prefixes containing %s not found", contains)
	}
	return res.Results, nil
}

func (i *IPAM) DeleteIPAddress(id int) error {
	err := i.client.DeleteIPAddress(id)
	if err != nil {
		return fmt.Errorf("unable to delete IP address (%d): %w", id, err)
	}
	return nil
}
