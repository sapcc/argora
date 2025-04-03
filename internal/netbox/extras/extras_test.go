// Copyright 2025 SAP SE
// SPDX-License-Identifier: Apache-2.0

package extras_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sapcc/go-netbox-go/common"
	"github.com/sapcc/go-netbox-go/models"

	"github.com/sapcc/argora/internal/netbox/extras"
)

func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Extras Suite")
}

type MockExtrasClient struct {
	// http connectable
	HTTPClientFunc    func() *http.Client
	SetHTTPClientFunc func(httpClient *http.Client)
	BaseURLFunc       func() *url.URL
	SetBaseURLFunc    func(url *url.URL)
	AuthTokenFunc     func() string
	SetAuthTokenFunc  func(authToken string)

	// tags
	ListTagsFunc func(opts models.ListTagsRequest) (*models.ListTagsResponse, error)
}

func (m *MockExtrasClient) HTTPClient() *http.Client {
	return m.HTTPClientFunc()
}

func (m *MockExtrasClient) SetHTTPClient(httpClient *http.Client) {
	m.SetHTTPClientFunc(httpClient)
}

func (m *MockExtrasClient) BaseURL() *url.URL {
	return m.BaseURLFunc()
}

func (m *MockExtrasClient) SetBaseURL(url *url.URL) {
	m.SetBaseURLFunc(url)
}

func (m *MockExtrasClient) AuthToken() string {
	return m.AuthTokenFunc()
}

func (m *MockExtrasClient) SetAuthToken(authToken string) {
	m.SetAuthTokenFunc(authToken)
}

func (m *MockExtrasClient) ListTags(opts models.ListTagsRequest) (*models.ListTagsResponse, error) {
	return m.ListTagsFunc(opts)
}

var _ = Describe("Extras", func() {
	var (
		mockClient    *MockExtrasClient
		extrasService extras.Extras
	)

	BeforeEach(func() {
		mockClient = &MockExtrasClient{}
		extrasService = extras.NewExtras(mockClient, logr.Discard())
	})

	Describe("GetTagByName", func() {
		Context("when the tag exists", func() {
			It("should return the tag", func() {
				expectedTag := models.Tag{
					NestedTag: models.NestedTag{
						Name: "test-tag",
					},
				}
				mockClient.ListTagsFunc = func(opts models.ListTagsRequest) (*models.ListTagsResponse, error) {
					Expect(opts.Name).To(Equal("test-tag"))
					return &models.ListTagsResponse{
						ReturnValues: common.ReturnValues{
							Count: 1,
						},
						Results: []models.Tag{expectedTag},
					}, nil
				}

				tag, err := extrasService.GetTagByName("test-tag")
				Expect(err).ToNot(HaveOccurred())
				Expect(tag).To(Equal(&expectedTag))
			})
		})

		Context("when the tag does not exist", func() {
			It("should return an error", func() {
				mockClient.ListTagsFunc = func(opts models.ListTagsRequest) (*models.ListTagsResponse, error) {
					Expect(opts.Name).To(Equal("nonexistent-tag"))
					return &models.ListTagsResponse{
						ReturnValues: common.ReturnValues{
							Count: 0,
						},
						Results: []models.Tag{},
					}, nil
				}

				tag, err := extrasService.GetTagByName("nonexistent-tag")
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(fmt.Sprintf("unexpected number of tags found by name nonexistent-tag: %d", 0)))
				Expect(tag).To(BeNil())
			})
		})

		Context("when there is an error listing tags", func() {
			It("should return an error", func() {
				mockClient.ListTagsFunc = func(opts models.ListTagsRequest) (*models.ListTagsResponse, error) {
					Expect(opts.Name).To(Equal("test-tag"))
					return nil, errors.New("list tags error")
				}

				tag, err := extrasService.GetTagByName("test-tag")
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("unable to list tags by name test-tag: list tags error"))
				Expect(tag).To(BeNil())
			})
		})

		Context("when multiple tags are found", func() {
			It("should return an error", func() {
				mockClient.ListTagsFunc = func(opts models.ListTagsRequest) (*models.ListTagsResponse, error) {
					Expect(opts.Name).To(Equal("test-tag"))
					return &models.ListTagsResponse{
						ReturnValues: common.ReturnValues{
							Count: 2,
						},
						Results: []models.Tag{
							{
								NestedTag: models.NestedTag{
									Name: "test-tag1",
								},
							},
							{
								NestedTag: models.NestedTag{
									Name: "test-tag2",
								},
							},
						},
					}, nil
				}

				tag, err := extrasService.GetTagByName("test-tag")
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(fmt.Sprintf("unexpected number of tags found by name test-tag: %d", 2)))
				Expect(tag).To(BeNil())
			})
		})
	})
})
