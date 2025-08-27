// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package ipam_test

import (
	"errors"
	"net/http"
	"net/url"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sapcc/go-netbox-go/common"
	"github.com/sapcc/go-netbox-go/models"

	"github.com/sapcc/argora/internal/netbox/ipam"
)

func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "IPAM Suite")
}

type MockIPAMClient struct {
	// http connectable
	HTTPClientFunc    func() *http.Client
	SetHTTPClientFunc func(httpClient *http.Client)
	BaseURLFunc       func() *url.URL
	SetBaseURLFunc    func(url *url.URL)
	AuthTokenFunc     func() string
	SetAuthTokenFunc  func(authToken string)

	// ip-addresses
	ListIPAddressesFunc func(opts models.ListIPAddressesRequest) (*models.ListIPAddressesResponse, error)
	GetIPAdressFunc     func(id int) (*models.IPAddress, error)
	CreateIPAddressFunc func(ip models.WriteableIPAddress) (*models.IPAddress, error)
	UpdateIPAddressFunc func(address models.WriteableIPAddress) (*models.IPAddress, error)
	DeleteIPAddressFunc func(id int) error

	// prefixes
	ListPrefixesFunc          func(opts models.ListPrefixesRequest) (*models.ListPrefixesReponse, error)
	CreatePrefixFunc          func(prefix models.WriteablePrefix) (*models.Prefix, error)
	ListAvailableIpsFunc      func(id int) ([]models.AvailableIP, error)
	CreateAvailablePrefixFunc func(id int, opts models.CreateAvailablePrefixRequest) (*models.Prefix, error)
	UpdatePrefixFunc          func(prefix models.WriteablePrefix) (*models.Prefix, error)
	DeletePrefixFunc          func(id int) error

	// roles
	ListRolesFunc func(opts models.ListRolesRequest) (*models.ListRolesResponse, error)

	// vlan
	ListVlansFunc func(opts models.ListVlanRequest) (*models.ListVlanResponse, error)
	GetVlanFunc   func(id int) (*models.Vlan, error)

	// vrfs
	ListVRFsFunc func(opts models.ListVRFsRequest) (*models.ListVRFsResponse, error)
}

func (m *MockIPAMClient) HTTPClient() *http.Client {
	return m.HTTPClientFunc()
}

func (m *MockIPAMClient) SetHTTPClient(httpClient *http.Client) {
	m.SetHTTPClientFunc(httpClient)
}

func (m *MockIPAMClient) BaseURL() *url.URL {
	return m.BaseURLFunc()
}

func (m *MockIPAMClient) SetBaseURL(url *url.URL) {
	m.SetBaseURLFunc(url)
}

func (m *MockIPAMClient) AuthToken() string {
	return m.AuthTokenFunc()
}

func (m *MockIPAMClient) SetAuthToken(authToken string) {
	m.SetAuthTokenFunc(authToken)
}

func (m *MockIPAMClient) ListIPAddresses(opts models.ListIPAddressesRequest) (*models.ListIPAddressesResponse, error) {
	return m.ListIPAddressesFunc(opts)
}

func (m *MockIPAMClient) GetIPAdress(id int) (*models.IPAddress, error) {
	return m.GetIPAdressFunc(id)
}

func (m *MockIPAMClient) CreateIPAddress(ip models.WriteableIPAddress) (*models.IPAddress, error) {
	return m.CreateIPAddressFunc(ip)
}

func (m *MockIPAMClient) UpdateIPAddress(address models.WriteableIPAddress) (*models.IPAddress, error) {
	return m.UpdateIPAddressFunc(address)
}

func (m *MockIPAMClient) DeleteIPAddress(id int) error {
	return m.DeleteIPAddressFunc(id)
}

func (m *MockIPAMClient) ListPrefixes(opts models.ListPrefixesRequest) (*models.ListPrefixesReponse, error) {
	return m.ListPrefixesFunc(opts)
}

func (m *MockIPAMClient) CreatePrefix(prefix models.WriteablePrefix) (*models.Prefix, error) {
	return m.CreatePrefixFunc(prefix)
}

func (m *MockIPAMClient) ListAvailableIps(id int) ([]models.AvailableIP, error) {
	return m.ListAvailableIpsFunc(id)
}

