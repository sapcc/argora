package ipam

import (
	"fmt"

	"github.com/sapcc/argora/internal/netbox"
	"github.com/sapcc/go-netbox-go/models"
)

type ListVlanRequest struct {
	name string
}

type ListVlanRequestOption func(c *ListVlanRequest)

func NewListVlanRequest(opts ...ListVlanRequestOption) *ListVlanRequest {
	r := &ListVlanRequest{}
	for _, opt := range opts {
		opt(r)
	}

	return r
}

func VlanWithName(name string) ListVlanRequestOption {
	opt := func(r *ListVlanRequest) {
		r.name = name
	}

	return opt
}

func (r *ListVlanRequest) BuildRequest() models.ListVlanRequest {
	listVlanRequest := models.ListVlanRequest{}
	if r.name != "" {
		listVlanRequest.Name = r.name
	}
	return listVlanRequest
}

type ListIPAddressesRequest struct {
	interfaceID int
	address     string
}

type ListIPAddressesRequestOption func(c *ListIPAddressesRequest)

func NewListIPAddressesRequest(opts ...ListIPAddressesRequestOption) *ListIPAddressesRequest {
	r := &ListIPAddressesRequest{}
	for _, opt := range opts {
		opt(r)
	}

	return r
}

func IPAddressesWithInterfaceID(interfaceID int) ListIPAddressesRequestOption {
	opt := func(r *ListIPAddressesRequest) {
		r.interfaceID = interfaceID
	}

	return opt
}

func IPAddressesWithAddress(address string) ListIPAddressesRequestOption {
	opt := func(r *ListIPAddressesRequest) {
		r.address = address
	}

	return opt
}

func (r *ListIPAddressesRequest) BuildRequest() models.ListIPAddressesRequest {
	listIPAddressesRequest := models.ListIPAddressesRequest{}
	if r.interfaceID != 0 {
		listIPAddressesRequest.InterfaceID = r.interfaceID
	}
	if r.address != "" {
		listIPAddressesRequest.Address = r.address
	}
	return listIPAddressesRequest
}

type ListPrefixesRequest struct {
	contains string
}

type ListPrefixesRequestOption func(c *ListPrefixesRequest)

func NewListPrefixesRequest(opts ...ListPrefixesRequestOption) *ListPrefixesRequest {
	r := &ListPrefixesRequest{}
	for _, opt := range opts {
		opt(r)
	}

	return r
}

func PrefixWithContains(contains string) ListPrefixesRequestOption {
	opt := func(r *ListPrefixesRequest) {
		r.contains = contains
	}

	return opt
}

func (r *ListPrefixesRequest) BuildRequest() models.ListPrefixesRequest {
	listPrefixesRequest := models.ListPrefixesRequest{}
	if r.contains != "" {
		listPrefixesRequest.Contains = r.contains
	}
	return listPrefixesRequest
}

type IPAM struct {
	client *netbox.NetboxClient
}

func NewIPAM(client *netbox.NetboxClient) *IPAM {
	return &IPAM{client: client}
}

func (i *IPAM) GetVlanByName(vlanName string) (*models.Vlan, error) {
	ListVlanRequest := NewListVlanRequest(
		VlanWithName(vlanName),
	).BuildRequest()

	res, err := i.client.IPAM.ListVlans(ListVlanRequest)
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

	res, err := i.client.IPAM.ListIPAddresses(ListIPAddressesRequest)
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

	res, err := i.client.IPAM.ListIPAddresses(ListIPAddressesRequest)
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

	res, err := i.client.IPAM.ListPrefixes(ListPrefixesRequest)
	if err != nil {
		return nil, fmt.Errorf("unable to list prefixes containing %s: %w", contains, err)
	}
	if len(res.Results) == 0 {
		return nil, fmt.Errorf("prefixes containing %s not found", contains)
	}
	return res.Results, nil
}
