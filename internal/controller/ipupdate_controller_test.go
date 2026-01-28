// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"

	metalv1alpha1 "github.com/ironcore-dev/metal-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sapcc/go-netbox-go/models"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ipamv1 "sigs.k8s.io/cluster-api/api/ipam/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/sapcc/argora/internal/controller/mock"
	"github.com/sapcc/argora/internal/credentials"
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
					GetDeviceByNameFunc: func(deviceName string) (*models.Device, error) {
						return &models.Device{
							ID:   1,
							Name: "node013-ap002",
						}, nil
					},
					GetInterfacesForDeviceFunc: func(device *models.Device) ([]models.Interface, error) {
						return []models.Interface{
							{
								Name: "LAG0",
								Type: models.InterfaceType{Value: "lag"},
							},
							{
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
				IPAMMock:   &mock.IPAMMock{},
				ExtrasMock: &mock.ExtrasMock{},
			}

			return netBoxMock
		}

		BeforeEach(func() {
			By("create Server CR")
			serverName := "test-server"
			server := &metalv1alpha1.Server{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: serverName, Namespace: resourceNamespace}, server)
			if err != nil && apierrors.IsNotFound(err) {
				resource := &metalv1alpha1.Server{
					ObjectMeta: metav1.ObjectMeta{
						Name:      serverName,
						Namespace: resourceNamespace,
					},
					Spec: metalv1alpha1.ServerSpec{
						SystemUUID: "test_uuid",
						BMCRef: &v1.LocalObjectReference{
							Name: "node013-ap002",
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}

			By("create ServerClaim CR")
			serverClaimName := "test-server-claim"
			serverClaim := &metalv1alpha1.ServerClaim{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: serverClaimName, Namespace: resourceNamespace}, serverClaim)
			if err != nil && apierrors.IsNotFound(err) {
				resource := &metalv1alpha1.ServerClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      serverClaimName,
						Namespace: resourceNamespace,
					},
					Spec: metalv1alpha1.ServerClaimSpec{
						ServerRef: &v1.LocalObjectReference{
							Name: serverName,
						},
						Image: "gardenlinux:latest",
						Power: "On",
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}

			By("create IPAM IPAddressClaim CR")
			ipClaimName := "test-claim"
			ipClaim := &ipamv1.IPAddressClaim{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: ipClaimName, Namespace: resourceNamespace}, ipClaim)
			if err != nil && apierrors.IsNotFound(err) {
				resource := &ipamv1.IPAddressClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      ipClaimName,
						Namespace: resourceNamespace,
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "metal.ironcore.dev/v1alpha1",
								Kind:       "ServerClaim",
								Name:       serverClaimName,
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
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}

			By("create IPAM IPAddress CR")
			err = k8sClient.Get(ctx, typeNamespacedUpdateName, ipAddress)
			if err != nil && apierrors.IsNotFound(err) {
				resource := &ipamv1.IPAddress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: resourceNamespace,
					},
					Spec: ipamv1.IPAddressSpec{
						Address: "192.168.1.100",
						PoolRef: ipamv1.IPPoolReference{
							Name:     "test-ippool",
							APIGroup: "ipam.cluster.x-k8s.io",
							Kind:     "IPPool",
						},
						ClaimRef: ipamv1.IPAddressClaimReference{
							Name: "test-claim",
						},
						Prefix: ptr.To(int32(24)),
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			By("delete IPAM IPAddress CR")
			err := k8sClient.Get(ctx, typeNamespacedUpdateName, ipAddress)
			if err == nil {
				Expect(k8sClient.Delete(ctx, ipAddress)).To(Succeed())
			}
		})

		It("should successfully reconcile the CR", func() {
			// given
			netBoxMock := prepareNetboxMock()
			controllerReconciler := createIPUpdateReconciler(netBoxMock, fileReaderMock)

			// when
			By("reconciling IPAM IPAddress CR")
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedUpdateName})

			// then
			Expect(err).ToNot(HaveOccurred())
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
