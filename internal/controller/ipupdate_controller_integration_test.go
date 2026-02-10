// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"

	metalv1alpha1 "github.com/ironcore-dev/metal-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sapcc/argora/internal/controller/mock"
	"github.com/sapcc/argora/internal/credentials"
	"github.com/sapcc/argora/internal/netbox/ipam"
	"github.com/sapcc/go-netbox-go/models"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ipamv1 "sigs.k8s.io/cluster-api/api/ipam/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("IP Update Controller Envtest Integration", func() {
	const (
		testNamespace             = "default"
		testIPName                = "test-ip-envtest"
		testClaimName             = "test-ip-claim-envtest"
		testServerClaimName       = "test-server-claim-envtest"
		testServerName            = "test-server-envtest"
		testDeviceName            = "node013-ap002"
		testIPAddressStr          = "192.168.1.100"
		testIPPrefix        int32 = 24
		testInterfaceID           = 123
		testDeviceID              = 321
		testIPAddressID           = 456
	)

	var (
		testCtx            context.Context
		reconciler         *IPUpdateReconciler
		netBoxMock         *mock.NetBoxMock
		fileReaderMock     *mock.FileReaderMock
		testIPAddress      *ipamv1.IPAddress
		testIPAddressClaim *ipamv1.IPAddressClaim
		testServerClaim    *metalv1alpha1.ServerClaim
		testServer         *metalv1alpha1.Server
	)

	BeforeEach(func() {
		testCtx = context.Background()

		fileReaderMock = &mock.FileReaderMock{
			FileContent: map[string]string{
				"/etc/credentials/credentials.json": `{
					"bmcUser": "user",
					"bmcPassword": "password",
					"netboxToken": "token"
				}`,
			},
			ReturnError: false,
		}

		netBoxMock = createDefaultNetBoxMockForEnvtest(testDeviceID, testInterfaceID, testIPAddressID)

		creds := credentials.NewDefaultCredentials(fileReaderMock)
		reconciler = &IPUpdateReconciler{
			k8sClient:   k8sClient,
			scheme:      k8sClient.Scheme(),
			credentials: creds,
			netBox:      netBoxMock,
		}
	})

	Context("with real Kubernetes API", func() {
		BeforeEach(func() {
			By("Creating Server CR")
			testServer = &metalv1alpha1.Server{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testServerName,
					Namespace: testNamespace,
				},
				Spec: metalv1alpha1.ServerSpec{
					SystemUUID: "test_uuid_123",
					BMCRef: &v1.LocalObjectReference{
						Name: testDeviceName,
					},
				},
			}
			Expect(k8sClient.Create(testCtx, testServer)).To(Succeed())

			By("Creating ServerClaim CR")
			testServerClaim = &metalv1alpha1.ServerClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testServerClaimName,
					Namespace: testNamespace,
				},
				Spec: metalv1alpha1.ServerClaimSpec{
					ServerRef: &v1.LocalObjectReference{
						Name: testServerName,
					},
					Image: "gardenlinux:latest",
					Power: "On",
				},
			}
			Expect(k8sClient.Create(testCtx, testServerClaim)).To(Succeed())

			By("Creating IPAddressClaim CR")
			testIPAddressClaim = &ipamv1.IPAddressClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testClaimName,
					Namespace: testNamespace,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "metal.ironcore.dev/v1alpha1",
							Kind:       "ServerClaim",
							Name:       testServerClaimName,
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
			Expect(k8sClient.Create(testCtx, testIPAddressClaim)).To(Succeed())

			By("Creating IPAddress CR")
			testIPAddress = &ipamv1.IPAddress{
				ObjectMeta: metav1.ObjectMeta{
					Name:       testIPName,
					Namespace:  testNamespace,
					Finalizers: []string{ipAddressFinalizer},
				},
				Spec: ipamv1.IPAddressSpec{
					Address: testIPAddressStr,
					PoolRef: ipamv1.IPPoolReference{
						Name:     "test-ippool",
						APIGroup: "ipam.cluster.x-k8s.io",
						Kind:     "IPPool",
					},
					ClaimRef: ipamv1.IPAddressClaimReference{
						Name: testClaimName,
					},
					Prefix: ptr.To(testIPPrefix),
				},
			}
			Expect(k8sClient.Create(testCtx, testIPAddress)).To(Succeed())
		})

		AfterEach(func() {
			By("Cleanup IPAddress")
			ip := &ipamv1.IPAddress{}
			if err := k8sClient.Get(testCtx, types.NamespacedName{Name: testIPName, Namespace: testNamespace}, ip); err == nil {
				ip.Finalizers = nil
				k8sClient.Update(testCtx, ip)
				k8sClient.Delete(testCtx, ip)
			}

			By("Cleanup IPAddressClaim")
			claim := &ipamv1.IPAddressClaim{}
			if err := k8sClient.Get(testCtx, types.NamespacedName{Name: testClaimName, Namespace: testNamespace}, claim); err == nil {
				claim.Finalizers = nil
				k8sClient.Update(testCtx, claim)
				k8sClient.Delete(testCtx, claim)
			}

			By("Cleanup ServerClaim")
			sc := &metalv1alpha1.ServerClaim{}
			if err := k8sClient.Get(testCtx, types.NamespacedName{Name: testServerClaimName, Namespace: testNamespace}, sc); err == nil {
				sc.Finalizers = nil
				k8sClient.Update(testCtx, sc)
				k8sClient.Delete(testCtx, sc)
			}

			By("Cleanup Server")
			s := &metalv1alpha1.Server{}
			if err := k8sClient.Get(testCtx, types.NamespacedName{Name: testServerName, Namespace: testNamespace}, s); err == nil {
				s.Finalizers = nil
				k8sClient.Update(testCtx, s)
				k8sClient.Delete(testCtx, s)
			}
		})

		It("should successfully reconcile IPAddress when IP does not exist in NetBox", func() {
			netBoxMock.IPAMMock = &mock.IPAMMock{
				GetIPAddressByAddressFunc: func(_ string) (*models.IPAddress, error) {
					return nil, ipam.ErrNoObjectsFound
				},
				CreateIPAddressFunc: func(params ipam.CreateIPAddressParams) (*models.IPAddress, error) {
					Expect(params.Address).To(Equal("192.168.1.100/24"))
					return &models.IPAddress{
						NestedIPAddress: models.NestedIPAddress{ID: testIPAddressID},
						AssignedInterface: models.NestedInterface{
							ID:     testInterfaceID,
							Device: models.NestedDevice{ID: testDeviceID},
						},
					}, nil
				},
				GetPrefixesByPrefixesFunc: func(_ string) ([]models.Prefix, error) {
					return nil, nil
				},
			}

			result, err := reconciler.Reconcile(testCtx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: testIPName, Namespace: testNamespace},
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(result.RequeueAfter).To(BeZero())

			ip := &ipamv1.IPAddress{}
			Expect(k8sClient.Get(testCtx, types.NamespacedName{Name: testIPName, Namespace: testNamespace}, ip)).To(Succeed())
			Expect(ip.Spec.Address).To(Equal(testIPAddressStr))
		})

		It("should handle matching state between Kubernetes and NetBox", func() {
			netBoxMock.IPAMMock = &mock.IPAMMock{
				GetIPAddressByAddressFunc: func(_ string) (*models.IPAddress, error) {
					return &models.IPAddress{
						NestedIPAddress: models.NestedIPAddress{
							ID:      testIPAddressID,
							Address: "192.168.1.100/24",
						},
						AssignedInterface: models.NestedInterface{
							ID:     testInterfaceID,
							Device: models.NestedDevice{ID: testDeviceID},
						},
					}, nil
				},
			}

			result, err := reconciler.Reconcile(testCtx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: testIPName, Namespace: testNamespace},
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(result.RequeueAfter).To(BeZero())
		})

		It("should update device primary IP when mismatch exists", func() {
			updateDeviceCalled := false
			netBoxMock.DCIMMock = &mock.DCIMMock{
				GetDeviceByNameFunc: func(_ string) (*models.Device, error) {
					return &models.Device{
						ID:        testDeviceID,
						PrimaryIP: models.NestedIPAddress{ID: 0},
					}, nil
				},
				GetInterfacesForDeviceFunc: func(_ *models.Device) ([]models.Interface, error) {
					return []models.Interface{
						{
							NestedInterface: models.NestedInterface{ID: testInterfaceID},
							Name:            "LAG1",
							Type:            models.InterfaceType{Value: "lag"},
						},
					}, nil
				},
				UpdateDeviceFunc: func(d models.WritableDeviceWithConfigContext) (*models.Device, error) {
					updateDeviceCalled = true
					Expect(d.PrimaryIP4).To(Equal(testIPAddressID))
					return &models.Device{ID: testDeviceID}, nil
				},
			}

			netBoxMock.IPAMMock = &mock.IPAMMock{
				GetIPAddressByAddressFunc: func(_ string) (*models.IPAddress, error) {
					return &models.IPAddress{
						NestedIPAddress: models.NestedIPAddress{ID: testIPAddressID},
						AssignedInterface: models.NestedInterface{
							ID:     testInterfaceID,
							Device: models.NestedDevice{ID: testDeviceID},
						},
					}, nil
				},
			}

			result, err := reconciler.Reconcile(testCtx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: testIPName, Namespace: testNamespace},
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(result.RequeueAfter).To(BeZero())
			Expect(updateDeviceCalled).To(BeTrue())
		})

		It("should fail when IPAddressClaim is missing", func() {
			ipClaim := &ipamv1.IPAddressClaim{}
			Expect(k8sClient.Get(testCtx, types.NamespacedName{Name: testClaimName, Namespace: testNamespace}, ipClaim)).To(Succeed())
			Expect(k8sClient.Delete(testCtx, ipClaim)).To(Succeed())

			_, err := reconciler.Reconcile(testCtx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: testIPName, Namespace: testNamespace},
			})

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to get IPAddressClaim"))
		})

		It("should fail when ServerClaim is missing", func() {
			sc := &metalv1alpha1.ServerClaim{}
			Expect(k8sClient.Get(testCtx, types.NamespacedName{Name: testServerClaimName, Namespace: testNamespace}, sc)).To(Succeed())
			Expect(k8sClient.Delete(testCtx, sc)).To(Succeed())

			_, err := reconciler.Reconcile(testCtx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: testIPName, Namespace: testNamespace},
			})

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to get ServerClaim"))
		})

		It("should fail when Server is missing", func() {
			s := &metalv1alpha1.Server{}
			Expect(k8sClient.Get(testCtx, types.NamespacedName{Name: testServerName, Namespace: testNamespace}, s)).To(Succeed())
			Expect(k8sClient.Delete(testCtx, s)).To(Succeed())

			_, err := reconciler.Reconcile(testCtx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: testIPName, Namespace: testNamespace},
			})

			Expect(err).To(HaveOccurred())
		})

		It("should fail when ServerClaim is not bound to Server", func() {
			sc := &metalv1alpha1.ServerClaim{}
			Expect(k8sClient.Get(testCtx, types.NamespacedName{Name: testServerClaimName, Namespace: testNamespace}, sc)).To(Succeed())
			Expect(k8sClient.Delete(testCtx, sc)).To(Succeed())

			sc = &metalv1alpha1.ServerClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testServerClaimName,
					Namespace: testNamespace,
				},
				Spec: metalv1alpha1.ServerClaimSpec{
					Image: "gardenlinux:latest",
					Power: "On",
				},
			}
			Expect(k8sClient.Create(testCtx, sc)).To(Succeed())

			_, err := reconciler.Reconcile(testCtx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: testIPName, Namespace: testNamespace},
			})

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not yet bound to a server"))
		})

		It("should add finalizer on initial reconciliation", func() {
			newIPName := "test-ip-new-envtest"
			newIP := &ipamv1.IPAddress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      newIPName,
					Namespace: testNamespace,
				},
				Spec: ipamv1.IPAddressSpec{
					Address: testIPAddressStr,
					PoolRef: ipamv1.IPPoolReference{
						Name:     "test-ippool",
						APIGroup: "ipam.cluster.x-k8s.io",
						Kind:     "IPPool",
					},
					ClaimRef: ipamv1.IPAddressClaimReference{
						Name: testClaimName,
					},
					Prefix: ptr.To(testIPPrefix),
				},
			}
			Expect(k8sClient.Create(testCtx, newIP)).To(Succeed())

			netBoxMock.IPAMMock = &mock.IPAMMock{
				GetIPAddressByAddressFunc: func(_ string) (*models.IPAddress, error) {
					return nil, ipam.ErrNoObjectsFound
				},
				CreateIPAddressFunc: func(_ ipam.CreateIPAddressParams) (*models.IPAddress, error) {
					return &models.IPAddress{
						NestedIPAddress: models.NestedIPAddress{ID: testIPAddressID},
						AssignedInterface: models.NestedInterface{
							ID:     testInterfaceID,
							Device: models.NestedDevice{ID: testDeviceID},
						},
					}, nil
				},
				GetPrefixesByPrefixesFunc: func(_ string) ([]models.Prefix, error) {
					return nil, nil
				},
			}

			_, err := reconciler.Reconcile(testCtx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: newIPName, Namespace: testNamespace},
			})

			Expect(err).ToNot(HaveOccurred())

			ip := &ipamv1.IPAddress{}
			Expect(k8sClient.Get(testCtx, types.NamespacedName{Name: newIPName, Namespace: testNamespace}, ip)).To(Succeed())
			Expect(ip.Finalizers).To(ContainElement(ipAddressFinalizer))

			ip.Finalizers = nil
			k8sClient.Update(testCtx, ip)
			k8sClient.Delete(testCtx, ip)
		})

		It("should handle NetBox connectivity failures", func() {
			netBoxMock.IPAMMock = &mock.IPAMMock{
				GetIPAddressByAddressFunc: func(_ string) (*models.IPAddress, error) {
					return nil, errors.New("network unreachable")
				},
			}

			_, err := reconciler.Reconcile(testCtx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: testIPName, Namespace: testNamespace},
			})

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("network unreachable"))
		})

		It("should detect IP conflict with different device", func() {
			netBoxMock.IPAMMock = &mock.IPAMMock{
				GetIPAddressByAddressFunc: func(_ string) (*models.IPAddress, error) {
					return &models.IPAddress{
						NestedIPAddress: models.NestedIPAddress{ID: testIPAddressID},
						AssignedInterface: models.NestedInterface{
							ID:     testInterfaceID,
							Device: models.NestedDevice{ID: 9999},
						},
					}, nil
				},
			}

			_, err := reconciler.Reconcile(testCtx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: testIPName, Namespace: testNamespace},
			})

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("already assigned device"))
		})
	})
})

