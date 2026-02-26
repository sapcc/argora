// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"strconv"

	metalv1alpha1 "github.com/ironcore-dev/metal-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sapcc/go-netbox-go/models"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ipamv1 "sigs.k8s.io/cluster-api/api/ipam/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/sapcc/argora/internal/controller/mock"
	"github.com/sapcc/argora/internal/credentials"
	"github.com/sapcc/argora/internal/netbox/ipam"
)

const (
	ipAddressString       = "192.168.1.100"
	ipAddressMask   int32 = 24
	ipAddressID           = 456
	fullIPAddress         = "192.168.1.100/24"
	interfaceID           = 123
	deviceID              = 321
)

var _ = Describe("IP Update Controller", func() {
	const resourceName = "test-resource"
	const resourceNamespace = "default"

	fileReaderMock := &mock.FileReaderMock{
		FileContent: make(map[string]string),
		ReturnError: false,
	}

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

		ipAddress := &ipamv1.IPAddress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: resourceNamespace,
			},
		}

		prepareNetboxMock := func() *mock.NetBoxMock {
			netBoxMock := &mock.NetBoxMock{
				ReturnError:        false,
				VirtualizationMock: &mock.VirtualizationMock{},
				DCIMMock: &mock.DCIMMock{
					GetDeviceByNameFunc: func(_ string) (*models.Device, error) {
						return &models.Device{
							ID:        deviceID,
							Name:      "node001-rack01",
							PrimaryIP: models.NestedIPAddress{ID: ipAddressID},
						}, nil
					},
					GetInterfacesForDeviceFunc: func(_ *models.Device) ([]models.Interface, error) {
						return []models.Interface{
							{
								Name: "LAG0",
								Type: models.InterfaceType{Value: "lag"},
							},
							{
								NestedInterface: models.NestedInterface{
									ID: interfaceID,
								},
								Name: "LAG1",
								Type: models.InterfaceType{Value: "lag"},
							},
							{
								Name: "eth0",
								Type: models.InterfaceType{Value: "1000base-t"},
							},
						}, nil
					},
				},
				IPAMMock: &mock.IPAMMock{
					GetIPAddressByAddressFunc: func(_ string) (*models.IPAddress, error) {
						return &models.IPAddress{
							NestedIPAddress: models.NestedIPAddress{
								ID:      ipAddressID,
								Address: fullIPAddress,
							},
							AssignedObjectID: interfaceID,
							AssignedInterface: models.NestedInterface{
								ID: interfaceID,
								Device: models.NestedDevice{
									ID: deviceID,
								},
							},
						}, nil
					},
				},

				ExtrasMock: &mock.ExtrasMock{},
			}

			return netBoxMock
		}

		BeforeEach(func() {
			By("create Server CR")
			server := &metalv1alpha1.Server{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-server",
					Namespace: resourceNamespace,
				},
			}

			server.Spec = metalv1alpha1.ServerSpec{
				SystemUUID: "test_uuid",
				BMCRef: &v1.LocalObjectReference{
					Name: "node001-rack01",
				},
			}

			Expect(k8sClient.Create(ctx, server)).To(Succeed())

			By("create Server Claim CR")
			serverClaim := &metalv1alpha1.ServerClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-server-claim",
					Namespace: resourceNamespace,
				},
			}

			serverClaim.Spec = metalv1alpha1.ServerClaimSpec{
				ServerRef: &v1.LocalObjectReference{
					Name: "test-server",
				},
				Image: "gardenlinux:latest",
				Power: "On",
			}

			Expect(k8sClient.Create(ctx, serverClaim)).To(Succeed())

			By("create IPAM IPAddressClaim CR")
			ipClaim := &ipamv1.IPAddressClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-claim",
					Namespace: resourceNamespace,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "metal.ironcore.dev/v1alpha1",
							Kind:       "ServerClaim",
							Name:       "test-server-claim",
							UID:        "fake-uid",
						},
					},
				},
				Spec: ipamv1.IPAddressClaimSpec{
					PoolRef: ipamv1.IPPoolReference{
						Name:     "test-pool",
						Kind:     "GlobalInClusterIPPool",
						APIGroup: "ipam.cluster.x-k8s.io",
					},
				},
			}

			Expect(k8sClient.Create(ctx, ipClaim)).To(Succeed())

			By("create IPAM IPAddress CR")
			ipAddress = &ipamv1.IPAddress{
				ObjectMeta: metav1.ObjectMeta{
					Name:       resourceName,
					Namespace:  resourceNamespace,
					Finalizers: []string{"ipupdate.argora.cloud.sap.com/finalizer"},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "ipam.cluster.x-k8s.io/v1beta2",
							Kind:               "IPAddressClaim",
							Name:               "test-claim",
							UID:                ipClaim.UID,
							Controller:         ptr.To(true),
							BlockOwnerDeletion: ptr.To(true),
						},
					},
				},
				Spec: ipamv1.IPAddressSpec{
					Address: ipAddressString,
					PoolRef: ipamv1.IPPoolReference{
						Name:     "test-ippool",
						APIGroup: "ipam.cluster.x-k8s.io",
						Kind:     "IPPool",
					},
					ClaimRef: ipamv1.IPAddressClaimReference{
						Name: "test-claim",
					},
					Prefix: ptr.To(ipAddressMask),
				},
			}

			Expect(k8sClient.Create(ctx, ipAddress)).To(Succeed())
		})

		AfterEach(func() {
			By("cleanup IPAM IPAddress CR")
			ipAddress := &ipamv1.IPAddress{}
			if err := k8sClient.Get(ctx, typeNamespacedUpdateName, ipAddress); err == nil {
				if len(ipAddress.Finalizers) > 0 {
					ipAddress.Finalizers = nil
					Expect(k8sClient.Update(ctx, ipAddress)).To(Succeed())
				}
				k8sClient.Delete(ctx, ipAddress)
			}

			By("cleanup IPAM IPAddressClaim CR")
			ipClaim := &ipamv1.IPAddressClaim{}
			if err := k8sClient.Get(ctx, types.NamespacedName{
				Name: "test-claim", Namespace: resourceNamespace,
			}, ipClaim); err == nil {
				Expect(k8sClient.Delete(ctx, ipClaim)).To(Succeed())
			}

			By("cleanup ServerClaim")
			serverClaim := &metalv1alpha1.ServerClaim{}
			if err := k8sClient.Get(ctx, types.NamespacedName{
				Name: "test-server-claim", Namespace: resourceNamespace,
			}, serverClaim); err == nil {
				Expect(k8sClient.Delete(ctx, serverClaim)).To(Succeed())
			}

			By("cleanup Server")
			server := &metalv1alpha1.Server{}
			if err := k8sClient.Get(ctx, types.NamespacedName{
				Name: "test-server", Namespace: resourceNamespace,
			}, server); err == nil {
				Expect(k8sClient.Delete(ctx, server)).To(Succeed())
			}
		})

		It("should successfully add finalizer", func() {
			// given
			netBoxMock := prepareNetboxMock()
			controllerReconciler := createIPUpdateReconciler(netBoxMock, fileReaderMock)

			ipAddress := &ipamv1.IPAddress{}
			Expect(k8sClient.Get(ctx, typeNamespacedUpdateName, ipAddress)).To(Succeed())

			if controllerutil.RemoveFinalizer(ipAddress, "ipupdate.argora.cloud.sap.com/finalizer") {
				Expect(k8sClient.Update(ctx, ipAddress)).To(Succeed())
			}

			// when
			By("reconciling IPAM IPAddress CR")
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedUpdateName})

			// then
			By("checking if finalizer is added to IPAddress")
			Expect(err).ToNot(HaveOccurred())
			Expect(k8sClient.Get(ctx, typeNamespacedUpdateName, ipAddress)).To(Succeed())
			Expect(ipAddress.Finalizers).To(ContainElement("ipupdate.argora.cloud.sap.com/finalizer"))
		})

		It("should successfully reconcile", func() {
			// given
			netBoxMock := prepareNetboxMock()
			controllerReconciler := createIPUpdateReconciler(netBoxMock, fileReaderMock)

			Expect(k8sClient.Get(ctx, typeNamespacedUpdateName, ipAddress)).To(Succeed())
			// when
			By("reconciling IPAM IPAddress CR")
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedUpdateName})

			// then
			Expect(err).ToNot(HaveOccurred())
			ipAddress := &ipamv1.IPAddress{}
			Expect(k8sClient.Get(ctx, typeNamespacedUpdateName, ipAddress)).To(Succeed())

			By("verifying netbox metadata annotations")
			Expect(ipAddress.Annotations).To(HaveKeyWithValue("netbox.argora.cloud.sap/device-id", "321"))
			Expect(ipAddress.Annotations).To(HaveKeyWithValue("netbox.argora.cloud.sap/interface-id", "123"))
		})

		It("fails when IPAddressClaim does not exist", func() {
			netBoxMock := prepareNetboxMock()
			controllerReconciler := createIPUpdateReconciler(netBoxMock, fileReaderMock)

			ipClaim := &ipamv1.IPAddressClaim{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name: "test-claim", Namespace: resourceNamespace,
			}, ipClaim)).To(Succeed())
			Expect(k8sClient.Delete(ctx, ipClaim)).To(Succeed())

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedUpdateName,
			})

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to get IPAddressClaim for IPAddress:"))
		})

		It("fails when IPAddressClaim has no ServerClaim owner", func() {
			netBoxMock := prepareNetboxMock()
			controllerReconciler := createIPUpdateReconciler(netBoxMock, fileReaderMock)

			ipClaim := &ipamv1.IPAddressClaim{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name: "test-claim", Namespace: resourceNamespace,
			}, ipClaim)).To(Succeed())

			ipClaim.OwnerReferences = nil
			Expect(k8sClient.Update(ctx, ipClaim)).To(Succeed())

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedUpdateName,
			})

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no ServerClaim owner"))
		})

		It("fails when ServerClaim is not yet bound to a Server", func() {
			netBoxMock := prepareNetboxMock()
			controllerReconciler := createIPUpdateReconciler(netBoxMock, fileReaderMock)

			By("recreating ServerClaim without ServerRef")
			serverClaim := &metalv1alpha1.ServerClaim{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      "test-server-claim",
				Namespace: resourceNamespace,
			}, serverClaim)).To(Succeed())

			Expect(k8sClient.Delete(ctx, serverClaim)).To(Succeed())

			serverClaim = &metalv1alpha1.ServerClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-server-claim",
					Namespace: resourceNamespace,
				},
				Spec: metalv1alpha1.ServerClaimSpec{
					Image: "gardenlinux:latest",
					Power: "On",
				},
			}
			Expect(k8sClient.Create(ctx, serverClaim)).To(Succeed())
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedUpdateName,
			})
			Expect(err).To(HaveOccurred())

			Expect(err.Error()).To(ContainSubstring("not yet bound to a server"))
		})

		It("fails when referenced Server does not exist", func() {
			netBoxMock := prepareNetboxMock()
			controllerReconciler := createIPUpdateReconciler(netBoxMock, fileReaderMock)

			server := &metalv1alpha1.Server{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      "test-server",
				Namespace: resourceNamespace,
			}, server)).To(Succeed())
			Expect(k8sClient.Delete(ctx, server)).To(Succeed())
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedUpdateName,
			})

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(`servers.metal.ironcore.dev "test-server" not found`))
		})

		It("fails when Server has no BMCRef name", func() {
			netBoxMock := prepareNetboxMock()
			controllerReconciler := createIPUpdateReconciler(netBoxMock, fileReaderMock)

			server := &metalv1alpha1.Server{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      "test-server",
				Namespace: resourceNamespace,
			}, server)).To(Succeed())

			server.Spec.BMCRef.Name = ""
			Expect(k8sClient.Update(ctx, server)).To(Succeed())
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedUpdateName,
			})

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("has no bmcRef name"))
		})

		It("fails when referenced ServerClaim does not exist", func() {
			netBoxMock := prepareNetboxMock()
			controllerReconciler := createIPUpdateReconciler(netBoxMock, fileReaderMock)

			serverClaim := &metalv1alpha1.ServerClaim{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      "test-server-claim",
				Namespace: resourceNamespace,
			}, serverClaim)).To(Succeed())
			Expect(k8sClient.Delete(ctx, serverClaim)).To(Succeed())

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedUpdateName,
			})

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to get ServerClaim"))
		})

		It("create IP address if address don't exist", func() {
			netBoxMock := prepareNetboxMock()
			netBoxMock.IPAMMock = &mock.IPAMMock{
				GetIPAddressByAddressFunc: func(_ string) (*models.IPAddress, error) {
					return nil, ipam.ErrNoObjectsFound
				},
				CreateIPAddressFunc: func(_ ipam.CreateIPAddressParams) (*models.IPAddress, error) {
					return &models.IPAddress{
						NestedIPAddress:   models.NestedIPAddress{ID: ipAddressID},
						AssignedInterface: models.NestedInterface{ID: interfaceID, Device: models.NestedDevice{ID: deviceID}},
					}, nil
				},
				GetPrefixesByPrefixesFunc: func(_ string) ([]models.Prefix, error) {
					return nil, nil
				},
			}

			controllerRecociler := createIPUpdateReconciler(netBoxMock, fileReaderMock)

			_, err := controllerRecociler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedUpdateName})

			Expect(err).To(Succeed())
			Expect(netBoxMock.IPAMMock.(*mock.IPAMMock).GetIPAddressByAddressCalls).To(Equal(1))
			Expect(netBoxMock.IPAMMock.(*mock.IPAMMock).CreateIPAddressCalls).To(Equal(1))
		})

		It("do nothing if there is matching ip", func() {
			netBoxMock := prepareNetboxMock()

			controllerRecociler := createIPUpdateReconciler(netBoxMock, fileReaderMock)

			_, err := controllerRecociler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedUpdateName})

			Expect(netBoxMock.IPAMMock.(*mock.IPAMMock).GetIPAddressByAddressCalls).To(Equal(1))

			Expect(err).To(Succeed())
		})

		It("raise error if unable to create IP address", func() {
			netBoxMock := prepareNetboxMock()
			netBoxMock.IPAMMock = &mock.IPAMMock{
				GetPrefixesByPrefixesFunc: func(_ string) ([]models.Prefix, error) {
					return nil, errors.New("netbox error")
				},
				GetIPAddressByAddressFunc: func(_ string) (*models.IPAddress, error) {
					return nil, ipam.ErrNoObjectsFound
				},
			}

			controllerRecociler := createIPUpdateReconciler(netBoxMock, fileReaderMock)

			_, err := controllerRecociler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedUpdateName})

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unable to create IPAddress"))
			Expect(netBoxMock.IPAMMock.(*mock.IPAMMock).GetIPAddressByAddressCalls).To(Equal(1))
		})

		It("raise error if IPAddress has invalid ip", func() {
			netBoxMock := prepareNetboxMock()

			ipAddress := &ipamv1.IPAddress{}
			Expect(k8sClient.Get(ctx, typeNamespacedUpdateName, ipAddress)).To(Succeed())
			ipAddress.Spec.Address = "invalid-ip"
			Expect(k8sClient.Update(ctx, ipAddress)).To(Succeed())

			controllerRecociler := createIPUpdateReconciler(netBoxMock, fileReaderMock)

			_, err := controllerRecociler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedUpdateName})

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("ParseAddr(\"invalid-ip\"): unable to parse IP"))
		})

		It("succeeds when there is a single netbox prefix found", func() {
			netBoxMock := prepareNetboxMock()
			netBoxMock.IPAMMock = &mock.IPAMMock{
				GetPrefixesByPrefixesFunc: func(_ string) ([]models.Prefix, error) {
					return []models.Prefix{
						{
							Vrf: models.NestedVRF{
								ID: 1,
							},
						},
					}, nil
				},
				GetIPAddressByAddressFunc: func(_ string) (*models.IPAddress, error) {
					return nil, ipam.ErrNoObjectsFound
				},
				CreateIPAddressFunc: func(params ipam.CreateIPAddressParams) (*models.IPAddress, error) {
					Expect(params.VrfID).To(Equal(1))
					return &models.IPAddress{
						NestedIPAddress:   models.NestedIPAddress{ID: ipAddressID},
						AssignedInterface: models.NestedInterface{ID: interfaceID, Device: models.NestedDevice{ID: deviceID}},
					}, nil
				},
			}

			controllerRecociler := createIPUpdateReconciler(netBoxMock, fileReaderMock)

			_, err := controllerRecociler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedUpdateName})

			Expect(err).ToNot(HaveOccurred())
			Expect(netBoxMock.IPAMMock.(*mock.IPAMMock).GetIPAddressByAddressCalls).To(Equal(1))
			Expect(netBoxMock.IPAMMock.(*mock.IPAMMock).CreateIPAddressCalls).To(Equal(1))
			Expect(netBoxMock.IPAMMock.(*mock.IPAMMock).UpdateIPAddressCalls).To(Equal(0))
		})

		It("set conflict annotation if ip exists, and assigned to another interface", func() {
			netBoxMock := prepareNetboxMock()
			netBoxMock.IPAMMock = &mock.IPAMMock{
				GetIPAddressByAddressFunc: func(_ string) (*models.IPAddress, error) {
					return &models.IPAddress{
						NestedIPAddress: models.NestedIPAddress{
							Address: fullIPAddress,
						},
						AssignedInterface: models.NestedInterface{
							ID: -1,
							Device: models.NestedDevice{
								ID: deviceID,
							},
						},
					}, nil
				},
			}

			controllerRecociler := createIPUpdateReconciler(netBoxMock, fileReaderMock)

			_, err := controllerRecociler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedUpdateName})

			Expect(err).ToNot(HaveOccurred())
			Expect(netBoxMock.IPAMMock.(*mock.IPAMMock).GetIPAddressByAddressCalls).To(Equal(1))

			ipAddress := &ipamv1.IPAddress{}
			Expect(k8sClient.Get(ctx, typeNamespacedUpdateName, ipAddress)).To(Succeed())
			Expect(ipAddress.Annotations).To(HaveKeyWithValue("netbox.argora.cloud.sap/conflicted", "interface"))
		})

		It("set conflict annotation if ip exists, and assigned to another device", func() {
			netBoxMock := prepareNetboxMock()
			netBoxMock.IPAMMock = &mock.IPAMMock{
				GetIPAddressByAddressFunc: func(_ string) (*models.IPAddress, error) {
					return &models.IPAddress{
						NestedIPAddress: models.NestedIPAddress{
							Address: fullIPAddress,
						},
						AssignedInterface: models.NestedInterface{
							ID: interfaceID,
							Device: models.NestedDevice{
								ID: -1,
							},
						},
					}, nil
				},
			}

			controllerRecociler := createIPUpdateReconciler(netBoxMock, fileReaderMock)

			_, err := controllerRecociler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedUpdateName})

			Expect(err).ToNot(HaveOccurred())
			Expect(netBoxMock.IPAMMock.(*mock.IPAMMock).GetIPAddressByAddressCalls).To(Equal(1))

			ipAddress := &ipamv1.IPAddress{}
			Expect(k8sClient.Get(ctx, typeNamespacedUpdateName, ipAddress)).To(Succeed())
			Expect(ipAddress.Annotations).To(HaveKeyWithValue("netbox.argora.cloud.sap/conflicted", "device"))
		})

		It("delete conflicted annotation, in case if conflict ended", func() {
			ipAddress := &ipamv1.IPAddress{}
			Expect(k8sClient.Get(ctx, typeNamespacedUpdateName, ipAddress)).To(Succeed())

			ipAddress.Annotations = map[string]string{
				"netbox.argora.cloud.sap/conflicted": "interface",
			}
			Expect(k8sClient.Update(ctx, ipAddress)).To(Succeed())

			netBoxMock := prepareNetboxMock()

			controllerRecociler := createIPUpdateReconciler(netBoxMock, fileReaderMock)

			_, err := controllerRecociler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedUpdateName})

			Expect(netBoxMock.IPAMMock.(*mock.IPAMMock).GetIPAddressByAddressCalls).To(Equal(1))

			Expect(k8sClient.Get(ctx, typeNamespacedUpdateName, ipAddress)).To(Succeed())
			Expect(ipAddress.Annotations).To(Not(HaveKey("netbox.argora.cloud.sap/conflicted")))
			Expect(err).To(Succeed())
		})

		It("updates device primary ip if is not matching", func() {
			netBoxMock := prepareNetboxMock()
			dcimMock := netBoxMock.DCIMMock.(*mock.DCIMMock)
			dcimMock.GetDeviceByNameFunc = func(_ string) (*models.Device, error) {
				return &models.Device{ID: deviceID, PrimaryIP: models.NestedIPAddress{ID: 0}}, nil
			}
			dcimMock.UpdateDeviceFunc = func(_ models.WritableDeviceWithConfigContext) (*models.Device, error) {
				return &models.Device{ID: deviceID, PrimaryIP: models.NestedIPAddress{ID: 0}}, nil
			}

			controllerRecociler := createIPUpdateReconciler(netBoxMock, fileReaderMock)

			_, err := controllerRecociler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedUpdateName})

			Expect(netBoxMock.IPAMMock.(*mock.IPAMMock).GetIPAddressByAddressCalls).To(Equal(1))
			Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).UpdateDeviceCalls).To(Equal(1))

			Expect(err).To(Succeed())
		})

		It("updates device primary ip if is not matching", func() {
			netBoxMock := prepareNetboxMock()
			dcimMock := netBoxMock.DCIMMock.(*mock.DCIMMock)
			dcimMock.GetDeviceByNameFunc = func(_ string) (*models.Device, error) {
				return &models.Device{ID: deviceID, PrimaryIP: models.NestedIPAddress{ID: 0}}, nil
			}
			dcimMock.UpdateDeviceFunc = func(_ models.WritableDeviceWithConfigContext) (*models.Device, error) {
				return &models.Device{ID: deviceID, PrimaryIP: models.NestedIPAddress{ID: 0}}, nil
			}

			controllerRecociler := createIPUpdateReconciler(netBoxMock, fileReaderMock)

			_, err := controllerRecociler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedUpdateName})

			Expect(err).To(Succeed())
			Expect(netBoxMock.IPAMMock.(*mock.IPAMMock).GetIPAddressByAddressCalls).To(Equal(1))
			Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).UpdateDeviceCalls).To(Equal(1))
		})

		It("raise error if error appear on device primary ip update", func() {
			netBoxMock := prepareNetboxMock()
			dcimMock := netBoxMock.DCIMMock.(*mock.DCIMMock)
			dcimMock.GetDeviceByNameFunc = func(_ string) (*models.Device, error) {
				return &models.Device{ID: deviceID, PrimaryIP: models.NestedIPAddress{ID: 0}}, nil
			}
			updateErr := errors.New("new error")
			dcimMock.UpdateDeviceFunc = func(_ models.WritableDeviceWithConfigContext) (*models.Device, error) {
				return nil, updateErr
			}

			controllerRecociler := createIPUpdateReconciler(netBoxMock, fileReaderMock)
			_, err := controllerRecociler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedUpdateName})

			Expect(err).To(HaveOccurred())
			Expect(netBoxMock.IPAMMock.(*mock.IPAMMock).GetIPAddressByAddressCalls).To(Equal(1))
			Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).UpdateDeviceCalls).To(Equal(1))
			Expect(err).To(MatchError(updateErr))
		})

		Context("Deletion", func() {
			BeforeEach(func() {
				ip := &ipamv1.IPAddress{}
				Expect(k8sClient.Get(ctx, typeNamespacedUpdateName, ip)).To(Succeed())
				ip.SetAnnotations(map[string]string{
					"netbox.argora.cloud.sap/device-id":    strconv.Itoa(deviceID),
					"netbox.argora.cloud.sap/interface-id": strconv.Itoa(interfaceID),
				})
				Expect(k8sClient.Update(ctx, ip)).To(Succeed())
			})
			It("does nothing when IP not found in NetBox", func() {
				netBoxMock := prepareNetboxMock()
				ipamMock := netBoxMock.IPAMMock.(*mock.IPAMMock)
				ipamMock.GetIPAddressByAddressFunc = func(address string) (*models.IPAddress, error) {
					return nil, ipam.ErrNoObjectsFound
				}

				controllerReconciler := createIPUpdateReconciler(netBoxMock, fileReaderMock)

				ip := &ipamv1.IPAddress{}
				Expect(k8sClient.Get(ctx, typeNamespacedUpdateName, ip)).To(Succeed())
				Expect(k8sClient.Delete(ctx, ip)).To(Succeed())

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedUpdateName})
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns error when GetIPAddressByAddress fails", func() {
				netBoxMock := prepareNetboxMock()
				ipamMock := netBoxMock.IPAMMock.(*mock.IPAMMock)
				ipamMock.GetIPAddressByAddressFunc = func(address string) (*models.IPAddress, error) {
					return nil, errors.New("netbox error")
				}

				controllerReconciler := createIPUpdateReconciler(netBoxMock, fileReaderMock)

				ip := &ipamv1.IPAddress{}
				Expect(k8sClient.Get(ctx, typeNamespacedUpdateName, ip)).To(Succeed())
				Expect(k8sClient.Delete(ctx, ip)).To(Succeed())

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedUpdateName})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unable to find IP in NetBox"))
			})

			It("returns error when IP address is invalid", func() {
				netBoxMock := prepareNetboxMock()

				controllerReconciler := createIPUpdateReconciler(netBoxMock, fileReaderMock)

				ip := &ipamv1.IPAddress{}
				Expect(k8sClient.Get(ctx, typeNamespacedUpdateName, ip)).To(Succeed())
				ip.Spec.Address = "invalid-ip"
				Expect(k8sClient.Update(ctx, ip)).To(Succeed())
				Expect(k8sClient.Delete(ctx, ip)).To(Succeed())

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedUpdateName})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("ParseAddr(\"invalid-ip\"): unable to parse IP"))
			})

			It("skips deletion when findNetboxTarget when can't find device", func() {
				netBoxMock := prepareNetboxMock()
				ipamMock := netBoxMock.IPAMMock.(*mock.IPAMMock)
				ipamMock.GetIPAddressByAddressFunc = func(address string) (*models.IPAddress, error) {
					return &models.IPAddress{
						NestedIPAddress: models.NestedIPAddress{ID: ipAddressID, Address: fullIPAddress},
					}, nil
				}
				dcim := netBoxMock.DCIMMock.(*mock.DCIMMock)
				dcim.GetDeviceByNameFunc = func(deviceName string) (*models.Device, error) {
					return nil, errors.New("device not found")
				}

				controllerReconciler := createIPUpdateReconciler(netBoxMock, fileReaderMock)

				ip := &ipamv1.IPAddress{}
				Expect(k8sClient.Get(ctx, typeNamespacedUpdateName, ip)).To(Succeed())
				Expect(k8sClient.Delete(ctx, ip)).To(Succeed())

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedUpdateName})
				Expect(err).ToNot(HaveOccurred())
			})

			It("skips deletion when findNetboxTarget fails if can't find interfaces", func() {
				netBoxMock := prepareNetboxMock()
				ipamMock := netBoxMock.IPAMMock.(*mock.IPAMMock)
				ipamMock.GetIPAddressByAddressFunc = func(address string) (*models.IPAddress, error) {
					return &models.IPAddress{
						NestedIPAddress: models.NestedIPAddress{ID: ipAddressID, Address: fullIPAddress},
					}, nil
				}
				dcim := netBoxMock.DCIMMock.(*mock.DCIMMock)
				dcim.GetInterfacesForDeviceFunc = func(device *models.Device) ([]models.Interface, error) {
					return nil, errors.New("interfaces not found")
				}

				controllerReconciler := createIPUpdateReconciler(netBoxMock, fileReaderMock)

				ip := &ipamv1.IPAddress{}
				Expect(k8sClient.Get(ctx, typeNamespacedUpdateName, ip)).To(Succeed())
				Expect(k8sClient.Delete(ctx, ip)).To(Succeed())

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedUpdateName})
				Expect(err).ToNot(HaveOccurred())
			})

			It("skips deletion when no annotations found", func() {
				netBoxMock := prepareNetboxMock()
				ipamMock := netBoxMock.IPAMMock.(*mock.IPAMMock)
				ipamMock.GetIPAddressByAddressFunc = func(address string) (*models.IPAddress, error) {
					return &models.IPAddress{
						NestedIPAddress: models.NestedIPAddress{ID: ipAddressID, Address: fullIPAddress},
					}, nil
				}

				controllerReconciler := createIPUpdateReconciler(netBoxMock, fileReaderMock)

				ip := &ipamv1.IPAddress{}
				Expect(k8sClient.Get(ctx, typeNamespacedUpdateName, ip)).To(Succeed())
				ip.Annotations = nil
				Expect(k8sClient.Update(ctx, ip)).To(Succeed())
				Expect(k8sClient.Delete(ctx, ip)).To(Succeed())

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedUpdateName})
				Expect(err).ToNot(HaveOccurred())
				Expect(netBoxMock.IPAMMock.(*mock.IPAMMock).DeleteIPAddressCalls).To(Equal(0))
			})

			It("skips deletion when annotations are not integers", func() {
				netBoxMock := prepareNetboxMock()
				ipamMock := netBoxMock.IPAMMock.(*mock.IPAMMock)
				ipamMock.GetIPAddressByAddressFunc = func(address string) (*models.IPAddress, error) {
					return &models.IPAddress{
						NestedIPAddress: models.NestedIPAddress{ID: ipAddressID, Address: fullIPAddress},
					}, nil
				}

				controllerReconciler := createIPUpdateReconciler(netBoxMock, fileReaderMock)

				ip := &ipamv1.IPAddress{}
				Expect(k8sClient.Get(ctx, typeNamespacedUpdateName, ip)).To(Succeed())
				ip.Annotations["netbox.argora.cloud.sap/device-id"] = "string"
				ip.Annotations["netbox.argora.cloud.sap/interface-id"] = "string"
				Expect(k8sClient.Update(ctx, ip)).To(Succeed())
				Expect(k8sClient.Delete(ctx, ip)).To(Succeed())

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedUpdateName})
				Expect(err).ToNot(HaveOccurred())
				Expect(netBoxMock.IPAMMock.(*mock.IPAMMock).DeleteIPAddressCalls).To(Equal(0))
			})

			It("skips deletion when IP assigned to different interface", func() {
				netBoxMock := prepareNetboxMock()
				ipamMock := netBoxMock.IPAMMock.(*mock.IPAMMock)
				ipamMock.GetIPAddressByAddressFunc = func(address string) (*models.IPAddress, error) {
					return &models.IPAddress{
						NestedIPAddress:  models.NestedIPAddress{ID: ipAddressID, Address: fullIPAddress},
						AssignedObjectID: 999,
					}, nil
				}

				controllerReconciler := createIPUpdateReconciler(netBoxMock, fileReaderMock)

				ip := &ipamv1.IPAddress{}
				Expect(k8sClient.Get(ctx, typeNamespacedUpdateName, ip)).To(Succeed())
				Expect(k8sClient.Delete(ctx, ip)).To(Succeed())

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedUpdateName})
				Expect(err).ToNot(HaveOccurred())
			})

			It("deletes IP when assigned to expected interface", func() {
				netBoxMock := prepareNetboxMock()
				ipamMock := netBoxMock.IPAMMock.(*mock.IPAMMock)
				ipamMock.GetIPAddressByAddressFunc = func(address string) (*models.IPAddress, error) {
					return &models.IPAddress{
						NestedIPAddress:  models.NestedIPAddress{ID: ipAddressID, Address: fullIPAddress},
						AssignedObjectID: interfaceID,
					}, nil
				}
				ipamMock.DeleteIPAddressFunc = func(id int) error {
					if id != ipAddressID {
						return errors.New("unexpected delete id")
					}
					return nil
				}

				dcim := netBoxMock.DCIMMock.(*mock.DCIMMock)
				dcim.GetInterfacesForDeviceFunc = func(device *models.Device) ([]models.Interface, error) {
					return []models.Interface{
						{
							NestedInterface: models.NestedInterface{ID: interfaceID},
							Name:            "LAG1",
							Type:            models.InterfaceType{Value: "lag"},
						},
					}, nil
				}

				controllerReconciler := createIPUpdateReconciler(netBoxMock, fileReaderMock)

				ip := &ipamv1.IPAddress{}
				Expect(k8sClient.Get(ctx, typeNamespacedUpdateName, ip)).To(Succeed())
				Expect(k8sClient.Delete(ctx, ip)).To(Succeed())

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedUpdateName})
				Expect(err).ToNot(HaveOccurred())
				Expect(netBoxMock.IPAMMock.(*mock.IPAMMock).DeleteIPAddressCalls).To(Equal(1))
			})

			It("successfully deletes IP when IPAddressClaim CR is deleted", func() {
				netBoxMock := prepareNetboxMock()
				ipamMock := netBoxMock.IPAMMock.(*mock.IPAMMock)
				ipamMock.GetIPAddressByAddressFunc = func(address string) (*models.IPAddress, error) {
					return &models.IPAddress{
						NestedIPAddress:  models.NestedIPAddress{ID: ipAddressID, Address: fullIPAddress},
						AssignedObjectID: interfaceID,
					}, nil
				}
				ipamMock.DeleteIPAddressFunc = func(id int) error {
					if id != ipAddressID {
						return errors.New("unexpected delete id")
					}
					return nil
				}

				dcim := netBoxMock.DCIMMock.(*mock.DCIMMock)
				dcim.GetInterfacesForDeviceFunc = func(device *models.Device) ([]models.Interface, error) {
					return []models.Interface{
						{
							NestedInterface: models.NestedInterface{ID: interfaceID},
							Name:            "LAG1",
							Type:            models.InterfaceType{Value: "lag"},
						},
					}, nil
				}

				controllerReconciler := createIPUpdateReconciler(netBoxMock, fileReaderMock)

				ipClaim := &ipamv1.IPAddressClaim{}
				Expect(k8sClient.Get(ctx, client.ObjectKey{
					Name:      "test-claim",
					Namespace: resourceNamespace,
				}, ipClaim)).To(Succeed())
				Expect(k8sClient.Delete(ctx, ipClaim)).To(Succeed())

				ip := &ipamv1.IPAddress{}
				Expect(k8sClient.Get(ctx, typeNamespacedUpdateName, ip)).To(Succeed())
				Expect(k8sClient.Delete(ctx, ip)).To(Succeed())

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedUpdateName})
				Expect(err).ToNot(HaveOccurred())
				Expect(netBoxMock.IPAMMock.(*mock.IPAMMock).DeleteIPAddressCalls).To(Equal(1))
			})

			It("reports error when DeleteIPAddress fails", func() {
				netBoxMock := prepareNetboxMock()
				ipamMock := netBoxMock.IPAMMock.(*mock.IPAMMock)
				ipamMock.GetIPAddressByAddressFunc = func(address string) (*models.IPAddress, error) {
					return &models.IPAddress{
						NestedIPAddress:  models.NestedIPAddress{ID: ipAddressID, Address: fullIPAddress},
						AssignedObjectID: interfaceID,
					}, nil
				}
				ipamMock.DeleteIPAddressFunc = func(id int) error {
					return errors.New("delete failed")
				}

				dcim := netBoxMock.DCIMMock.(*mock.DCIMMock)
				dcim.GetInterfacesForDeviceFunc = func(device *models.Device) ([]models.Interface, error) {
					return []models.Interface{
						{
							NestedInterface: models.NestedInterface{ID: interfaceID},
							Name:            "LAG1",
							Type:            models.InterfaceType{Value: "lag"},
						},
					}, nil
				}

				controllerReconciler := createIPUpdateReconciler(netBoxMock, fileReaderMock)

				ip := &ipamv1.IPAddress{}
				Expect(k8sClient.Get(ctx, typeNamespacedUpdateName, ip)).To(Succeed())
				Expect(k8sClient.Delete(ctx, ip)).To(Succeed())

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedUpdateName})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("delete ip from netbox"))
			})

			It("returns error when IP prefix cannot be parsed", func() {
				netBoxMock := prepareNetboxMock()
				controllerReconciler := createIPUpdateReconciler(netBoxMock, fileReaderMock)

				ip := &ipamv1.IPAddress{}
				Expect(k8sClient.Get(ctx, typeNamespacedUpdateName, ip)).To(Succeed())
				ip.Spec.Address = "not-an-ip"
				Expect(k8sClient.Update(ctx, ip)).To(Succeed())
				Expect(k8sClient.Delete(ctx, ip)).To(Succeed())

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedUpdateName})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("ParseAddr"))
			})
		})
	})

	Context("findTargetInterface", func() {
		It("returns error when no LAG interfaces exist", func() {
			r := &IPUpdateReconciler{}

			_, err := r.findTargetInterface([]models.Interface{
				{
					Name: "eth0",
					Type: models.InterfaceType{Value: "1000base-t"},
				},
			})

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no LAG interface"))
		})

		It("returns the only LAG interface when only one exists", func() {
			r := &IPUpdateReconciler{}

			iface, err := r.findTargetInterface([]models.Interface{
				{
					Name: "LAG0",
					Type: models.InterfaceType{Value: "lag"},
				},
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(iface.Name).To(Equal("LAG0"))
		})

		It("returns the highest numbered LAG interface", func() {
			r := &IPUpdateReconciler{}

			iface, err := r.findTargetInterface([]models.Interface{
				{
					Name: "LAG0",
					Type: models.InterfaceType{Value: "lag"},
				},
				{
					Name: "LAG2",
					Type: models.InterfaceType{Value: "lag"},
				},
				{
					Name: "LAG1",
					Type: models.InterfaceType{Value: "lag"},
				},
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(iface.Name).To(Equal("LAG2"))
		})
	})
})

func createIPUpdateReconciler(netBoxMock *mock.NetBoxMock, fileReaderMock credentials.FileReader) *IPUpdateReconciler {
	return &IPUpdateReconciler{
		k8sClient:   k8sClient,
		scheme:      k8sClient.Scheme(),
		netBox:      netBoxMock,
		credentials: credentials.NewDefaultCredentials(fileReaderMock),
	}
}
