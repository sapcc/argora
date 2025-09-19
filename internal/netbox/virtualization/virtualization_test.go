// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package virtualization_test

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

	"github.com/sapcc/argora/internal/netbox/virtualization"
)

func TestVirtualization(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Virtualization Suite")
}

type MockVirtualizationClient struct {
	// http connectable
	HTTPClientFunc    func() *http.Client
	SetHTTPClientFunc func(httpClient *http.Client)
	BaseURLFunc       func() *url.URL
	SetBaseURLFunc    func(url *url.URL)
	AuthTokenFunc     func() string
	SetAuthTokenFunc  func(authToken string)

	// cluster
	ListClustersFunc func(opts models.ListClusterRequest) (*models.ListClusterResponse, error)

	// virtual machines
	CreateVirtualMachineFunc func(vm models.WriteableVirtualMachine) (*models.VirtualMachine, error)
	UpdateVirtualMachineFunc func(vm models.WriteableVirtualMachine) (*models.VirtualMachine, error)
	DeleteVirtualMachineFunc func(id int) error
	GetVirtualMachineFunc    func(id int) (*models.VirtualMachine, error)
	ListVirtualMachinesFunc  func(opts models.ListVirtualMachinesRequest) (*models.ListVirtualMachinesResponse, error)

	// vm interfaces
	CreateVMInterfaceFunc func(vmni models.WritableVMInterface) (*models.VMInterface, error)
	UpdateVMInterfaceFunc func(vmi models.WritableVMInterface) (*models.VMInterface, error)
	DeleteVMInterfaceFunc func(id int) error
	ListVMInterfacesFunc  func(opts models.ListVMInterfacesRequest) (*models.ListVMInterfacesResponse, error)
	GetVMInterfaceFunc    func(id int) (*models.VMInterface, error)
}

func (m *MockVirtualizationClient) HTTPClient() *http.Client {
	return m.HTTPClientFunc()
}

func (m *MockVirtualizationClient) SetHTTPClient(httpClient *http.Client) {
	m.SetHTTPClientFunc(httpClient)
}

func (m *MockVirtualizationClient) BaseURL() *url.URL {
	return m.BaseURLFunc()
}

func (m *MockVirtualizationClient) SetBaseURL(url *url.URL) {
	m.SetBaseURLFunc(url)
}

func (m *MockVirtualizationClient) AuthToken() string {
	return m.AuthTokenFunc()
}

func (m *MockVirtualizationClient) SetAuthToken(authToken string) {
	m.SetAuthTokenFunc(authToken)
}

func (m *MockVirtualizationClient) ListClusters(opts models.ListClusterRequest) (*models.ListClusterResponse, error) {
	return m.ListClustersFunc(opts)
}

func (m *MockVirtualizationClient) CreateVirtualMachine(vm models.WriteableVirtualMachine) (*models.VirtualMachine, error) {
	return m.CreateVirtualMachineFunc(vm)
}

func (m *MockVirtualizationClient) UpdateVirtualMachine(vm models.WriteableVirtualMachine) (*models.VirtualMachine, error) {
	return m.UpdateVirtualMachineFunc(vm)
}

func (m *MockVirtualizationClient) DeleteVirtualMachine(id int) error {
	return m.DeleteVirtualMachineFunc(id)
}

func (m *MockVirtualizationClient) GetVirtualMachine(id int) (*models.VirtualMachine, error) {
	return m.GetVirtualMachineFunc(id)
}

func (m *MockVirtualizationClient) ListVirtualMachines(opts models.ListVirtualMachinesRequest) (*models.ListVirtualMachinesResponse, error) {
	return m.ListVirtualMachinesFunc(opts)
}

func (m *MockVirtualizationClient) CreateVMInterface(vmni models.WritableVMInterface) (*models.VMInterface, error) {
	return m.CreateVMInterfaceFunc(vmni)
}

func (m *MockVirtualizationClient) UpdateVMInterface(vmi models.WritableVMInterface) (*models.VMInterface, error) {
	return m.UpdateVMInterfaceFunc(vmi)
}

func (m *MockVirtualizationClient) DeleteVMInterface(id int) error {
	return m.DeleteVMInterfaceFunc(id)
}

func (m *MockVirtualizationClient) ListVMInterfaces(opts models.ListVMInterfacesRequest) (*models.ListVMInterfacesResponse, error) {
	return m.ListVMInterfacesFunc(opts)
}

func (m *MockVirtualizationClient) GetVMInterface(id int) (*models.VMInterface, error) {
	return m.GetVMInterfaceFunc(id)
}

var _ = Describe("Virtualization", func() {
	var (
		mockClient            *MockVirtualizationClient
		virtualizationService virtualization.Virtualization
	)

	BeforeEach(func() {
		mockClient = &MockVirtualizationClient{}
		virtualizationService = virtualization.NewVirtualization(mockClient, logr.Discard())
	})

	Describe("GetClustersByNameRegionType", func() {
		It("should return the cluster when found", func() {
			mockClient.ListClustersFunc = func(opts models.ListClusterRequest) (*models.ListClusterResponse, error) {
				Expect(opts.Name).To(Equal("test-cluster"))
				Expect(opts.Region).To(Equal(""))
				Expect(opts.Type).To(Equal(""))
				return &models.ListClusterResponse{
					ReturnValues: common.ReturnValues{
						Count: 1,
					},
					Results: []models.Cluster{
						{
							Name: "test-cluster",
						},
					},
				}, nil
			}

			clusters, err := virtualizationService.GetClustersByNameRegionType("test-cluster", "", "")
			Expect(err).ToNot(HaveOccurred())
			Expect(clusters).To(HaveLen(1))
			Expect(clusters[0].Name).To(Equal("test-cluster"))
		})

		It("should return an error when no clusters are found", func() {
			mockClient.ListClustersFunc = func(opts models.ListClusterRequest) (*models.ListClusterResponse, error) {
				Expect(opts.Name).To(Equal(""))
				Expect(opts.Region).To(Equal("test-region"))
				Expect(opts.Type).To(Equal(""))
				return &models.ListClusterResponse{
					ReturnValues: common.ReturnValues{
						Count: 0,
					},
					Results: []models.Cluster{},
				}, nil
			}

			cluster, err := virtualizationService.GetClustersByNameRegionType("", "test-region", "")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("no clusters found"))
			Expect(cluster).To(BeNil())
		})

		It("should return an error when the client returns an error", func() {
			mockClient.ListClustersFunc = func(opts models.ListClusterRequest) (*models.ListClusterResponse, error) {
				Expect(opts.Name).To(Equal(""))
				Expect(opts.Region).To(Equal(""))
				Expect(opts.Type).To(Equal("test-type"))
				return nil, errors.New("client error")
			}

			cluster, err := virtualizationService.GetClustersByNameRegionType("", "", "test-type")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("client error"))
			Expect(cluster).To(BeNil())
		})
	})
})
