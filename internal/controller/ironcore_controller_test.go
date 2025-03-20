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
		bmcName           = "device-name1"
	)

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

		typeNamespacedBMCName := types.NamespacedName{
			Name:      bmcName,
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
				Expect(clusterType).To(BeElementOf("type1", "type2"))

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
			Expect(labels).To(Equal(map[string]string{
				"topology.kubernetes.io/region":      "region1",
				"topology.kubernetes.io/zone":        "site1",
				"kubernetes.metal.cloud.sap/cluster": "cluster1",
				"kubernetes.metal.cloud.sap/name":    "device-name1",
				"kubernetes.metal.cloud.sap/bb":      "name1",
				"kubernetes.metal.cloud.sap/type":    "type1",
				"kubernetes.metal.cloud.sap/role":    "role1",
			}))
		}

		expectBMCSecret := func(bmcSecret *metalv1alpha1.BMCSecret) {
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

		expectBMC := func(bmc *metalv1alpha1.BMC) {
			Expect(bmc.Name).To(Equal("device-name1"))
			expectLabels(bmc.Labels)
			Expect(bmc.ObjectMeta.OwnerReferences).To(BeEmpty())

			Expect(bmc.Spec.Endpoint.IP.String()).To(Equal("192.168.1.1"))
			Expect(bmc.Spec.Protocol.Name).To(Equal(metalv1alpha1.ProtocolNameRedfish))
			Expect(bmc.Spec.Protocol.Port).To(Equal(int32(443)))
			Expect(bmc.Spec.BMCSecretRef.Name).To(Equal("device-name1"))
		}

		Context("Envtest", func() {
			AfterEach(func() {
				By("cleanup")
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
			})

			It("should successfully reconcile", func() {
				// given
				netBoxMock := prepareNetboxMock()
				controllerReconciler := createIronCoreReconciler(k8sClient, netBoxMock, fileReaderMock)

				// when
				res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{})

				// then
				Expect(err).NotTo(HaveOccurred())
				Expect(res.Requeue).To(BeFalse())

				bmcSecret := &metalv1alpha1.BMCSecret{}
				err = k8sClient.Get(ctx, typeNamespacedBMCName, bmcSecret)
				Expect(err).NotTo(HaveOccurred())
				expectBMCSecret(bmcSecret)

				bmc := &metalv1alpha1.BMC{}
				err = k8sClient.Get(ctx, typeNamespacedBMCName, bmc)
				Expect(err).NotTo(HaveOccurred())
				expectBMC(bmc)

				netBoxMock.VirtualizationMock.(*mock.VirtualizationMock).GetClustersByNameRegionTypeCalls = 2
				netBoxMock.DCIMMock.(*mock.DCIMMock).GetDevicesByClusterIDCalls = 2
				netBoxMock.DCIMMock.(*mock.DCIMMock).GetRegionForDeviceCalls = 2
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
				Expect(res.Requeue).To(BeFalse())
			})

			It("should return an error if GetClustersByNameRegionType returns multiple clusters", func() {
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
						{
							ID:   2,
							Name: "cluster2",
						},
					}, nil
				}

				controllerReconciler := createIronCoreReconciler(k8sClient, netBoxMock, fileReaderMock)

				// when
				res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{})

				// then
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("multiple clusters found"))
				Expect(res.Requeue).To(BeFalse())
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
				Expect(res.Requeue).To(BeFalse())
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
				Expect(err).NotTo(HaveOccurred())
				Expect(res.Requeue).To(BeFalse())
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
				Expect(res.Requeue).To(BeFalse())
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
				Expect(err).NotTo(HaveOccurred())

				// when
				res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{})

				// then
				Expect(err).NotTo(HaveOccurred())
				Expect(res.Requeue).To(BeFalse())
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
				Expect(res.Requeue).To(BeFalse())
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
				Expect(res.Requeue).To(BeFalse())
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
