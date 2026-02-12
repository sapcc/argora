// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	metalv1alpha1 "github.com/ironcore-dev/metal-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sapcc/go-netbox-go/models"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ipamv1 "sigs.k8s.io/cluster-api/api/ipam/v1beta2"

	"github.com/sapcc/argora/internal/controller/mock"
	"github.com/sapcc/argora/internal/netbox/ipam"
)

var _ = Describe("IP Update Controller Envtest Integration", func() {
	const (
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
		netBoxMock         *mock.NetBoxMock
		fileReaderMock     *mock.FileReaderMock
		testIPAddress      *ipamv1.IPAddress
		testIPAddressClaim *ipamv1.IPAddressClaim
		testServerClaim    *metalv1alpha1.ServerClaim
		testServer         *metalv1alpha1.Server
		namespace          *v1.Namespace
		localMgrCancel     context.CancelFunc
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

		// Set up manager and reconciler for this test
		_, _, localMgrCancel = SetupManagerForIntegrationTest(fileReaderMock, netBoxMock)

		// Create a fresh namespace for each test
		namespace = CreateTestNamespace()
		testNamespace = namespace
	})

	AfterEach(func() {
		// Clean up resources in the test namespace
		EnsureCleanState()
		if localMgrCancel != nil {
			localMgrCancel()
		}
	})

	Context("with real Kubernetes API", func() {
		BeforeEach(func() {
			By("Creating Server CR")
			testServer = &metalv1alpha1.Server{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testServerName,
					Namespace: namespace.Name,
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
					Namespace: namespace.Name,
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
					Namespace: namespace.Name,
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
					Namespace:  namespace.Name,
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

			Eventually(func() (string, error) {
				ip := &ipamv1.IPAddress{}
				if err := k8sClient.Get(testCtx, types.NamespacedName{Name: testIPName, Namespace: namespace.Name}, ip); err != nil {
					return "", err
				}
				if v, ok := ip.Annotations[annotationDeviceKey]; ok {
					return v, nil
				}
				return "", fmt.Errorf("annotation %s not set", annotationDeviceKey)
			}, 5*time.Second, 250*time.Millisecond).Should(Not(BeEmpty()))
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

			Eventually(func() error {
				ip := &ipamv1.IPAddress{}
				if err := k8sClient.Get(testCtx, types.NamespacedName{Name: testIPName, Namespace: namespace.Name}, ip); err != nil {
					return err
				}
				if _, ok := ip.Annotations[annotationDeviceKey]; !ok {
					return fmt.Errorf("annotation %s not set", annotationDeviceKey)
				}
				return nil
			}, 5*time.Second, 250*time.Millisecond).Should(Succeed())
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

			Eventually(func() bool {
				return updateDeviceCalled
			}, 5*time.Second, 250*time.Millisecond).Should(BeTrue())
		})

		It("should fail when IPAddressClaim is missing", func() {
			ipClaim := &ipamv1.IPAddressClaim{}
			Expect(k8sClient.Get(testCtx, types.NamespacedName{Name: testClaimName, Namespace: namespace.Name}, ipClaim)).To(Succeed())
			Expect(k8sClient.Delete(testCtx, ipClaim)).To(Succeed())

			Eventually(func() error {
				ip := &ipamv1.IPAddress{}
				if err := k8sClient.Get(testCtx, types.NamespacedName{Name: testIPName, Namespace: namespace.Name}, ip); err != nil {
					return err
				}
				_, hasAnnotation := ip.Annotations[annotationDeviceKey]
				if hasAnnotation {
					return fmt.Errorf("expected no annotation but found one")
				}
				return nil
			}, 5*time.Second, 250*time.Millisecond).Should(Succeed())
		})

		It("should fail when ServerClaim is missing", func() {
			sc := &metalv1alpha1.ServerClaim{}
			Expect(k8sClient.Get(testCtx, types.NamespacedName{Name: testServerClaimName, Namespace: namespace.Name}, sc)).To(Succeed())
			Expect(k8sClient.Delete(testCtx, sc)).To(Succeed())

			Eventually(func() error {
				ip := &ipamv1.IPAddress{}
				if err := k8sClient.Get(testCtx, types.NamespacedName{Name: testIPName, Namespace: namespace.Name}, ip); err != nil {
					return err
				}
				_, hasAnnotation := ip.Annotations[annotationDeviceKey]
				if hasAnnotation {
					return fmt.Errorf("expected no annotation but found one")
				}
				return nil
			}, 5*time.Second, 250*time.Millisecond).Should(Succeed())
		})

		It("should fail when Server is missing", func() {
			s := &metalv1alpha1.Server{}
			Expect(k8sClient.Get(testCtx, types.NamespacedName{Name: testServerName, Namespace: namespace.Name}, s)).To(Succeed())
			Expect(k8sClient.Delete(testCtx, s)).To(Succeed())

			Eventually(func() error {
				ip := &ipamv1.IPAddress{}
				if err := k8sClient.Get(testCtx, types.NamespacedName{Name: testIPName, Namespace: namespace.Name}, ip); err != nil {
					return err
				}
				_, hasAnnotation := ip.Annotations[annotationDeviceKey]
				if hasAnnotation {
					return fmt.Errorf("expected no annotation but found one")
				}
				return nil
			}, 5*time.Second, 250*time.Millisecond).Should(Succeed())
		})

		It("should fail when ServerClaim is not bound to Server", func() {
			sc := &metalv1alpha1.ServerClaim{}
			Expect(k8sClient.Get(testCtx, types.NamespacedName{Name: testServerClaimName, Namespace: namespace.Name}, sc)).To(Succeed())
			Expect(k8sClient.Delete(testCtx, sc)).To(Succeed())

			sc = &metalv1alpha1.ServerClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testServerClaimName,
					Namespace: namespace.Name,
				},
				Spec: metalv1alpha1.ServerClaimSpec{
					Image: "gardenlinux:latest",
					Power: "On",
				},
			}
			Expect(k8sClient.Create(testCtx, sc)).To(Succeed())

			Eventually(func() error {
				ip := &ipamv1.IPAddress{}
				if err := k8sClient.Get(testCtx, types.NamespacedName{Name: testIPName, Namespace: namespace.Name}, ip); err != nil {
					return err
				}
				_, hasAnnotation := ip.Annotations[annotationDeviceKey]
				if hasAnnotation {
					return fmt.Errorf("expected no annotation but found one")
				}
				return nil
			}, 5*time.Second, 250*time.Millisecond).Should(Succeed())
		})

		It("should add finalizer on initial reconciliation", func() {
			newIPName := "test-ip-new-envtest"
			newIP := &ipamv1.IPAddress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      newIPName,
					Namespace: namespace.Name,
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

			Eventually(func() bool {
				ip := &ipamv1.IPAddress{}
				if err := k8sClient.Get(testCtx, types.NamespacedName{Name: newIPName, Namespace: namespace.Name}, ip); err != nil {
					return false
				}
				for _, finalizer := range ip.Finalizers {
					if finalizer == ipAddressFinalizer {
						return true
					}
				}
				return false
			}, 5*time.Second, 250*time.Millisecond).Should(BeTrue())

			ip := &ipamv1.IPAddress{}
			Expect(k8sClient.Get(testCtx, types.NamespacedName{Name: newIPName, Namespace: namespace.Name}, ip)).To(Succeed())
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

			Eventually(func() error {
				ip := &ipamv1.IPAddress{}
				if err := k8sClient.Get(testCtx, types.NamespacedName{Name: testIPName, Namespace: namespace.Name}, ip); err != nil {
					return err
				}
				_, hasAnnotation := ip.Annotations[annotationDeviceKey]
				if hasAnnotation {
					return fmt.Errorf("expected no annotation but found one")
				}
				return nil
			}, 5*time.Second, 250*time.Millisecond).Should(Succeed())
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

			Eventually(func() error {
				ip := &ipamv1.IPAddress{}
				if err := k8sClient.Get(testCtx, types.NamespacedName{Name: testIPName, Namespace: namespace.Name}, ip); err != nil {
					return err
				}
				_, hasAnnotation := ip.Annotations[annotationDeviceKey]
				if hasAnnotation {
					return fmt.Errorf("expected no annotation but found one")
				}
				return nil
			}, 5*time.Second, 250*time.Millisecond).Should(Succeed())
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
