// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"

	metalv1alpha1 "github.com/ironcore-dev/metal-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sapcc/go-netbox-go/models"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ipamv1 "sigs.k8s.io/cluster-api/api/ipam/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/sapcc/argora/internal/controller/mock"
	"github.com/sapcc/argora/internal/credentials"
	"github.com/sapcc/argora/internal/netbox/ipam"
)

const ipAddressString = "192.168.1.100"
const ipAddressMask int32 = 24
const fullIPAddress = "192.168.1.100/24"
const interfaceID = 123
const deviceID = 321

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
							ID:   deviceID,
							Name: "node013-ap002",
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
			By("reset Server CR")
			server := &metalv1alpha1.Server{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-server",
					Namespace: resourceNamespace,
				},
			}

			_ = k8sClient.Delete(ctx, server)

			server.Spec = metalv1alpha1.ServerSpec{
				SystemUUID: "test_uuid",
				BMCRef: &v1.LocalObjectReference{
					Name: "node013-ap002",
				},
			}

			Expect(k8sClient.Create(ctx, server)).To(Succeed())

			By("reset ServerClaim CR")
			serverClaim := &metalv1alpha1.ServerClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-server-claim",
					Namespace: resourceNamespace,
				},
			}

			_ = k8sClient.Delete(ctx, serverClaim)

			serverClaim.Spec = metalv1alpha1.ServerClaimSpec{
				ServerRef: &v1.LocalObjectReference{
					Name: "test-server",
				},
				Image: "gardenlinux:latest",
				Power: "On",
			}

			Expect(k8sClient.Create(ctx, serverClaim)).To(Succeed())

			By("reset IPAM IPAddressClaim CR")
			ipClaim := &ipamv1.IPAddressClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-claim",
					Namespace: resourceNamespace,
				},
			}

			_ = k8sClient.Delete(ctx, ipClaim)

			ipClaim = &ipamv1.IPAddressClaim{
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

			By("reset IPAM IPAddress CR")
			ipAddress.ObjectMeta = metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: resourceNamespace,
			}

			_ = k8sClient.Delete(ctx, ipAddress)

			ipAddress = &ipamv1.IPAddress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: resourceNamespace,
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
			By("delete IPAM IPAddress CR")
			err := k8sClient.Get(ctx, typeNamespacedUpdateName, ipAddress)
			if err == nil {
				Expect(k8sClient.Delete(ctx, ipAddress)).To(Succeed())
			}
		})

		It("leshould successfully reconci the CR", func() {
			// given
			netBoxMock := prepareNetboxMock()
			controllerReconciler := createIPUpdateReconciler(netBoxMock, fileReaderMock)

			// when
			By("reconciling IPAM IPAddress CR")
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedUpdateName})

			// then
			Expect(err).ToNot(HaveOccurred())
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
			Expect(err.Error()).To(ContainSubstring("get Server"))
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

		It("create IP address if address don't exist", func() {
			netBoxMock := prepareNetboxMock()
			netBoxMock.IPAMMock = &mock.IPAMMock{
				GetIPAddressByAddressFunc: func(_ string) (*models.IPAddress, error) {
					return nil, errors.New("not exists")
				},
				CreateIPAddressFunc: func(_ ipam.CreateIPAddressParams) (*models.IPAddress, error) {
					return &models.IPAddress{}, nil
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

		It("raise error if ip exists, and assigned to another interface", func() {
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

			Expect(err).To(HaveOccurred())
			Expect(netBoxMock.IPAMMock.(*mock.IPAMMock).GetIPAddressByAddressCalls).To(Equal(1))
		})

		It("raise error if ip exists, and assigned to another device", func() {
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

			Expect(err).To(HaveOccurred())
			Expect(netBoxMock.IPAMMock.(*mock.IPAMMock).GetIPAddressByAddressCalls).To(Equal(1))
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
