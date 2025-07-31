// Copyright 2025 SAP SE
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"strings"
	"time"

	metalv1alpha1 "github.com/ironcore-dev/metal-operator/api/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/sapcc/go-netbox-go/models"

	"github.com/sapcc/argora/internal/config"
	"github.com/sapcc/argora/internal/controller/mock"
)

var _ = Describe("Ironcore Controller", func() {
	const (
		resourceName      = "test-resource"
		resourceNamespace = "default"
		bmcName1          = "device-name1"
		bmcName2          = "device-name2"
	)

	fileReaderMock := &mock.FileReaderMock{
		FileContent: make(map[string]string),
		ReturnError: false,
	}
	fileReaderMock.FileContent["/etc/config/config.json"] = `{
		"serverController": "ironcore",
		"ironCore": [{
			"name": "name1",
			"region": "region1",
			"type": "type1"
		}],
		"netboxUrl": "http://netbox"
	}`
	fileReaderMock.FileContent["/etc/credentials/credentials.json"] = `{
		"bmcUser": "user",
		"bmcPassword": "password",
		"netboxToken": "token"
	}`

	Context("Reconcile", func() {
		ctx := context.Background()

		typeNamespacedBMCName1 := types.NamespacedName{
			Name:      bmcName1,
			Namespace: resourceNamespace,
		}
		typeNamespacedBMCName2 := types.NamespacedName{
			Name:      bmcName2,
			Namespace: resourceNamespace,
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
				Expect(name).To(Equal("name1"))
				Expect(region).To(Equal("region1"))
				Expect(clusterType).To(Equal("type1"))

				return []models.Cluster{
					{
						ID:   1,
						Name: "cluster1",
						Type: models.NestedClusterType{
							Slug: clusterType,
						},
					},
				}, nil
			}
			netBoxMock.DCIMMock.(*mock.DCIMMock).GetDevicesByClusterIDFunc = func(clusterID int) ([]models.Device, error) {
				Expect(clusterID).To(BeElementOf(1, 2))
				return []models.Device{
					{
						ID:     clusterID,
						Name:   "device-name1",
						Status: models.DeviceStatus{Value: "active"},
						Platform: models.NestedPlatform{
							ID: 1,
						},
						OOBIp: models.NestedIPAddress{
							ID:      1,
							Address: "192.168.1.1/24",
						},
						Site: models.NestedSite{
							Slug: "site1",
						},
						DeviceType: models.NestedDeviceType{
							Slug: "type1",
						},
						DeviceRole: models.NestedDeviceRole{
							Slug: "role1",
						},
					},
				}, nil
			}
			netBoxMock.DCIMMock.(*mock.DCIMMock).GetRegionForDeviceFunc = func(device *models.Device) (string, error) {
				Expect(device.Name).To(Equal("device-name1"))
				return "region1", nil
			}

			return netBoxMock
		}

		expectLabels := func(labels map[string]string, bmcName string) {
			bb, _ := strings.CutPrefix(bmcName, "device-")
			Expect(labels).To(ConsistOf(map[string]string{
				"topology.kubernetes.io/region":           "region1",
				"topology.kubernetes.io/zone":             "site1",
				"kubernetes.metal.cloud.sap/cluster":      "cluster1",
				"kubernetes.metal.cloud.sap/cluster-type": "type1",
				"kubernetes.metal.cloud.sap/name":         bmcName,
				"kubernetes.metal.cloud.sap/bb":           bb,
				"kubernetes.metal.cloud.sap/type":         "type1",
				"kubernetes.metal.cloud.sap/role":         "role1",
				"kubernetes.metal.cloud.sap/platform":     "platform1",
			}))
		}

		expectBMCSecret := func(bmcSecret *metalv1alpha1.BMCSecret, bmcName string) {
			Expect(bmcSecret.Name).To(Equal(bmcName))
			expectLabels(bmcSecret.Labels, bmcName)

			Expect(bmcSecret.ObjectMeta.OwnerReferences).To(HaveLen(1))
			Expect(bmcSecret.ObjectMeta.OwnerReferences[0].APIVersion).To(Equal("metal.ironcore.dev/v1alpha1"))
			Expect(bmcSecret.ObjectMeta.OwnerReferences[0].Kind).To(Equal("BMC"))
			Expect(bmcSecret.ObjectMeta.OwnerReferences[0].Name).To(Equal(bmcName))
			Expect(*bmcSecret.ObjectMeta.OwnerReferences[0].Controller).To(BeTrue())
			Expect(*bmcSecret.ObjectMeta.OwnerReferences[0].BlockOwnerDeletion).To(BeTrue())

			Expect(bmcSecret.Data).To(Equal(map[string][]byte{
				metalv1alpha1.BMCSecretUsernameKeyName: []byte("user"),
				metalv1alpha1.BMCSecretPasswordKeyName: []byte("password"),
			}))
		}

		expectBMC := func(bmc *metalv1alpha1.BMC, bmcName string) {
			Expect(bmc.Name).To(Equal(bmcName))
			expectLabels(bmc.Labels, bmcName)
			Expect(bmc.ObjectMeta.OwnerReferences).To(BeEmpty())

			Expect(bmc.Spec.Endpoint.IP.String()).To(Equal("192.168.1.1"))
			Expect(bmc.Spec.Protocol.Name).To(Equal(metalv1alpha1.ProtocolNameRedfish))
			Expect(bmc.Spec.Protocol.Port).To(Equal(int32(443)))
			Expect(bmc.Spec.BMCSecretRef.Name).To(Equal(bmcName))
		}

		expectBMCResources := func() {
			bmcSecretList := &metalv1alpha1.BMCSecretList{}
			err := k8sClient.List(ctx, bmcSecretList)
			Expect(err).ToNot(HaveOccurred())
			Expect(bmcSecretList.Items).To(HaveLen(1))

			bmcSecret := &metalv1alpha1.BMCSecret{}
			err = k8sClient.Get(ctx, typeNamespacedBMCName1, bmcSecret)
			Expect(err).ToNot(HaveOccurred())
			expectBMCSecret(bmcSecret, bmcName1)

			bmcList := &metalv1alpha1.BMCList{}
			err = k8sClient.List(ctx, bmcList)
			Expect(err).ToNot(HaveOccurred())
			Expect(bmcList.Items).To(HaveLen(1))

			bmc := &metalv1alpha1.BMC{}
			err = k8sClient.Get(ctx, typeNamespacedBMCName1, bmc)
			Expect(err).ToNot(HaveOccurred())
			expectBMC(bmc, bmcName1)
		}

		Context("Envtest", func() {
			AfterEach(func() {
				By("cleanup")
				bmcSecret := &metalv1alpha1.BMCSecret{}
				err := k8sClient.Get(ctx, typeNamespacedBMCName1, bmcSecret)
				if err == nil {
					By("delete BMC Secret 1")
					Expect(k8sClient.Delete(ctx, bmcSecret)).To(Succeed())
				}
				err = k8sClient.Get(ctx, typeNamespacedBMCName2, bmcSecret)
				if err == nil {
					By("delete BMC Secret 2")
					Expect(k8sClient.Delete(ctx, bmcSecret)).To(Succeed())
				}

				bmc := &metalv1alpha1.BMC{}
				err = k8sClient.Get(ctx, typeNamespacedBMCName1, bmc)
				if err == nil {
					By("delete BMC 1")
					Expect(k8sClient.Delete(ctx, bmc)).To(Succeed())
				}
				err = k8sClient.Get(ctx, typeNamespacedBMCName2, bmc)
				if err == nil {
					By("delete BMC 2")
					Expect(k8sClient.Delete(ctx, bmc)).To(Succeed())
				}
			})

			It("should successfully reconcile", func() {
				// given
				netBoxMock := prepareNetboxMock()
				controllerReconciler := createIronCoreReconciler(k8sClient, netBoxMock, fileReaderMock)

				// when
				res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{})

				// then
				Expect(err).ToNot(HaveOccurred())
				Expect(res.RequeueAfter).To(Equal(0 * time.Second))

				expectBMCResources()

				Expect(netBoxMock.VirtualizationMock.(*mock.VirtualizationMock).GetClustersByNameRegionTypeCalls).To(Equal(1))
				Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).GetDevicesByClusterIDCalls).To(Equal(1))
				Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).GetRegionForDeviceCalls).To(Equal(1))
			})

			It("should successfully reconcile if cluster selection is only by name", func() {
				// given
				netBoxMock := prepareNetboxMock()
				netBoxMock.VirtualizationMock.(*mock.VirtualizationMock).GetClustersByNameRegionTypeFunc = func(name, region, clusterType string) ([]models.Cluster, error) {
					Expect(name).To(Equal("name1"))
					Expect(region).To(BeEmpty())
					Expect(clusterType).To(BeEmpty())

					return []models.Cluster{
						{
							ID:   1,
							Name: "cluster1",
						},
					}, nil
				}
				fileReaderMockWithNameOnly := &mock.FileReaderMock{
					FileContent: make(map[string]string),
					ReturnError: false,
				}
				fileReaderMockWithNameOnly.FileContent["/etc/config/config.json"] = `{
					"serverController": "ironcore",
					"ironCore": [{
						"name": "name1"
					}],
					"netboxUrl": "http://netbox"
				}`
				fileReaderMockWithNameOnly.FileContent["/etc/credentials/credentials.json"] = fileReaderMock.FileContent["/etc/credentials/credentials.json"]
				controllerReconciler := createIronCoreReconciler(k8sClient, netBoxMock, fileReaderMockWithNameOnly)

				// when
				res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{})

				// then
				Expect(err).ToNot(HaveOccurred())
				Expect(res.RequeueAfter).To(Equal(0 * time.Second))

				expectBMCResources()

				Expect(netBoxMock.VirtualizationMock.(*mock.VirtualizationMock).GetClustersByNameRegionTypeCalls).To(Equal(1))
				Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).GetDevicesByClusterIDCalls).To(Equal(1))
				Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).GetRegionForDeviceCalls).To(Equal(1))
			})

			It("should return an error if configuration reload fails", func() {
				// given
				netBoxMock := prepareNetboxMock()
				fileReaderMockToError := &mock.FileReaderMock{
					FileContent: make(map[string]string),
					ReturnError: true,
				}
				controllerReconciler := createIronCoreReconciler(k8sClient, netBoxMock, fileReaderMockToError)

				// when
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{})

				// then
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("unable to read config.json: error"))
			})

			It("should not reconcile if server controller is not set to ironcore", func() {
				// given
				netBoxMock := prepareNetboxMock()
				fileReaderMockWithNameOnly := &mock.FileReaderMock{
					FileContent: make(map[string]string),
					ReturnError: false,
				}
				fileReaderMockWithNameOnly.FileContent["/etc/config/config.json"] = `{
					"serverController": "metal3",
					"ironCore": [],
					"netboxUrl": "http://netbox"
				}`
				fileReaderMockWithNameOnly.FileContent["/etc/credentials/credentials.json"] = fileReaderMock.FileContent["/etc/credentials/credentials.json"]
				controllerReconciler := createIronCoreReconciler(k8sClient, netBoxMock, fileReaderMockWithNameOnly)

				// when
				res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{})

				// then
				Expect(err).ToNot(HaveOccurred())
				Expect(res.RequeueAfter).To(Equal(0 * time.Second))

				Expect(netBoxMock.VirtualizationMock.(*mock.VirtualizationMock).GetClustersByNameRegionTypeCalls).To(Equal(0))
				Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).GetDevicesByClusterIDCalls).To(Equal(0))
				Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).GetRegionForDeviceCalls).To(Equal(0))
			})

			It("should succeed if multiple clusters in configuration", func() {
				// given
				netBoxMock := prepareNetboxMock()
				netBoxMock.VirtualizationMock.(*mock.VirtualizationMock).GetClustersByNameRegionTypeFunc = func(name, region, clusterType string) ([]models.Cluster, error) {
					Expect(name).To(BeElementOf("name1", "name2"))
					Expect(region).To(Equal(""))
					Expect(clusterType).To(Equal(""))

					return []models.Cluster{
						{
							ID:   1,
							Name: "cluster1",
						},
					}, nil
				}

				fileReaderMockWithNameOnly := &mock.FileReaderMock{
					FileContent: make(map[string]string),
					ReturnError: false,
				}
				fileReaderMockWithNameOnly.FileContent["/etc/config/config.json"] = `{
					"serverController": "ironcore",
					"ironCore": [
						{
							"name": "name1"
						},
						{
							"name": "name2"
						}
					],
					"netboxUrl": "http://netbox"
				}`
				fileReaderMockWithNameOnly.FileContent["/etc/credentials/credentials.json"] = fileReaderMock.FileContent["/etc/credentials/credentials.json"]
				controllerReconciler := createIronCoreReconciler(k8sClient, netBoxMock, fileReaderMockWithNameOnly)

				// when
				res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{})

				// then
				Expect(err).ToNot(HaveOccurred())
				Expect(res.RequeueAfter).To(Equal(0 * time.Second))

				expectBMCResources()

				Expect(netBoxMock.VirtualizationMock.(*mock.VirtualizationMock).GetClustersByNameRegionTypeCalls).To(Equal(2))
				Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).GetDevicesByClusterIDCalls).To(Equal(2))
				Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).GetRegionForDeviceCalls).To(Equal(1))
			})

			It("should succeed if multiple clusters fetched from netbox", func() {
				// given
				netBoxMock := prepareNetboxMock()
				netBoxMock.VirtualizationMock.(*mock.VirtualizationMock).GetClustersByNameRegionTypeFunc = func(name, region, clusterType string) ([]models.Cluster, error) {
					Expect(name).To(Equal(""))
					Expect(region).To(Equal(""))
					Expect(clusterType).To(Equal("type1"))

					return []models.Cluster{
						{
							ID:   1,
							Name: "cluster1",
						},
						{
							ID:   2,
							Name: "cluster2",
						},
					}, nil
				}

				fileReaderMockWithNameOnly := &mock.FileReaderMock{
					FileContent: make(map[string]string),
					ReturnError: false,
				}
				fileReaderMockWithNameOnly.FileContent["/etc/config/config.json"] = `{
					"serverController": "ironcore",
					"ironCore": [
						{
							"type": "type1"
						}
					],
					"netboxUrl": "http://netbox"
				}`
				fileReaderMockWithNameOnly.FileContent["/etc/credentials/credentials.json"] = fileReaderMock.FileContent["/etc/credentials/credentials.json"]
				controllerReconciler := createIronCoreReconciler(k8sClient, netBoxMock, fileReaderMockWithNameOnly)

				// when
				res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{})

				// then
				Expect(err).ToNot(HaveOccurred())
				Expect(res.RequeueAfter).To(Equal(0 * time.Second))

				expectBMCResources()

				Expect(netBoxMock.VirtualizationMock.(*mock.VirtualizationMock).GetClustersByNameRegionTypeCalls).To(Equal(1))
				Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).GetDevicesByClusterIDCalls).To(Equal(2))
				Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).GetRegionForDeviceCalls).To(Equal(1))
			})

			It("should succeed if multiple devices fetched from netbox", func() {
				// given
				netBoxMock := prepareNetboxMock()
				netBoxMock.VirtualizationMock.(*mock.VirtualizationMock).GetClustersByNameRegionTypeFunc = func(name, region, clusterType string) ([]models.Cluster, error) {
					Expect(name).To(Equal("cluster1"))
					Expect(region).To(Equal(""))
					Expect(clusterType).To(Equal(""))

					return []models.Cluster{
						{
							ID:   1,
							Name: "cluster1",
						},
					}, nil
				}
				netBoxMock.DCIMMock.(*mock.DCIMMock).GetDevicesByClusterIDFunc = func(clusterID int) ([]models.Device, error) {
					Expect(clusterID).To(BeElementOf(1))
					devices := []models.Device{}
					for _, deviceName := range []string{"device-name1", "device-name2"} {
						devices = append(devices, models.Device{
							ID:     1,
							Name:   deviceName,
							Status: models.DeviceStatus{Value: "active"},
							Platform: models.NestedPlatform{
								ID:   1,
								Slug: "platform1",
							},
							OOBIp: models.NestedIPAddress{
								ID:      1,
								Address: "192.168.1.1/24",
							},
							Site: models.NestedSite{
								Slug: "site1",
							},
							DeviceType: models.NestedDeviceType{
								Slug: "type1",
							},
							DeviceRole: models.NestedDeviceRole{
								Slug: "role1",
							},
						})
					}
					return devices, nil
				}

				netBoxMock.DCIMMock.(*mock.DCIMMock).GetRegionForDeviceFunc = func(device *models.Device) (string, error) {
					Expect(device.Name).To(BeElementOf("device-name1", "device-name2"))
					return "region1", nil
				}

				fileReaderMockWithNameOnly := &mock.FileReaderMock{
					FileContent: make(map[string]string),
					ReturnError: false,
				}
				fileReaderMockWithNameOnly.FileContent["/etc/config/config.json"] = `{
					"serverController": "ironcore",
					"ironCore": [
						{
							"name": "cluster1"
						}
					],
					"netboxUrl": "http://netbox"
				}`
				fileReaderMockWithNameOnly.FileContent["/etc/credentials/credentials.json"] = fileReaderMock.FileContent["/etc/credentials/credentials.json"]
				controllerReconciler := createIronCoreReconciler(k8sClient, netBoxMock, fileReaderMockWithNameOnly)

				// when
				res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{})

				// then
				Expect(err).ToNot(HaveOccurred())
				Expect(res.RequeueAfter).To(Equal(0 * time.Second))

				bmcSecretList := &metalv1alpha1.BMCSecretList{}
				err = k8sClient.List(ctx, bmcSecretList)
				Expect(err).ToNot(HaveOccurred())
				Expect(bmcSecretList.Items).To(HaveLen(2))

				bmcSecret1 := &metalv1alpha1.BMCSecret{}
				err = k8sClient.Get(ctx, typeNamespacedBMCName1, bmcSecret1)
				Expect(err).ToNot(HaveOccurred())
				expectBMCSecret(bmcSecret1, bmcName1)

				bmcSecret2 := &metalv1alpha1.BMCSecret{}
				err = k8sClient.Get(ctx, typeNamespacedBMCName2, bmcSecret2)
				Expect(err).ToNot(HaveOccurred())
				expectBMCSecret(bmcSecret2, bmcName2)

				bmcList := &metalv1alpha1.BMCList{}
				err = k8sClient.List(ctx, bmcList)
				Expect(err).ToNot(HaveOccurred())
				Expect(bmcList.Items).To(HaveLen(2))

				bmc1 := &metalv1alpha1.BMC{}
				err = k8sClient.Get(ctx, typeNamespacedBMCName1, bmc1)
				Expect(err).ToNot(HaveOccurred())
				expectBMC(bmc1, bmcName1)

				bmc2 := &metalv1alpha1.BMC{}
				err = k8sClient.Get(ctx, typeNamespacedBMCName2, bmc2)
				Expect(err).ToNot(HaveOccurred())
				expectBMC(bmc2, bmcName2)

				Expect(netBoxMock.VirtualizationMock.(*mock.VirtualizationMock).GetClustersByNameRegionTypeCalls).To(Equal(1))
				Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).GetDevicesByClusterIDCalls).To(Equal(1))
				Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).GetRegionForDeviceCalls).To(Equal(2))
			})

			It("should return an error if netbox reload fails", func() {
				// given
				controllerReconciler := createIronCoreReconciler(k8sClient, &mock.NetBoxMock{ReturnError: true}, fileReaderMock)

				// when
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{})

				// then
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("unable to reload netbox"))
			})

			It("should return an error if GetClustersByNameRegionType fails", func() {
				// given
				netBoxMock := prepareNetboxMock()
				netBoxMock.VirtualizationMock.(*mock.VirtualizationMock).GetClustersByNameRegionTypeFunc = func(name, region, clusterType string) ([]models.Cluster, error) {
					Expect(name).To(Equal("name1"))
					Expect(region).To(Equal("region1"))
					Expect(clusterType).To(Equal("type1"))

					return nil, errors.New("unable to find clusters")
				}

				controllerReconciler := createIronCoreReconciler(k8sClient, netBoxMock, fileReaderMock)

				// when
				res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{})

				// then
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("unable to find clusters"))
				Expect(res.RequeueAfter).To(Equal(0 * time.Second))
			})

			It("should return an error if GetDevicesByClusterID fails", func() {
				// given
				netBoxMock := prepareNetboxMock()
				netBoxMock.VirtualizationMock.(*mock.VirtualizationMock).GetClustersByNameRegionTypeFunc = func(name, region, clusterType string) ([]models.Cluster, error) {
					Expect(name).To(Equal("name1"))
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

				controllerReconciler := createIronCoreReconciler(k8sClient, netBoxMock, fileReaderMock)

				// when
				res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{})

				// then
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("unable to find devices"))
				Expect(res.RequeueAfter).To(Equal(0 * time.Second))
			})

			It("should skip the device when the device is not active", func() {
				// given
				netBoxMock := prepareNetboxMock()
				netBoxMock.VirtualizationMock.(*mock.VirtualizationMock).GetClustersByNameRegionTypeFunc = func(name, region, clusterType string) ([]models.Cluster, error) {
					return []models.Cluster{
						{
							ID:   1,
							Name: "cluster1",
						},
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

				controllerReconciler := createIronCoreReconciler(k8sClient, netBoxMock, fileReaderMock)

				// when
				res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{})

				// then
				Expect(err).ToNot(HaveOccurred())
				Expect(res.RequeueAfter).To(Equal(0 * time.Second))
			})

			It("should return an error if GetRegionForDevice fails", func() {
				// given
				netBoxMock := prepareNetboxMock()
				netBoxMock.DCIMMock.(*mock.DCIMMock).GetRegionForDeviceFunc = func(device *models.Device) (string, error) {
					return "", errors.New("unable to get region for device")
				}

				controllerReconciler := createIronCoreReconciler(k8sClient, netBoxMock, fileReaderMock)

				// when
				res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{})

				// then
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("unable to get region for device: unable to get region for device"))
				Expect(res.RequeueAfter).To(Equal(0 * time.Second))
			})

			It("should skip the device when BMC custom resource already exists", func() {
				// given
				netBoxMock := prepareNetboxMock()
				controllerReconciler := createIronCoreReconciler(k8sClient, netBoxMock, fileReaderMock)

				// create existing BMC
				bmc := &metalv1alpha1.BMC{
					ObjectMeta: ctrl.ObjectMeta{
						Name:      "device-name1",
						Namespace: "default",
					},
					Spec: metalv1alpha1.BMCSpec{
						Endpoint: &metalv1alpha1.InlineEndpoint{
							IP: metalv1alpha1.MustParseIP("192.168.1.1"),
						},
					},
				}
				err := k8sClient.Create(ctx, bmc)
				Expect(err).ToNot(HaveOccurred())

				// when
				res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{})

				// then
				Expect(err).ToNot(HaveOccurred())
				Expect(res.RequeueAfter).To(Equal(0 * time.Second))
			})
		})

		Context("Fake Client", func() {
			It("should return an error if createBmcSecret fails", func() {
				// given
				netBoxMock := prepareNetboxMock()

				fakeClient := createFakeClient()
				failClient := &shouldFailClient{fakeClient, "BMCSecret"}

				controllerReconciler := createIronCoreReconciler(failClient, netBoxMock, fileReaderMock)

				// when
				res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{})

				// then
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("unable to create bmc secret: intentionally failing client on client.Create for BMCSecret"))
				Expect(res.RequeueAfter).To(Equal(0 * time.Second))
			})

			It("should return an error if createBmc fails", func() {
				// given
				netBoxMock := prepareNetboxMock()

				fakeClient := createFakeClient()
				failClient := &shouldFailClient{fakeClient, "BMC"}

				controllerReconciler := createIronCoreReconciler(failClient, netBoxMock, fileReaderMock)

				// when
				res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{})

				// then
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("unable to create bmc: unable to create BMC: intentionally failing client on client.Create for BMC"))
				Expect(res.RequeueAfter).To(Equal(0 * time.Second))
			})
		})
	})
})

func createIronCoreReconciler(k8sClient client.Client, netBoxMock *mock.NetBoxMock, fileReaderMock config.FileReader) *IronCoreReconciler {
	return &IronCoreReconciler{
		k8sClient:         k8sClient,
		scheme:            k8sClient.Scheme(),
		cfg:               config.NewDefaultConfiguration(k8sClient, fileReaderMock),
		netBox:            netBoxMock,
		reconcileInterval: reconcileInterval,
		eventChannel:      make(chan event.GenericEvent),
	}
}

type shouldFailClient struct {
	client.Client
	FailOnCreateKind string
}

func (p *shouldFailClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if obj.GetObjectKind().GroupVersionKind().Kind == p.FailOnCreateKind {
		return errors.New("intentionally failing client on client.Create for " + p.FailOnCreateKind)
	}
	return p.Client.Create(ctx, obj, opts...)
}
