package extras_test

import (
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sapcc/argora/internal/netbox/extras"
	"github.com/sapcc/go-netbox-go/common"
	"github.com/sapcc/go-netbox-go/models"
)

type MockExtrasClient struct {
	ListTagsFunc func(opts models.ListTagsRequest) (*models.ListTagsResponse, error)
}

func (m *MockExtrasClient) ListTags(opts models.ListTagsRequest) (*models.ListTagsResponse, error) {
	return m.ListTagsFunc(opts)
}

var _ = Describe("Extras", func() {
	var (
		mockClient   *MockExtrasClient
		extrasClient *extras.Extras
	)

	BeforeEach(func() {
		mockClient = &MockExtrasClient{}
		extrasClient = extras.NewExtras(mockClient)
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
					return &models.ListTagsResponse{
						ReturnValues: common.ReturnValues{
							Count: 1,
						},
						Results: []models.Tag{expectedTag},
					}, nil
				}

				tag, err := extrasClient.GetTagByName("test-tag")
				Expect(err).NotTo(HaveOccurred())
				Expect(tag).To(Equal(&expectedTag))
			})
		})

		Context("when the tag does not exist", func() {
			It("should return an error", func() {
				mockClient.ListTagsFunc = func(opts models.ListTagsRequest) (*models.ListTagsResponse, error) {
					return &models.ListTagsResponse{
						ReturnValues: common.ReturnValues{
							Count: 0,
						},
						Results: []models.Tag{},
					}, nil
				}

				tag, err := extrasClient.GetTagByName("nonexistent-tag")
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(fmt.Sprintf("unexpected number of tags found (%d)", 0)))
				Expect(tag).To(BeNil())
			})
		})

		Context("when there is an error listing tags", func() {
			It("should return an error", func() {
				mockClient.ListTagsFunc = func(opts models.ListTagsRequest) (*models.ListTagsResponse, error) {
					return nil, errors.New("list tags error")
				}

				tag, err := extrasClient.GetTagByName("test-tag")
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("unable to list tags by name test-tag: list tags error"))
				Expect(tag).To(BeNil())
			})
		})

		Context("when multiple tags are found", func() {
			It("should return an error", func() {
				mockClient.ListTagsFunc = func(opts models.ListTagsRequest) (*models.ListTagsResponse, error) {
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

				tag, err := extrasClient.GetTagByName("test-tag")
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(fmt.Sprintf("unexpected number of tags found (%d)", 2)))
				Expect(tag).To(BeNil())
			})
		})
	})
})
