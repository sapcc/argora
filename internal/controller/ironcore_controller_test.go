// Copyright 2025 SAP SE
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"

	metalv1alpha1 "github.com/ironcore-dev/metal-operator/api/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
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
		bmcName           = "device-name1"
	)

	fileReaderMock := &mock.FileReaderMock{
		FileContent: make(map[string]string),
		ReturnError: false,
	}
	fileReaderMock.FileContent["/etc/config/config.json"] = `{
		"serverController": "ironcore",
		"ironCoreTypes": "type1",
		"ironCoreRegion": "region1",
		"netboxUrl": "http://netbox"
	}`
	fileReaderMock.FileContent["/etc/credentials/credentials.json"] = `{
		"bmcUser": "user",
		"bmcPassword": "password",
		"netboxToken": "token"
	}`

	Context("when reconciling", func() {
		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: resourceNamespace,
		}

		typeNamespacedBMCName := types.NamespacedName{
			Name:      bmcName,
			Namespace: resourceNamespace,
		}

		prepareMockSuccess := func() *mock.NetBoxMock {
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

		expectLabels := func(labels map[string]string) {
			Expect(labels).To(HaveLen(7))
			Expect(labels).To(HaveKeyWithValue("topology.kubernetes.io/region", "region1"))
			Expect(labels).To(HaveKeyWithValue("topology.kubernetes.io/zone", "site1"))
			Expect(labels).To(HaveKeyWithValue("kubernetes.metal.cloud.sap/cluster", "cluster1"))
			Expect(labels).To(HaveKeyWithValue("kubernetes.metal.cloud.sap/name", "device-name1"))
			Expect(labels).To(HaveKeyWithValue("kubernetes.metal.cloud.sap/bb", "name1"))
			Expect(labels).To(HaveKeyWithValue("kubernetes.metal.cloud.sap/type", "type1"))
			Expect(labels).To(HaveKeyWithValue("kubernetes.metal.cloud.sap/role", "role1"))
		}

		expectBMCSecret := func() {
			bmcSecret := &metalv1alpha1.BMCSecret{}
			err := k8sClient.Get(ctx, typeNamespacedBMCName, bmcSecret)
			Expect(err).NotTo(HaveOccurred())

			Expect(bmcSecret.Name).To(Equal("device-name1"))
			expectLabels(bmcSecret.Labels)

			Expect(bmcSecret.ObjectMeta.OwnerReferences).To(HaveLen(1))
			Expect(bmcSecret.ObjectMeta.OwnerReferences[0].APIVersion).To(Equal("metal.ironcore.dev/v1alpha1"))
			Expect(bmcSecret.ObjectMeta.OwnerReferences[0].Kind).To(Equal("BMC"))
			Expect(bmcSecret.ObjectMeta.OwnerReferences[0].Name).To(Equal("device-name1"))
			Expect(*bmcSecret.ObjectMeta.OwnerReferences[0].Controller).To(BeTrue())
			Expect(*bmcSecret.ObjectMeta.OwnerReferences[0].BlockOwnerDeletion).To(BeTrue())

			Expect(bmcSecret.Data).To(Equal(map[string][]byte{
				metalv1alpha1.BMCSecretUsernameKeyName: []byte("user"),
				metalv1alpha1.BMCSecretPasswordKeyName: []byte("password"),
			}))
		}

		expectBMC := func() {
			bmc := &metalv1alpha1.BMC{}
			err := k8sClient.Get(ctx, typeNamespacedBMCName, bmc)
			Expect(err).NotTo(HaveOccurred())

			Expect(bmc.Name).To(Equal("device-name1"))
			expectLabels(bmc.Labels)
			Expect(bmc.ObjectMeta.OwnerReferences).To(BeEmpty())

			Expect(bmc.Spec.Endpoint.IP.String()).To(Equal("192.168.1.1"))
			Expect(bmc.Spec.Protocol.Name).To(Equal(metalv1alpha1.ProtocolNameRedfish))
			Expect(bmc.Spec.Protocol.Port).To(Equal(int32(443)))
			Expect(bmc.Spec.BMCSecretRef.Name).To(Equal("device-name1"))
		}

		cleanUp := func() {
			bmcSecret := &metalv1alpha1.BMCSecret{}
			err := k8sClient.Get(ctx, typeNamespacedBMCName, bmcSecret)
			if err == nil {
				By("delete BMC Secret")
				Expect(k8sClient.Delete(ctx, bmcSecret)).To(Succeed())
			}

			bmc := &metalv1alpha1.BMC{}
			err = k8sClient.Get(ctx, typeNamespacedBMCName, bmc)
			if err == nil {
				By("delete BMC")
				Expect(k8sClient.Delete(ctx, bmc)).To(Succeed())
			}
		}

		BeforeEach(func() {
			cleanUp()
		})

		AfterEach(func() {
			cleanUp()
		})

		It("should successfully reconcile", func() {
			// given
			netBoxMock := prepareMockSuccess()
			controllerReconciler := createIronCoreReconciler(netBoxMock, fileReaderMock)

			// when
			res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})

			// then
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Requeue).To(BeFalse())

			expectBMCSecret()
			expectBMC()

			netBoxMock.VirtualizationMock.(*mock.VirtualizationMock).GetClusterByNameRegionTypeCalls = 1
			netBoxMock.DCIMMock.(*mock.DCIMMock).GetDevicesByClusterIDCalls = 1
			netBoxMock.DCIMMock.(*mock.DCIMMock).GetRegionForDeviceCalls = 1
		})

		It("should return an error if netbox reload fails", func() {
			// given
			controllerReconciler := createIronCoreReconciler(&mock.NetBoxMock{ReturnError: true}, fileReaderMock)

			// when
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})

			// then
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unable to reload netbox"))
		})

		It("should return an error if GetClusterByNameRegionType fails", func() {
			// given
			netBoxMock := prepareMockSuccess()
			netBoxMock.VirtualizationMock.(*mock.VirtualizationMock).GetClusterByNameRegionTypeFunc = func(name, region, clusterType string) (*models.Cluster, error) {
				Expect(name).To(BeEmpty())
				Expect(region).To(Equal("region1"))
				Expect(clusterType).To(Equal("type1"))

				return nil, errors.New("unable to find clusters")
			}

			controllerReconciler := createIronCoreReconciler(netBoxMock, fileReaderMock)

			// when
			res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})

			// then
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unable to find clusters"))
			Expect(res.Requeue).To(BeFalse())
		})

		It("should return an error if GetDevicesByClusterID fails", func() {
			// given
			netBoxMock := prepareMockSuccess()
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

			controllerReconciler := createIronCoreReconciler(netBoxMock, fileReaderMock)

			// when
			res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})

			// then
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unable to find devices"))
			Expect(res.Requeue).To(BeFalse())
		})

		It("should skip the device when the device is not active", func() {
			// given
			netBoxMock := prepareMockSuccess()
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

			controllerReconciler := createIronCoreReconciler(netBoxMock, fileReaderMock)

			// when
			res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})

			// then
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Requeue).To(BeFalse())
		})

		It("should return an error if GetRegionForDevice fails", func() {
			// given
			netBoxMock := prepareMockSuccess()
			netBoxMock.DCIMMock.(*mock.DCIMMock).GetRegionForDeviceFunc = func(device *models.Device) (string, error) {
				return "", errors.New("unable to get region for device")
			}

			controllerReconciler := createIronCoreReconciler(netBoxMock, fileReaderMock)

			// when
			res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})

			// then
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unable to get region for device: unable to get region for device"))
			Expect(res.Requeue).To(BeFalse())
		})

		// It("should return an error if createBmcSecret fails", func() {
		// 	// given
		// 	netBoxMock := prepareMockSuccess()
		// 	controllerReconciler := createIronCoreReconciler(netBoxMock, fileReaderMock)

		// 	// when
		// 	res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
		// 		NamespacedName: typeNamespacedName,
		// 	})

		// 	// then
		// 	Expect(err).To(HaveOccurred())
		// 	Expect(err).To(MatchError("unable to create bmc secret: bmc user or password not set"))
		// 	Expect(res.Requeue).To(BeFalse())
		// })

		// It("should return an error if createBmc fails", func() {
		// 	// given
		// 	netBoxMock := prepareMockSuccess()
		// 	controllerReconciler := createIronCoreReconciler(netBoxMock, fileReaderMock)

		// 	// when
		// 	res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
		// 		NamespacedName: typeNamespacedName,
		// 	})

		// 	// then
		// 	Expect(err).To(HaveOccurred())
		// 	Expect(err).To(MatchError("unable to create bmc: unable to parse OOB IP: invalid CIDR address: 192.168.1.1/24"))
		// 	Expect(res.Requeue).To(BeFalse())
		// })

		It("should skip the device when BMC custom resource already exists", func() {
			// given
			netBoxMock := prepareMockSuccess()
			controllerReconciler := createIronCoreReconciler(netBoxMock, fileReaderMock)

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
			Expect(err).NotTo(HaveOccurred())

			// when
			res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})

			// then
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Requeue).To(BeFalse())
		})
	})
})

func createIronCoreReconciler(netBoxMock *mock.NetBoxMock, fileReaderMock config.FileReader) *IronCoreReconciler {
	return &IronCoreReconciler{
		k8sClient:         k8sClient,
		scheme:            k8sClient.Scheme(),
		cfg:               config.NewDefaultConfiguration(k8sClient, fileReaderMock),
		netBox:            netBoxMock,
		reconcileInterval: reconcileInterval,
		eventChannel:      make(chan event.GenericEvent),
	}
}