func (m *MockIPAMClient) CreateAvailablePrefix(id int, opts models.CreateAvailablePrefixRequest) (*models.Prefix, error) {
	return m.CreateAvailablePrefixFunc(id, opts)
}

func (m *MockIPAMClient) UpdatePrefix(prefix models.WriteablePrefix) (*models.Prefix, error) {
	return m.UpdatePrefixFunc(prefix)
}

func (m *MockIPAMClient) DeletePrefix(id int) error {
	return m.DeletePrefixFunc(id)
}

func (m *MockIPAMClient) ListRoles(opts models.ListRolesRequest) (*models.ListRolesResponse, error) {
	return m.ListRolesFunc(opts)
}

func (m *MockIPAMClient) ListVlans(opts models.ListVlanRequest) (*models.ListVlanResponse, error) {
	return m.ListVlansFunc(opts)
}

func (m *MockIPAMClient) GetVlan(id int) (*models.Vlan, error) {
	return m.GetVlanFunc(id)
}

func (m *MockIPAMClient) ListVRFs(opts models.ListVRFsRequest) (*models.ListVRFsResponse, error) {
	return m.ListVRFsFunc(opts)
}

var _ = Describe("IPAM", func() {
	var (
		mockClient  *MockIPAMClient
		ipamService ipam.IPAM
	)

	BeforeEach(func() {
		mockClient = &MockIPAMClient{}
		ipamService = ipam.NewIPAM(mockClient, logr.Discard())
	})

	Describe("GetVlanByName", func() {
		It("should return the VLAN when found", func() {
			mockClient.ListVlansFunc = func(opts models.ListVlanRequest) (*models.ListVlanResponse, error) {
				Expect(opts.Name).To(Equal("test-vlan"))
				return &models.ListVlanResponse{
					ReturnValues: common.ReturnValues{
						Count: 1,
					},
					Results: []models.Vlan{
						{
							NestedVLAN: models.NestedVLAN{Name: "test-vlan"},
						},
					},
				}, nil
			}

			vlan, err := ipamService.GetVlanByName("test-vlan")
			Expect(err).ToNot(HaveOccurred())
			Expect(vlan.Name).To(Equal("test-vlan"))
		})

		It("should return an error when VLAN is not found", func() {
			mockClient.ListVlansFunc = func(opts models.ListVlanRequest) (*models.ListVlanResponse, error) {
				Expect(opts.Name).To(Equal("non-existent-vlan"))
				return &models.ListVlanResponse{
					ReturnValues: common.ReturnValues{
						Count: 0,
					},
				}, nil
			}

			_, err := ipamService.GetVlanByName("non-existent-vlan")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unexpected number of VLANs found by name non-existent-vlan: 0"))
		})
	})

	Describe("GetIPAddressByAddress", func() {
		It("should return the IP address when found", func() {
			mockClient.ListIPAddressesFunc = func(opts models.ListIPAddressesRequest) (*models.ListIPAddressesResponse, error) {
				Expect(opts.Address).To(Equal("192.168.1.1"))
				return &models.ListIPAddressesResponse{
					Results: []models.IPAddress{
						{
							NestedIPAddress: models.NestedIPAddress{
								Address: "192.168.1.1",
							},
						},
					},
				}, nil
			}

			ip, err := ipamService.GetIPAddressByAddress("192.168.1.1")
			Expect(err).ToNot(HaveOccurred())
			Expect(ip.Address).To(Equal("192.168.1.1"))
		})

		It("should return an error when IP address is not found", func() {
			mockClient.ListIPAddressesFunc = func(opts models.ListIPAddressesRequest) (*models.ListIPAddressesResponse, error) {
				Expect(opts.Address).To(Equal("192.168.1.2"))
				return &models.ListIPAddressesResponse{Results: []models.IPAddress{}}, nil
			}

			_, err := ipamService.GetIPAddressByAddress("192.168.1.2")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unexpected number of IP addresses found with address 192.168.1.2: 0"))
		})
	})

	Describe("GetIPAddressesForInterface", func() {
		It("should return the IP addresses for the interface", func() {
			mockClient.ListIPAddressesFunc = func(opts models.ListIPAddressesRequest) (*models.ListIPAddressesResponse, error) {
				Expect(opts.InterfaceID).To(Equal(1))
				return &models.ListIPAddressesResponse{
					Results: []models.IPAddress{
						{
							NestedIPAddress: models.NestedIPAddress{
								Address: "192.168.1.1",
							},
						},
						{
							NestedIPAddress: models.NestedIPAddress{
								Address: "192.168.1.2",
							},
						},
					},
				}, nil
			}

			ips, err := ipamService.GetIPAddressesForInterface(1)
			Expect(err).ToNot(HaveOccurred())
			Expect(ips).To(HaveLen(2))
		})

		It("should return an error when unable to list IP addresses", func() {
			mockClient.ListIPAddressesFunc = func(opts models.ListIPAddressesRequest) (*models.ListIPAddressesResponse, error) {
				Expect(opts.InterfaceID).To(Equal(1))
				return nil, errors.New("error listing IP addresses")
			}

			_, err := ipamService.GetIPAddressesForInterface(1)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unable to list IP addresses for interface ID 1: error listing IP addresses"))
		})
	})

	Describe("GetIPAddressForInterface", func() {
		It("should return the IP address for the interface", func() {
			mockClient.ListIPAddressesFunc = func(opts models.ListIPAddressesRequest) (*models.ListIPAddressesResponse, error) {
				Expect(opts.InterfaceID).To(Equal(1))
				return &models.ListIPAddressesResponse{
					Results: []models.IPAddress{
						{
							NestedIPAddress: models.NestedIPAddress{
								Address: "192.168.1.1",
							},
						},
					},
				}, nil
			}

			ip, err := ipamService.GetIPAddressForInterface(1)
			Expect(err).ToNot(HaveOccurred())
			Expect(ip.Address).To(Equal("192.168.1.1"))
		})

		It("should return an error when multiple IP addresses are found", func() {
			mockClient.ListIPAddressesFunc = func(opts models.ListIPAddressesRequest) (*models.ListIPAddressesResponse, error) {
				Expect(opts.InterfaceID).To(Equal(1))
				return &models.ListIPAddressesResponse{
					Results: []models.IPAddress{
						{
							NestedIPAddress: models.NestedIPAddress{
								Address: "192.168.1.1",
							},
						},
						{
							NestedIPAddress: models.NestedIPAddress{
								Address: "192.168.1.2",
							},
						},
					},
				}, nil
			}

			_, err := ipamService.GetIPAddressForInterface(1)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unexpected number of IP addresses found for interface ID 1: 2"))
		})
	})

	Describe("GetPrefixesContaining", func() {
		It("should return the prefixes containing the specified address", func() {
			mockClient.ListPrefixesFunc = func(opts models.ListPrefixesRequest) (*models.ListPrefixesReponse, error) {
				Expect(opts.Contains).To(Equal("192.168.1.1"))
				return &models.ListPrefixesReponse{
					Results: []models.Prefix{{Prefix: "192.168.1.0/24"}},
				}, nil
			}

			prefixes, err := ipamService.GetPrefixesContaining("192.168.1.1")
			Expect(err).ToNot(HaveOccurred())
			Expect(prefixes).To(HaveLen(1))
		})

		It("should return an error when no prefixes are found", func() {
			mockClient.ListPrefixesFunc = func(opts models.ListPrefixesRequest) (*models.ListPrefixesReponse, error) {
				Expect(opts.Contains).To(Equal("192.168.1.1"))
				return &models.ListPrefixesReponse{Results: []models.Prefix{}}, nil
			}

			_, err := ipamService.GetPrefixesContaining("192.168.1.1")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("prefixes containing 192.168.1.1 not found"))
		})
	})

	Describe("DeleteIPAddress", func() {
		It("should delete the IP address successfully", func() {
			mockClient.DeleteIPAddressFunc = func(id int) error {
				Expect(id).To(Equal(1))
				return nil
			}

			err := ipamService.DeleteIPAddress(1)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return an error when unable to delete the IP address", func() {
			mockClient.DeleteIPAddressFunc = func(id int) error {
				Expect(id).To(Equal(1))
				return errors.New("error deleting IP address")
			}

			err := ipamService.DeleteIPAddress(1)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unable to delete IP address (1): error deleting IP address"))
		})
	})
})
