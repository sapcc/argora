// Copyright 2025 SAP SE
// SPDX-License-Identifier: Apache-2.0

package ipam_test

import (
	"errors"
	"testing"

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
	ListVlansFunc       func(opts models.ListVlanRequest) (*models.ListVlanResponse, error)
	ListIPAddressesFunc func(opts models.ListIPAddressesRequest) (*models.ListIPAddressesResponse, error)
	ListPrefixesFunc    func(opts models.ListPrefixesRequest) (*models.ListPrefixesReponse, error)

	DeleteIPAddressFunc func(id int) error
}

func (m *MockIPAMClient) ListVlans(opts models.ListVlanRequest) (*models.ListVlanResponse, error) {
	return m.ListVlansFunc(opts)
}

func (m *MockIPAMClient) ListIPAddresses(opts models.ListIPAddressesRequest) (*models.ListIPAddressesResponse, error) {
	return m.ListIPAddressesFunc(opts)
}

func (m *MockIPAMClient) ListPrefixes(opts models.ListPrefixesRequest) (*models.ListPrefixesReponse, error) {
	return m.ListPrefixesFunc(opts)
}

func (m *MockIPAMClient) DeleteIPAddress(id int) error {
	return m.DeleteIPAddressFunc(id)
}

var _ = Describe("IPAM", func() {
	var (
		mockClient  *MockIPAMClient
		ipamService ipam.IPAM
	)

	BeforeEach(func() {
		mockClient = &MockIPAMClient{}
		ipamService = ipam.NewIPAM(mockClient)
	})

	Describe("GetVlanByName", func() {
		It("should return the VLAN when found", func() {
			mockClient.ListVlansFunc = func(opts models.ListVlanRequest) (*models.ListVlanResponse, error) {
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
				return &models.ListVlanResponse{
					ReturnValues: common.ReturnValues{
						Count: 0,
					},
				}, nil
			}

			_, err := ipamService.GetVlanByName("non-existent-vlan")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unexpected number of VLANs found (0)"))
		})
	})

	Describe("GetIPAddressByAddress", func() {
		It("should return the IP address when found", func() {
			mockClient.ListIPAddressesFunc = func(opts models.ListIPAddressesRequest) (*models.ListIPAddressesResponse, error) {
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
				return &models.ListIPAddressesResponse{Results: []models.IPAddress{}}, nil
			}

			_, err := ipamService.GetIPAddressByAddress("192.168.1.2")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unexpected number of IP addresses found (0)"))
		})
	})

	Describe("GetIPAddressesForInterface", func() {
		It("should return the IP addresses for the interface", func() {
			mockClient.ListIPAddressesFunc = func(opts models.ListIPAddressesRequest) (*models.ListIPAddressesResponse, error) {
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
			Expect(err).To(MatchError("unexpected number of IP addresses found (2)"))
		})
	})

	Describe("GetPrefixesContaining", func() {
		It("should return the prefixes containing the specified address", func() {
			mockClient.ListPrefixesFunc = func(opts models.ListPrefixesRequest) (*models.ListPrefixesReponse, error) {
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
				return nil
			}

			err := ipamService.DeleteIPAddress(1)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return an error when unable to delete the IP address", func() {
			mockClient.DeleteIPAddressFunc = func(id int) error {
				return errors.New("error deleting IP address")
			}

			err := ipamService.DeleteIPAddress(1)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unable to delete IP address (1): error deleting IP address"))
		})
	})
})
