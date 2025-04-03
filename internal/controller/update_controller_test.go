// Copyright 2025 SAP SE
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/sapcc/go-netbox-go/models"

	argorav1alpha1 "github.com/sapcc/argora/api/v1alpha1"
	"github.com/sapcc/argora/internal/config"
	"github.com/sapcc/argora/internal/controller/mock"
	"github.com/sapcc/argora/internal/status"

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
		"serverController": "ironcore",
		"ironCore": {
			"name": "name1",
			"region": "region1",
			"types": "type1"
		},
		"netboxUrl": "http://netbox"
	}`
	fileReaderMock.FileContent["/etc/credentials/credentials.json"] = `{
		"bmcUser": "user",
		"bmcPassword": "password",
		"netboxToken": "token"
	}`

	Context("Reconcile", func() {
		ctx := context.Background()

		typeNamespacedUpdateName := types.NamespacedName{
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
			err := k8sClient.Get(ctx, typeNamespacedUpdateName, update)
			Expect(err).ToNot(HaveOccurred())
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

		prepareNetboxMock := func() *mock.NetBoxMock {
			netBoxMock := &mock.NetBoxMock{
				ReturnError:        false,
				VirtualizationMock: &mock.VirtualizationMock{},
				DCIMMock:           &mock.DCIMMock{},
				IPAMMock:           &mock.IPAMMock{},
				ExtrasMock:         &mock.ExtrasMock{},
			}

			netBoxMock.VirtualizationMock.(*mock.VirtualizationMock).GetClustersByNameRegionTypeFunc = func(name, region, clusterType string) ([]models.Cluster, error) {
				Expect(name).To(BeEmpty())
				Expect(region).To(Equal("region1"))
				Expect(clusterType).To(Equal("type1"))

				return []models.Cluster{
					{
						ID:   1,
						Name: "cluster1",
					},
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
				Expect(platformName).To(Equal("GardenLinux"))
				return &models.Platform{
					NestedPlatform: models.NestedPlatform{ID: 1},
				}, nil
			}

			return netBoxMock
		}

		BeforeEach(func() {
			By("create Update CR")
			err := k8sClient.Get(ctx, typeNamespacedUpdateName, update)
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
			err := k8sClient.Get(ctx, typeNamespacedUpdateName, update)
			Expect(err).ToNot(HaveOccurred())

			By("delete Update CR")
			Expect(k8sClient.Delete(ctx, update)).To(Succeed())
		})

		It("should successfully reconcile the CR", func() {
			// given
			netBoxMock := prepareNetboxMock()
			controllerReconciler := createUpdateReconciler(netBoxMock, fileReaderMock)

			// when
			By("reconciling Update CR")
			res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedUpdateName,
			})

			// then
			Expect(err).ToNot(HaveOccurred())
			Expect(res.RequeueAfter).To(Equal(reconcileInterval))

			netBoxMock.VirtualizationMock.(*mock.VirtualizationMock).GetClustersByNameRegionTypeCalls = 1
			netBoxMock.DCIMMock.(*mock.DCIMMock).GetDevicesByClusterIDCalls = 1
			netBoxMock.DCIMMock.(*mock.DCIMMock).GetInterfacesForDeviceCalls = 2
			netBoxMock.DCIMMock.(*mock.DCIMMock).GetInterfaceForDeviceCalls = 1
			netBoxMock.IPAMMock.(*mock.IPAMMock).GetIPAddressForInterfaceCalls = 1
			netBoxMock.DCIMMock.(*mock.DCIMMock).GetPlatformByNameCalls = 1

			expectStatus(argorav1alpha1.Ready, "")
		})

		It("should return an error if configuration reload fails", func() {
			// given
			netBoxMock := prepareNetboxMock()
			fileReaderMockToError := &mock.FileReaderMock{
				FileContent: make(map[string]string),
				ReturnError: true,
			}
			controllerReconciler := createUpdateReconciler(netBoxMock, fileReaderMockToError)

			// when
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{})

			// then
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unable to read config.json: error"))
		})

		It("should return an error if netbox reload fails", func() {
			// given
			controllerReconciler := createUpdateReconciler(&mock.NetBoxMock{ReturnError: true}, fileReaderMock)

			// when
			By("reconciling Update CR")
			res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedUpdateName,
			})

			// then
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unable to reload netbox"))
			Expect(res.Requeue).To(BeFalse())
		})

		It("should return an error if GetClustersByNameRegionType fails", func() {
			// given
			netBoxMock := prepareNetboxMock()
			netBoxMock.VirtualizationMock.(*mock.VirtualizationMock).GetClustersByNameRegionTypeFunc = func(name, region, clusterType string) ([]models.Cluster, error) {
				Expect(name).To(BeEmpty())
				Expect(region).To(Equal("region1"))
				Expect(clusterType).To(Equal("type1"))

				return nil, errors.New("unable to find clusters")
			}

			controllerReconciler := createUpdateReconciler(netBoxMock, fileReaderMock)

			// when
			By("reconciling Update CR")
			res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedUpdateName,
			})

			// then
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unable to find clusters"))
			Expect(res.Requeue).To(BeFalse())

			expectStatus(argorav1alpha1.Error, "unable to reconcile cluster: unable to find clusters")
		})

		It("should return an error if GetDevicesByClusterID fails", func() {
			// given
			netBoxMock := prepareNetboxMock()
			netBoxMock.VirtualizationMock.(*mock.VirtualizationMock).GetClustersByNameRegionTypeFunc = func(name, region, clusterType string) ([]models.Cluster, error) {
				Expect(name).To(BeEmpty())
				Expect(region).To(Equal("region1"))
				Expect(clusterType).To(Equal("type1"))

				return []models.Cluster{
					{
						ID:   1,
						Name: "cluster1",
					},
				}, nil
			}
			netBoxMock.DCIMMock.(*mock.DCIMMock).GetDevicesByClusterIDFunc = func(clusterID int) ([]models.Device, error) {
				Expect(clusterID).To(Equal(1))
				return nil, errors.New("unable to find devices")
			}

			controllerReconciler := createUpdateReconciler(netBoxMock, fileReaderMock)

			// when
			By("reconciling Update CR")
			res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedUpdateName,
			})

			// then
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unable to find devices"))
			Expect(res.Requeue).To(BeFalse())

			expectStatus(argorav1alpha1.Error, "unable to reconcile devices on cluster cluster1 (1): unable to find devices")
		})

		It("should succeed when device is in staged status", func() {
			// given
			netBoxMock := prepareNetboxMock()
			netBoxMock.DCIMMock.(*mock.DCIMMock).GetDevicesByClusterIDFunc = func(clusterID int) ([]models.Device, error) {
				Expect(clusterID).To(Equal(1))
				return []models.Device{
					{
						ID:     1,
						Name:   "device1",
						Status: models.DeviceStatus{Value: "staged"},
						Platform: models.NestedPlatform{
							ID: 1,
						},
						OOBIp: models.NestedIPAddress{
							ID: 1,
						},
					},
				}, nil
			}
			controllerReconciler := createUpdateReconciler(netBoxMock, fileReaderMock)

			// when
			By("reconciling Update CR")
			res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedUpdateName,
			})

			// then
			Expect(err).ToNot(HaveOccurred())
			Expect(res.RequeueAfter).To(Equal(reconcileInterval))

			netBoxMock.VirtualizationMock.(*mock.VirtualizationMock).GetClustersByNameRegionTypeCalls = 1
			netBoxMock.DCIMMock.(*mock.DCIMMock).GetDevicesByClusterIDCalls = 1
			netBoxMock.DCIMMock.(*mock.DCIMMock).GetInterfacesForDeviceCalls = 2
			netBoxMock.DCIMMock.(*mock.DCIMMock).GetInterfaceForDeviceCalls = 1
			netBoxMock.IPAMMock.(*mock.IPAMMock).GetIPAddressForInterfaceCalls = 1
			netBoxMock.DCIMMock.(*mock.DCIMMock).GetPlatformByNameCalls = 1

			expectStatus(argorav1alpha1.Ready, "")
		})

		It("should skip the device when the device is not active or staged", func() {
			// given
			netBoxMock := prepareNetboxMock()
			netBoxMock.DCIMMock.(*mock.DCIMMock).GetDevicesByClusterIDFunc = func(clusterID int) ([]models.Device, error) {
				return []models.Device{
					{
						ID:     1,
						Name:   "device1",
						Status: models.DeviceStatus{Value: "planned"},
					},
				}, nil
			}

			controllerReconciler := createUpdateReconciler(netBoxMock, fileReaderMock)

			// when
			By("reconciling Update CR")
			res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedUpdateName,
			})

			// then
			Expect(err).ToNot(HaveOccurred())
			Expect(res.Requeue).To(BeFalse())

			expectStatus(argorav1alpha1.Ready, "")
		})

		It("should rename iDRAC interface to remoteboard", func() {
			// given
			netBoxMock := prepareNetboxMock()
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
				return &models.Interface{
					NestedInterface: models.NestedInterface{ID: 1},
					Name:            "remoteboard",
				}, nil
			}

			controllerReconciler := createUpdateReconciler(netBoxMock, fileReaderMock)

			// when
			By("reconciling Update CR")
			res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedUpdateName,
			})

			// then
			Expect(err).ToNot(HaveOccurred())
			Expect(res.Requeue).To(BeFalse())

			Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).UpdateInterfaceCalls).To(Equal(1))

			expectStatus(argorav1alpha1.Ready, "")
		})

		It("should return error when iDRAC interface renaming to remoteboard fails", func() {
			// given
			netBoxMock := prepareNetboxMock()
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

			controllerReconciler := createUpdateReconciler(netBoxMock, fileReaderMock)

			// when
			By("reconciling Update CR")
			res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedUpdateName,
			})

			// then
			Expect(err).To(HaveOccurred())
			Expect(res.Requeue).To(BeFalse())

			Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).UpdateInterfaceCalls).To(Equal(1))

			expectStatus(argorav1alpha1.Error, "unable to reconcile device device1 (1) on cluster cluster1 (1): unable to rename remoteboard interface for device device1: unable to rename iDRAC interface: unable to rename iDRAC interface")
		})

		It("should return error when the device has no remoteboard interface", func() {
			// given
			netBoxMock := prepareNetboxMock()
			netBoxMock.DCIMMock.(*mock.DCIMMock).GetInterfacesForDeviceFunc = func(device *models.Device) ([]models.Interface, error) {
				return []models.Interface{}, nil
			}
			netBoxMock.DCIMMock.(*mock.DCIMMock).GetInterfaceForDeviceFunc = func(device *models.Device, ifaceName string) (*models.Interface, error) {
				Expect(ifaceName).To(Equal("remoteboard"))
				return nil, errors.New("remoteboard interface not found")
			}

			controllerReconciler := createUpdateReconciler(netBoxMock, fileReaderMock)

			// when
			By("reconciling Update CR")
			res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedUpdateName,
			})

			// then
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("unable to update device device1 data: remoteboard interface not found"))
			Expect(res.Requeue).To(BeFalse())

			Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).UpdateDeviceCalls).To(Equal(0))

			expectStatus(argorav1alpha1.Error, "unable to reconcile device device1 (1) on cluster cluster1 (1): unable to update device device1 data: remoteboard interface not found")
		})

		It("should update device data when plaform does not match", func() {
			// given
			netBoxMock := prepareNetboxMock()
			netBoxMock.DCIMMock.(*mock.DCIMMock).GetPlatformByNameFunc = func(platformName string) (*models.Platform, error) {
				Expect(platformName).To(Equal("GardenLinux"))
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

			controllerReconciler := createUpdateReconciler(netBoxMock, fileReaderMock)

			// when
			By("reconciling Update CR")
			res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedUpdateName,
			})

			// then
			Expect(err).ToNot(HaveOccurred())
			Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).UpdateDeviceCalls).To(Equal(1))
			Expect(res.Requeue).To(BeFalse())

			expectStatus(argorav1alpha1.Ready, "")
		})

		It("should update device data when OOB IP does not match", func() {
			// given
			netBoxMock := prepareNetboxMock()
			netBoxMock.IPAMMock.(*mock.IPAMMock).GetIPAddressForInterfaceFunc = func(ifaceID int) (*models.IPAddress, error) {
				Expect(ifaceID).To(Equal(1))
				return &models.IPAddress{
					NestedIPAddress: models.NestedIPAddress{
						ID: 2,
					},
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

			controllerReconciler := createUpdateReconciler(netBoxMock, fileReaderMock)

			// when
			By("reconciling Update CR")
			res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedUpdateName,
			})

			// then
			Expect(err).ToNot(HaveOccurred())
			Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).UpdateDeviceCalls).To(Equal(1))
			Expect(res.Requeue).To(BeFalse())

			expectStatus(argorav1alpha1.Ready, "")
		})

		It("should return error when update device data fails", func() {
			// given
			netBoxMock := prepareNetboxMock()
			netBoxMock.DCIMMock.(*mock.DCIMMock).GetPlatformByNameFunc = func(platformName string) (*models.Platform, error) {
				Expect(platformName).To(Equal("GardenLinux"))
				return &models.Platform{
					NestedPlatform: models.NestedPlatform{ID: 2},
				}, nil
			}
			netBoxMock.DCIMMock.(*mock.DCIMMock).UpdateDeviceFunc = func(device models.WritableDeviceWithConfigContext) (*models.Device, error) {
				Expect(device.Platform).To(Equal(2))
				Expect(device.ID).To(Equal(1))
				return nil, errors.New("unable to update device")
			}

			controllerReconciler := createUpdateReconciler(netBoxMock, fileReaderMock)

			// when
			By("reconciling Update CR")
			res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedUpdateName,
			})

			// then
			Expect(err).To(HaveOccurred())
			Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).UpdateDeviceCalls).To(Equal(1))
			Expect(res.Requeue).To(BeFalse())

			expectStatus(argorav1alpha1.Error, "unable to reconcile device device1 (1) on cluster cluster1 (1): unable to update device device1 data: unable to update device")
		})

		It("should remove VMK interfaces and IPs", func() {
			// given
			netBoxMock := prepareNetboxMock()
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

			controllerReconciler := createUpdateReconciler(netBoxMock, fileReaderMock)

			// when
			By("reconciling Update CR")
			res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedUpdateName,
			})

			// then
			Expect(err).ToNot(HaveOccurred())
			Expect(netBoxMock.IPAMMock.(*mock.IPAMMock).DeleteIPAddressCalls).To(Equal(1))
			Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).DeleteInterfaceCalls).To(Equal(1))
			Expect(res.Requeue).To(BeFalse())

			expectStatus(argorav1alpha1.Ready, "")
		})

		It("should return error when unable to remove VMK interface IPs", func() {
			// given
			netBoxMock := prepareNetboxMock()
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
			netBoxMock.IPAMMock.(*mock.IPAMMock).GetIPAddressesForInterfaceFunc = func(ifaceID int) ([]models.IPAddress, error) {
				Expect(ifaceID).To(Equal(2))
				return []models.IPAddress{
					{
						NestedIPAddress: models.NestedIPAddress{
							ID:      2,
							Address: "192.168.0.1",
						},
					},
				}, nil
			}
			netBoxMock.IPAMMock.(*mock.IPAMMock).DeleteIPAddressFunc = func(ipID int) error {
				Expect(ipID).To(Equal(2))
				return errors.New("failed to delete IP (2)")
			}

			controllerReconciler := createUpdateReconciler(netBoxMock, fileReaderMock)

			// when
			By("reconciling Update CR")
			res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedUpdateName,
			})

			// then
			Expect(err).To(HaveOccurred())
			Expect(netBoxMock.IPAMMock.(*mock.IPAMMock).DeleteIPAddressCalls).To(Equal(1))
			Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).DeleteInterfaceCalls).To(Equal(0))
			Expect(res.Requeue).To(BeFalse())

			expectStatus(argorav1alpha1.Error, "unable to reconcile device device1 (1) on cluster cluster1 (1): unable to remove vmk interfaces and IPs for device device1: unable to delete IP address (192.168.0.1): failed to delete IP (2)")
		})

		It("should return error when unable to remove VMK interface", func() {
			// given
			netBoxMock := prepareNetboxMock()
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
				return errors.New("failed to delete interface (2)")
			}

			controllerReconciler := createUpdateReconciler(netBoxMock, fileReaderMock)

			// when
			By("reconciling Update CR")
			res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedUpdateName,
			})

			// then
			Expect(err).To(HaveOccurred())
			Expect(netBoxMock.IPAMMock.(*mock.IPAMMock).DeleteIPAddressCalls).To(Equal(1))
			Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).DeleteInterfaceCalls).To(Equal(1))
			Expect(res.Requeue).To(BeFalse())

			expectStatus(argorav1alpha1.Error, "unable to reconcile device device1 (1) on cluster cluster1 (1): unable to remove vmk interfaces and IPs for device device1: unable to delete vmk0 interface: failed to delete interface (2)")
		})
	})

	It("should return an error when an Update CR is not found", func() {
		// given
		netBoxMock := &mock.NetBoxMock{
			ReturnError:        false,
			VirtualizationMock: &mock.VirtualizationMock{},
			DCIMMock:           &mock.DCIMMock{},
			IPAMMock:           &mock.IPAMMock{},
			ExtrasMock:         &mock.ExtrasMock{},
		}

		controllerReconciler := createUpdateReconciler(netBoxMock, fileReaderMock)

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
		Expect(res.Requeue).To(BeFalse())
	})
})

func createUpdateReconciler(netBoxMock *mock.NetBoxMock, fileReaderMock config.FileReader) *UpdateReconciler {
	return &UpdateReconciler{
		k8sClient:         k8sClient,
		scheme:            k8sClient.Scheme(),
		statusHandler:     status.NewStatusHandler(k8sClient),
		netBox:            netBoxMock,
		cfg:               config.NewDefaultConfiguration(k8sClient, fileReaderMock),
		reconcileInterval: reconcileInterval,
	}
}
