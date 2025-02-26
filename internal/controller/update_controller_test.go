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
	"errors"

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
	const resourceName = "test-resource"
	const resourceNamespace = "default"

	fileReaderMock := &mock.FileReaderMock{
		FileContent: make(map[string]string),
		ReturnError: false,
	}
	fileReaderMock.FileContent["/etc/config/config.json"] = `{
		"ironCoreRoles": "role1",
		"ironCoreRegion": "region1",
		"serverController": "ironcore"
	}`
	fileReaderMock.FileContent["/etc/credentials/credentials.json"] = `{
		"netboxUrl": "aHR0cDovL25ldGJveA==",
		"netboxToken": "dG9rZW4=",
		"bmcUser": "dXNlcg==",
		"bmcPassword": "cGFzc3dvcmQ="
	}`

	Context("when reconciling a Update CR", func() {
		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: resourceNamespace,
		}

		update := &argorav1alpha1.Update{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: resourceNamespace,
			},
		}

		expectStatus := func(state argorav1alpha1.State, description string) {
			err := k8sClient.Get(ctx, typeNamespacedName, update)
			Expect(err).NotTo(HaveOccurred())
			Expect(update.Status.State).To(Equal(state))
			Expect(update.Status.Description).To(Equal(description))
			Expect(update.Status.Conditions).ToNot(BeNil())
			Expect((*update.Status.Conditions)).To(HaveLen(1))
			if state == argorav1alpha1.Ready {
				Expect((*update.Status.Conditions)[0].Type).To(Equal(string(argorav1alpha1.ConditionTypeReady)))
				Expect((*update.Status.Conditions)[0].Status).To(Equal(metav1.ConditionTrue))
				Expect((*update.Status.Conditions)[0].Reason).To(Equal(string(argorav1alpha1.ConditionReasonUpdateSucceeded)))
				Expect((*update.Status.Conditions)[0].Message).To(Equal(argorav1alpha1.ConditionReasonUpdateSucceededMessage))
			} else {
				Expect((*update.Status.Conditions)[0].Type).To(Equal(string(argorav1alpha1.ConditionTypeReady)))
				Expect((*update.Status.Conditions)[0].Status).To(Equal(metav1.ConditionFalse))
				Expect((*update.Status.Conditions)[0].Reason).To(Equal(string(argorav1alpha1.ConditionReasonUpdateFailed)))
				Expect((*update.Status.Conditions)[0].Message).To(Equal(argorav1alpha1.ConditionReasonUpdateFailedMessage))
			}
		}

		BeforeEach(func() {
			By("creating Update CR")
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

			By("cleanup Update CR")
			Expect(k8sClient.Delete(ctx, update)).To(Succeed())
		})

		It("should successfully reconcile the CR", func() {
			// given
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

			controllerReconciler := createReconciler(netBoxMock, fileReaderMock)

			// when
			By("reconciling Update CR")
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})

			// then
			Expect(err).NotTo(HaveOccurred())
			netBoxMock.VirtualizationMock.(*mock.VirtualizationMock).GetClusterByNameRegionTypeCalls = 1
			netBoxMock.DCIMMock.(*mock.DCIMMock).GetDevicesByClusterIDCalls = 1
			netBoxMock.DCIMMock.(*mock.DCIMMock).GetInterfacesForDeviceCalls = 2
			netBoxMock.DCIMMock.(*mock.DCIMMock).GetInterfaceForDeviceCalls = 1
			netBoxMock.IPAMMock.(*mock.IPAMMock).GetIPAddressForInterfaceCalls = 1
			netBoxMock.DCIMMock.(*mock.DCIMMock).GetPlatformByNameCalls = 1

			expectStatus(argorav1alpha1.Ready, "")
		})

		It("should return an error if netbox reload fails", func() {
			// given
			netBoxMock := &mock.NetBoxMock{
				ReturnError: true,
			}

			controllerReconciler := createReconciler(netBoxMock, fileReaderMock)

			// when
			By("reconciling Update CR")
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})

			// then
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unable to reload netbox"))
		})

		It("should return an error if GetClusterByNameRegionType fails", func() {
			// given
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

				return nil, errors.New("unable to find clusters")
			}

			controllerReconciler := createReconciler(netBoxMock, fileReaderMock)

			// when
			By("reconciling Update CR")
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})

			// then
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unable to find clusters"))

			expectStatus(argorav1alpha1.Error, "unable to reconcile cluster: unable to find clusters")
		})

		It("should return an error if GetDevicesByClusterID fails", func() {
			// given
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
				return nil, errors.New("unable to find devices")
			}

			controllerReconciler := createReconciler(netBoxMock, fileReaderMock)

			// when
			By("reconciling Update CR")
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})

			// then
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unable to find devices"))

			expectStatus(argorav1alpha1.Error, "unable to reconcile devices on cluster cluster1 (1): unable to find devices")
		})

		It("should skip the device when the device is not active", func() {
			// given
			netBoxMock := &mock.NetBoxMock{
				ReturnError:        false,
				VirtualizationMock: &mock.VirtualizationMock{},
				DCIMMock:           &mock.DCIMMock{},
				IPAMMock:           &mock.IPAMMock{},
				ExtrasMock:         &mock.ExtrasMock{},
			}

			netBoxMock.VirtualizationMock.(*mock.VirtualizationMock).GetClusterByNameRegionTypeFunc = func(name, region, clusterType string) (*models.Cluster, error) {
				return &models.Cluster{
					ID:   1,
					Name: "cluster1",
				}, nil
			}
			netBoxMock.DCIMMock.(*mock.DCIMMock).GetDevicesByClusterIDFunc = func(clusterID int) ([]models.Device, error) {
				return []models.Device{
					{
						ID:     1,
						Name:   "device1",
						Status: models.DeviceStatus{Value: "inactive"},
					},
				}, nil
			}

			controllerReconciler := createReconciler(netBoxMock, fileReaderMock)

			// when
			By("reconciling Update CR")
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})

			// then
			Expect(err).NotTo(HaveOccurred())

			expectStatus(argorav1alpha1.Ready, "")
		})

		It("should rename iDRAC interface to remoteboard", func() {
			// given
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
						Name:            "iDRAC",
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
			netBoxMock.DCIMMock.(*mock.DCIMMock).UpdateInterfaceFunc = func(wIface models.WritableInterface, ifaceID int) (*models.Interface, error) {
				Expect(wIface.Name).To(Equal("remoteboard"))
				Expect(wIface.Device).To(Equal(1))
				Expect(wIface.Type).To(BeEmpty())
				Expect(ifaceID).To(Equal(1))
				return &models.Interface{
					NestedInterface: models.NestedInterface{ID: 1},
					Name:            "remoteboard",
				}, nil
			}

			controllerReconciler := createReconciler(netBoxMock, fileReaderMock)

			// when
			By("reconciling Update CR")
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})

			// then
			Expect(err).NotTo(HaveOccurred())
			Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).UpdateInterfaceCalls).To(Equal(1))

			expectStatus(argorav1alpha1.Ready, "")
		})

		It("should return error when iDRAC interface renaming to remoteboard fails", func() {
			// given
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
						Name:            "iDRAC",
					},
				}, nil
			}
			netBoxMock.DCIMMock.(*mock.DCIMMock).UpdateInterfaceFunc = func(wIface models.WritableInterface, ifaceID int) (*models.Interface, error) {
				Expect(wIface.Name).To(Equal("remoteboard"))
				Expect(wIface.Device).To(Equal(1))
				Expect(wIface.Type).To(BeEmpty())
				Expect(ifaceID).To(Equal(1))
				return nil, errors.New("unable to rename iDRAC interface")
			}

			controllerReconciler := createReconciler(netBoxMock, fileReaderMock)

			// when
			By("reconciling Update CR")
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})

			// then
			Expect(err).To(HaveOccurred())
			Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).UpdateInterfaceCalls).To(Equal(1))

			expectStatus(argorav1alpha1.Error, "unable to reconcile device device1 (1) on cluster cluster1 (1): unable to rename remoteboard interface for device device1: unable to rename iDRAC interface: unable to rename iDRAC interface")
		})

		It("should return error when the device has no remoteboard interface", func() {
			// given
			netBoxMock := &mock.NetBoxMock{
				ReturnError:        false,
				VirtualizationMock: &mock.VirtualizationMock{},
				DCIMMock:           &mock.DCIMMock{},
				IPAMMock:           &mock.IPAMMock{},
				ExtrasMock:         &mock.ExtrasMock{},
			}

			netBoxMock.VirtualizationMock.(*mock.VirtualizationMock).GetClusterByNameRegionTypeFunc = func(name, region, clusterType string) (*models.Cluster, error) {
				return &models.Cluster{
					ID:   1,
					Name: "cluster1",
				}, nil
			}
			netBoxMock.DCIMMock.(*mock.DCIMMock).GetDevicesByClusterIDFunc = func(clusterID int) ([]models.Device, error) {
				return []models.Device{
					{
						ID:     1,
						Name:   "device1",
						Status: models.DeviceStatus{Value: "active"},
					},
				}, nil
			}
			netBoxMock.DCIMMock.(*mock.DCIMMock).GetInterfacesForDeviceFunc = func(device *models.Device) ([]models.Interface, error) {
				return []models.Interface{}, nil
			}
			netBoxMock.DCIMMock.(*mock.DCIMMock).GetInterfaceForDeviceFunc = func(device *models.Device, ifaceName string) (*models.Interface, error) {
				Expect(ifaceName).To(Equal("remoteboard"))
				return nil, errors.New("remoteboard interface not found")
			}

			controllerReconciler := createReconciler(netBoxMock, fileReaderMock)

			// when
			By("reconciling Update CR")
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})

			// then
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("unable to update device device1 data: remoteboard interface not found"))

			expectStatus(argorav1alpha1.Error, "unable to reconcile device device1 (1) on cluster cluster1 (1): unable to update device device1 data: remoteboard interface not found")
		})

		It("should update device data when plaform does not match", func() {
			// given
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
					NestedPlatform: models.NestedPlatform{ID: 2},
				}, nil
			}
			netBoxMock.DCIMMock.(*mock.DCIMMock).UpdateDeviceFunc = func(device models.WritableDeviceWithConfigContext) (*models.Device, error) {
				Expect(device.Platform).To(Equal(2))
				Expect(device.OOBIp).To(Equal(1))
				Expect(device.ID).To(Equal(1))
				return &models.Device{
					ID: 1,
				}, nil
			}

			controllerReconciler := createReconciler(netBoxMock, fileReaderMock)

			// when
			By("reconciling Update CR")
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})

			// then
			Expect(err).NotTo(HaveOccurred())
			Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).UpdateDeviceCalls).To(Equal(1))

			expectStatus(argorav1alpha1.Ready, "")
		})

		It("should update device data when OOB IP does not match", func() {
			// given
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
						ID: 2,
					},
				}, nil
			}
			netBoxMock.DCIMMock.(*mock.DCIMMock).GetPlatformByNameFunc = func(platformName string) (*models.Platform, error) {
				Expect(platformName).To(Equal("Linux KVM"))
				return &models.Platform{
					NestedPlatform: models.NestedPlatform{ID: 1},
				}, nil
			}
			netBoxMock.DCIMMock.(*mock.DCIMMock).UpdateDeviceFunc = func(device models.WritableDeviceWithConfigContext) (*models.Device, error) {
				Expect(device.Platform).To(Equal(1))
				Expect(device.OOBIp).To(Equal(2))
				Expect(device.ID).To(Equal(1))
				return &models.Device{
					ID: 1,
				}, nil
			}

			controllerReconciler := createReconciler(netBoxMock, fileReaderMock)

			// when
			By("reconciling Update CR")
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})

			// then
			Expect(err).NotTo(HaveOccurred())
			Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).UpdateDeviceCalls).To(Equal(1))

			expectStatus(argorav1alpha1.Ready, "")
		})

		It("should return error when update device data fails", func() {
			// given
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
					NestedPlatform: models.NestedPlatform{ID: 2},
				}, nil
			}
			netBoxMock.DCIMMock.(*mock.DCIMMock).UpdateDeviceFunc = func(device models.WritableDeviceWithConfigContext) (*models.Device, error) {
				Expect(device.Platform).To(Equal(2))
				Expect(device.ID).To(Equal(1))
				return nil, errors.New("unable to update device")
			}

			controllerReconciler := createReconciler(netBoxMock, fileReaderMock)

			// when
			By("reconciling Update CR")
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})

			// then
			Expect(err).To(HaveOccurred())
			Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).UpdateDeviceCalls).To(Equal(1))

			expectStatus(argorav1alpha1.Error, "unable to reconcile device device1 (1) on cluster cluster1 (1): unable to update device device1 data: unable to update device")
		})

		It("should remove VMK interfaces and IPs", func() {
			// given
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
					{
						NestedInterface: models.NestedInterface{ID: 2},
						Name:            "vmk0",
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
			netBoxMock.IPAMMock.(*mock.IPAMMock).GetIPAddressesForInterfaceFunc = func(ifaceID int) ([]models.IPAddress, error) {
				Expect(ifaceID).To(Equal(2))
				return []models.IPAddress{
					{
						NestedIPAddress: models.NestedIPAddress{ID: 2},
					},
				}, nil
			}
			netBoxMock.IPAMMock.(*mock.IPAMMock).DeleteIPAddressFunc = func(ipID int) error {
				Expect(ipID).To(Equal(2))
				return nil
			}
			netBoxMock.DCIMMock.(*mock.DCIMMock).DeleteInterfaceFunc = func(ifaceID int) error {
				Expect(ifaceID).To(Equal(2))
				return nil
			}

			controllerReconciler := createReconciler(netBoxMock, fileReaderMock)

			// when
			By("reconciling Update CR")
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})

			// then
			Expect(err).NotTo(HaveOccurred())
			Expect(netBoxMock.IPAMMock.(*mock.IPAMMock).DeleteIPAddressCalls).To(Equal(1))
			Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).DeleteInterfaceCalls).To(Equal(1))

			expectStatus(argorav1alpha1.Ready, "")
		})
	})

	It("should return an error when the Update CR is not found", func() {
		// given
		netBoxMock := &mock.NetBoxMock{
			ReturnError:        false,
			VirtualizationMock: &mock.VirtualizationMock{},
			DCIMMock:           &mock.DCIMMock{},
			IPAMMock:           &mock.IPAMMock{},
			ExtrasMock:         &mock.ExtrasMock{},
		}

		controllerReconciler := createReconciler(netBoxMock, fileReaderMock)

		// when
		By("reconciling Update CR")
		res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      "non-existent-resource",
				Namespace: resourceNamespace,
			},
		})

		// then
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("updates.argora.cloud.sap \"non-existent-resource\" not found"))
		Expect(res).To(Equal(reconcile.Result{}))
	})
})

func createReconciler(netBoxMock *mock.NetBoxMock, fileReaderMock config.FileReader) *UpdateReconciler {
	return &UpdateReconciler{
		k8sClient:         k8sClient,
		scheme:            k8sClient.Scheme(),
		netBox:            netBoxMock,
		cfg:               config.NewDefaultConfiguration(k8sClient, fileReaderMock),
		reconcileInterval: reconcileInterval,
	}
}