func createDefaultNetBoxMockForEnvtest(deviceID, interfaceID, ipAddressID int) *mock.NetBoxMock {
	return &mock.NetBoxMock{
		ReturnError:        false,
		VirtualizationMock: &mock.VirtualizationMock{},
		DCIMMock: &mock.DCIMMock{
			GetDeviceByNameFunc: func(_ string) (*models.Device, error) {
				return &models.Device{
					ID:        deviceID,
					Name:      "node013-ap002",
					PrimaryIP: models.NestedIPAddress{ID: ipAddressID},
				}, nil
			},
			GetInterfacesForDeviceFunc: func(_ *models.Device) ([]models.Interface, error) {
				return []models.Interface{
					{
						NestedInterface: models.NestedInterface{ID: interfaceID},
						Name:            "LAG1",
						Type:            models.InterfaceType{Value: "lag"},
					},
				}, nil
			},
		},
		IPAMMock: &mock.IPAMMock{
			GetIPAddressByAddressFunc: func(_ string) (*models.IPAddress, error) {
				return &models.IPAddress{
					NestedIPAddress: models.NestedIPAddress{ID: ipAddressID},
					AssignedInterface: models.NestedInterface{
						ID:     interfaceID,
						Device: models.NestedDevice{ID: deviceID},
					},
				}, nil
			},
		},
		ExtrasMock: &mock.ExtrasMock{},
	}
}
