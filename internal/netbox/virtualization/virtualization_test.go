// Copyright 2025 SAP SE
// SPDX-License-Identifier: Apache-2.0

package virtualization_test

import (
	"errors"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sapcc/go-netbox-go/common"
	"github.com/sapcc/go-netbox-go/models"

	"github.com/sapcc/argora/internal/netbox/virtualization"
)

func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Virtualization Suite")
}

type MockVirtualizationClient struct {
	ListClustersFunc func(opts models.ListClusterRequest) (*models.ListClusterResponse, error)
}

func (m *MockVirtualizationClient) ListClusters(opts models.ListClusterRequest) (*models.ListClusterResponse, error) {
	return m.ListClustersFunc(opts)
}

var _ = Describe("Virtualization", func() {
	var (
		mockClient            *MockVirtualizationClient
		virtualizationService virtualization.Virtualization
	)

	BeforeEach(func() {
		mockClient = &MockVirtualizationClient{}
		virtualizationService = virtualization.NewVirtualization(mockClient)
	})

	Describe("GetClusterByName", func() {
		It("should return the cluster when found", func() {
			expectedCluster := models.Cluster{Name: "test-cluster"}
			mockClient.ListClustersFunc = func(opts models.ListClusterRequest) (*models.ListClusterResponse, error) {
				return &models.ListClusterResponse{
					ReturnValues: common.ReturnValues{
						Count: 1,
					},
					Results: []models.Cluster{expectedCluster},
				}, nil
			}

			cluster, err := virtualizationService.GetClusterByName("test-cluster")
			Expect(err).NotTo(HaveOccurred())
			Expect(cluster).To(Equal(&expectedCluster))
		})

		It("should return an error when no clusters are found", func() {
			mockClient.ListClustersFunc = func(opts models.ListClusterRequest) (*models.ListClusterResponse, error) {
				return &models.ListClusterResponse{
					ReturnValues: common.ReturnValues{
						Count: 0,
					},
					Results: []models.Cluster{},
				}, nil
			}

			cluster, err := virtualizationService.GetClusterByName("test-cluster")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unexpected number of clusters found (0)"))
			Expect(cluster).To(BeNil())
		})

		It("should return an error when multiple clusters are found", func() {
			mockClient.ListClustersFunc = func(opts models.ListClusterRequest) (*models.ListClusterResponse, error) {
				return &models.ListClusterResponse{
					ReturnValues: common.ReturnValues{
						Count: 2,
					},
					Results: []models.Cluster{{Name: "test-cluster-1"}, {Name: "test-cluster-2"}},
				}, nil
			}

			cluster, err := virtualizationService.GetClusterByName("test-cluster")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unexpected number of clusters found (2)"))
			Expect(cluster).To(BeNil())
		})

		It("should return an error when the client returns an error", func() {
			mockClient.ListClustersFunc = func(opts models.ListClusterRequest) (*models.ListClusterResponse, error) {
				return nil, errors.New("client error")
			}

			cluster, err := virtualizationService.GetClusterByName("test-cluster")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unable to list clusters by name test-cluster: client error"))
			Expect(cluster).To(BeNil())
		})
	})

	Describe("GetClusterByNameRegionType", func() {
		It("should return the cluster when found", func() {
			expectedCluster := models.Cluster{Name: "test-cluster"}
			mockClient.ListClustersFunc = func(opts models.ListClusterRequest) (*models.ListClusterResponse, error) {
				return &models.ListClusterResponse{
					ReturnValues: common.ReturnValues{
						Count: 1,
					},
					Results: []models.Cluster{expectedCluster},
				}, nil
			}

			cluster, err := virtualizationService.GetClusterByNameRegionType("test-cluster", "test-region", "test-type")
			Expect(err).NotTo(HaveOccurred())
			Expect(cluster).To(Equal(&expectedCluster))
		})

		It("should return an error when no clusters are found", func() {
			mockClient.ListClustersFunc = func(opts models.ListClusterRequest) (*models.ListClusterResponse, error) {
				return &models.ListClusterResponse{
					ReturnValues: common.ReturnValues{
						Count: 0,
					},
					Results: []models.Cluster{},
				}, nil
			}

			cluster, err := virtualizationService.GetClusterByNameRegionType("test-cluster", "test-region", "test-type")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unexpected number of clusters found (0)"))
			Expect(cluster).To(BeNil())
		})

		It("should return an error when multiple clusters are found", func() {
			mockClient.ListClustersFunc = func(opts models.ListClusterRequest) (*models.ListClusterResponse, error) {
				return &models.ListClusterResponse{
					ReturnValues: common.ReturnValues{
						Count: 2,
					},
					Results: []models.Cluster{{Name: "test-cluster-1"}, {Name: "test-cluster-2"}},
				}, nil
			}

			cluster, err := virtualizationService.GetClusterByNameRegionType("test-cluster", "test-region", "test-type")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unexpected number of clusters found (2)"))
			Expect(cluster).To(BeNil())
		})

		It("should return an error when the client returns an error", func() {
			mockClient.ListClustersFunc = func(opts models.ListClusterRequest) (*models.ListClusterResponse, error) {
				return nil, errors.New("client error")
			}

			cluster, err := virtualizationService.GetClusterByNameRegionType("test-cluster", "test-region", "test-type")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("client error"))
			Expect(cluster).To(BeNil())
		})
	})
})
