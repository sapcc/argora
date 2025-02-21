package ipam_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sapcc/argora/internal/netbox/ipam"
	"github.com/sapcc/go-netbox-go/common"
	"github.com/sapcc/go-netbox-go/models"
)

type MockIPAMClient struct {
	ListVlansFunc       func(opts models.ListVlanRequest) (*models.ListVlanResponse, error)
	ListIPAddressesFunc func(opts models.ListIPAddressesRequest) (*models.ListIPAddressesResponse, error)
	ListPrefixesFunc    func(opts models.ListPrefixesRequest) (*models.ListPrefixesReponse, error)
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

var _ = Describe("IPAM", func() {
	var (
		mockClient *MockIPAMClient
		ipamClient *ipam.IPAM
	)

	BeforeEach(func() {
		mockClient = &MockIPAMClient{}
		ipamClient = ipam.NewIPAM(mockClient)
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

			vlan, err := ipamClient.GetVlanByName("test-vlan")
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

			_, err := ipamClient.GetVlanByName("non-existent-vlan")
			Expect(err).To(HaveOccurred())
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

			ip, err := ipamClient.GetIPAddressByAddress("192.168.1.1")
			Expect(err).ToNot(HaveOccurred())
			Expect(ip.Address).To(Equal("192.168.1.1"))
		})

		It("should return an error when IP address is not found", func() {
			mockClient.ListIPAddressesFunc = func(opts models.ListIPAddressesRequest) (*models.ListIPAddressesResponse, error) {
				return &models.ListIPAddressesResponse{Results: []models.IPAddress{}}, nil
			}

			_, err := ipamClient.GetIPAddressByAddress("192.168.1.2")
			Expect(err).To(HaveOccurred())
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

			ips, err := ipamClient.GetIPAddressesForInterface(1)
			Expect(err).ToNot(HaveOccurred())
			Expect(ips).To(HaveLen(2))
		})

		It("should return an error when unable to list IP addresses", func() {
			mockClient.ListIPAddressesFunc = func(opts models.ListIPAddressesRequest) (*models.ListIPAddressesResponse, error) {
				return nil, errors.New("error listing IP addresses")
			}

			_, err := ipamClient.GetIPAddressesForInterface(1)
			Expect(err).To(HaveOccurred())
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

			ip, err := ipamClient.GetIPAddressForInterface(1)
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

			_, err := ipamClient.GetIPAddressForInterface(1)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("GetPrefixesContaining", func() {
		It("should return the prefixes containing the specified address", func() {
			mockClient.ListPrefixesFunc = func(opts models.ListPrefixesRequest) (*models.ListPrefixesReponse, error) {
				return &models.ListPrefixesReponse{
					Results: []models.Prefix{{Prefix: "192.168.1.0/24"}},
				}, nil
			}

			prefixes, err := ipamClient.GetPrefixesContaining("192.168.1.1")
			Expect(err).ToNot(HaveOccurred())
			Expect(prefixes).To(HaveLen(1))
		})

		It("should return an error when no prefixes are found", func() {
			mockClient.ListPrefixesFunc = func(opts models.ListPrefixesRequest) (*models.ListPrefixesReponse, error) {
				return &models.ListPrefixesReponse{Results: []models.Prefix{}}, nil
			}

			_, err := ipamClient.GetPrefixesContaining("192.168.1.1")
			Expect(err).To(HaveOccurred())
		})
	})
})
