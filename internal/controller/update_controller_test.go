/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	argorav1alpha1 "github.com/sapcc/argora/api/v1alpha1"
	"github.com/sapcc/argora/internal/config"
	"github.com/sapcc/argora/internal/controller/mock"
	"github.com/sapcc/go-netbox-go/models"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Update Controller", func() {
	var fileReaderMock *mock.FileReaderMock

	Context("When reconciling a Update custom resource", func() {
		const resourceName = "test-resource"
		const resourceNamespace = "default"

		fileReaderMock = &mock.FileReaderMock{
			FileContent: make(map[string]string),
			ReturnError: false,
		}
		configJson := `{
			"ironCoreRoles": "role1",
			"ironCoreRegion": "region1",
			"serverController": "ironcore"
		}`
		credentialsJson := `{
			"netboxUrl": "aHR0cDovL25ldGJveA==",
			"netboxToken": "dG9rZW4=",
			"bmcUser": "dXNlcg==",
			"bmcPassword": "cGFzc3dvcmQ="
		}`

		fileReaderMock.FileContent["/etc/config/config.json"] = configJson
		fileReaderMock.FileContent["/etc/credentials/credentials.json"] = credentialsJson

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: resourceNamespace,
		}
		update := &argorav1alpha1.Update{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind Update")
			err := k8sClient.Get(ctx, typeNamespacedName, update)
			if err != nil && apierrors.IsNotFound(err) {
				resource := &argorav1alpha1.Update{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: resourceNamespace,
					},
					Spec: argorav1alpha1.UpdateSpec{
						Clusters: []*argorav1alpha1.Clusters{
							{
								Name:   "",
								Region: "region1",
								Type:   "type1",
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			err := k8sClient.Get(ctx, typeNamespacedName, update)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Update")
			Expect(k8sClient.Delete(ctx, update)).To(Succeed())
		})

		It("should successfully reconcile the CR", func() {
			netBoxMock := &mock.NetBoxMock{
				ReturnError:        false,
				VirtualizationMock: &mock.VirtualizationMock{},
				DCIMMock:           &mock.DCIMMock{},
				IPAMMock:           &mock.IPAMMock{},
				ExtrasMock:         &mock.ExtrasMock{},
			}

			netBoxMock.VirtualizationMock.(*mock.VirtualizationMock).GetClusterByNameRegionTypeFunc = func(name, region, clusterType string) (*models.Cluster, error) {
				Expect(name).To(BeEmpty())
				Expect(region).To(Equal("region1"))
				Expect(clusterType).To(Equal("type1"))

				return &models.Cluster{
					ID:   1,
					Name: "cluster1",
				}, nil
			}
			netBoxMock.DCIMMock.(*mock.DCIMMock).GetDevicesByClusterIDFunc = func(clusterID int) ([]models.Device, error) {
				Expect(clusterID).To(Equal(1))
				return []models.Device{
					{
						ID:     1,
						Name:   "device1",
						Status: models.DeviceStatus{Value: "active"},
						Platform: models.NestedPlatform{
							ID: 1,
						},
						OOBIp: models.NestedIPAddress{
							ID: 1,
						},
					},
				}, nil
			}
			netBoxMock.DCIMMock.(*mock.DCIMMock).GetInterfacesForDeviceFunc = func(device *models.Device) ([]models.Interface, error) {
				Expect(device.Name).To(Equal("device1"))
				return []models.Interface{
					{
						NestedInterface: models.NestedInterface{ID: 1},
						Name:            "remoteboard",
					},
				}, nil
			}
			netBoxMock.DCIMMock.(*mock.DCIMMock).GetInterfaceForDeviceFunc = func(device *models.Device, ifaceName string) (*models.Interface, error) {
				Expect(device.Name).To(Equal("device1"))
				Expect(ifaceName).To(Equal("remoteboard"))
				return &models.Interface{
					NestedInterface: models.NestedInterface{ID: 1},
					Name:            "remoteboard",
				}, nil
			}
			netBoxMock.IPAMMock.(*mock.IPAMMock).GetIPAddressForInterfaceFunc = func(ifaceID int) (*models.IPAddress, error) {
				Expect(ifaceID).To(Equal(1))
				return &models.IPAddress{
					NestedIPAddress: models.NestedIPAddress{
						ID: 1,
					},
				}, nil
			}
			netBoxMock.DCIMMock.(*mock.DCIMMock).GetPlatformByNameFunc = func(platformName string) (*models.Platform, error) {
				Expect(platformName).To(Equal("Linux KVM"))
				return &models.Platform{
					NestedPlatform: models.NestedPlatform{ID: 1},
				}, nil
			}
			// given
			By("Reconciling the created resource")
			controllerReconciler := &UpdateReconciler{
				k8sClient:         k8sClient,
				scheme:            k8sClient.Scheme(),
				netBox:            netBoxMock,
				cfg:               config.NewDefaultConfiguration(k8sClient, fileReaderMock),
				reconcileInterval: reconcileInterval,
			}

			// when
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})

			// then
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Get(ctx, typeNamespacedName, update)
			Expect(err).NotTo(HaveOccurred())
			Expect(update.Status.State).To(Equal(argorav1alpha1.Ready))
			Expect(update.Status.Description).To(BeEmpty())
			Expect(update.Status.Conditions).ToNot(BeNil())
			Expect((*update.Status.Conditions)).To(HaveLen(1))
			Expect((*update.Status.Conditions)[0].Type).To(Equal(string(argorav1alpha1.ConditionTypeReady)))
			Expect((*update.Status.Conditions)[0].Status).To(Equal(metav1.ConditionTrue))
			Expect((*update.Status.Conditions)[0].Reason).To(Equal(string(argorav1alpha1.ConditionReasonUpdateSucceeded)))
		})
	})
})
