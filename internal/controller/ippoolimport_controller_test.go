// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"time"

	ipamv1alpha2 "sigs.k8s.io/cluster-api-ipam-provider-in-cluster/api/v1alpha2"

	"github.com/sapcc/go-netbox-go/models"

	"github.com/sapcc/argora/internal/controller/mock"
	"github.com/sapcc/argora/internal/credentials"
	"github.com/sapcc/argora/internal/status"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	argorav1alpha1 "github.com/sapcc/argora/api/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("IPPoolImport Controller", func() {
	const (
		resourceName      = "test-resource"
		resourceNamespace = "default"
		regionName        = "region1"
		roleName          = "role1"
		iPPoolName1       = "ippool-site-1a"
		iPPoolPrefix1     = "10.10.10.0/24"
		iPPoolPrefixMask1 = 24
		iPPoolPrefixSite1 = "site-1a"
		iPPoolPrefix2     = "10.10.20.0/24"
		iPPoolPrefixSite2 = "site-2a"
	)

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

		typeNamespacedIPPoolImportName := types.NamespacedName{
			Name:      resourceName,
			Namespace: resourceNamespace,
		}
		typeNamespacedIPPoolName1 := types.NamespacedName{
			Name:      iPPoolName1,
			Namespace: resourceNamespace,
		}
		ipPoolImport := &argorav1alpha1.IPPoolImport{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: resourceNamespace,
			},
		}

		expectStatus := func(state argorav1alpha1.State, description string) {
			err := k8sClient.Get(ctx, typeNamespacedIPPoolImportName, ipPoolImport)
			Expect(err).ToNot(HaveOccurred())
			Expect(ipPoolImport.Status.State).To(Equal(state))
			Expect(ipPoolImport.Status.Description).To(Equal(description))
			Expect(ipPoolImport.Status.Conditions).ToNot(BeNil())
			Expect(*ipPoolImport.Status.Conditions).To(HaveLen(1))
			if state == argorav1alpha1.Ready {
				Expect((*ipPoolImport.Status.Conditions)[0].Type).To(Equal(string(argorav1alpha1.ConditionTypeReady)))
				Expect((*ipPoolImport.Status.Conditions)[0].Status).To(Equal(metav1.ConditionTrue))
				Expect((*ipPoolImport.Status.Conditions)[0].Reason).To(Equal(string(argorav1alpha1.ConditionReasonIPPoolImportSucceeded)))
				Expect((*ipPoolImport.Status.Conditions)[0].Message).To(Equal(argorav1alpha1.ConditionReasonIPPoolImportSucceededMessage))
			} else {
				Expect((*ipPoolImport.Status.Conditions)[0].Type).To(Equal(string(argorav1alpha1.ConditionTypeReady)))
				Expect((*ipPoolImport.Status.Conditions)[0].Status).To(Equal(metav1.ConditionFalse))
				Expect((*ipPoolImport.Status.Conditions)[0].Reason).To(Equal(string(argorav1alpha1.ConditionReasonIPPoolImportFailed)))
				Expect((*ipPoolImport.Status.Conditions)[0].Message).To(Equal(argorav1alpha1.ConditionReasonIPPoolImportFailedMessage))
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

			netBoxMock.IPAMMock.(*mock.IPAMMock).GetPrefixesByRegionRoleFunc = func(region, role string) ([]models.Prefix, error) {
				Expect(region).To(Equal(regionName))
				Expect(role).To(Equal(roleName))
				return []models.Prefix{
					{
						ID:     1,
						Prefix: iPPoolPrefix1,
						Site: models.Site{
							ID:   1,
							Name: iPPoolPrefixSite1,
							Slug: iPPoolPrefixSite1,
						},
					},
					{
						ID:     2,
						Prefix: iPPoolPrefix2,
						Site: models.Site{
							ID:   1,
							Name: iPPoolPrefixSite2,
							Slug: iPPoolPrefixSite2,
						},
					},
				}, nil
			}
			return netBoxMock
		}

		expectIPPool := func(pool *ipamv1alpha2.GlobalInClusterIPPool, ipPoolName string, prefix string, mask int) {
			Expect(pool.Name).To(Equal(ipPoolName))
			Expect(pool.ObjectMeta.OwnerReferences).To(BeEmpty())

			Expect(pool.Spec.Addresses).To(Equal([]string{prefix}))
			Expect(pool.Spec.Prefix).To(Equal(mask))
		}

		BeforeEach(func() {
			By("create IPPoolImport CR")
			err := k8sClient.Get(ctx, typeNamespacedIPPoolImportName, ipPoolImport)
			if err != nil && apierrors.IsNotFound(err) {
				resource := &argorav1alpha1.IPPoolImport{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: resourceNamespace,
					},
					Spec: argorav1alpha1.IPPoolImportSpec{
						IPPools: []*argorav1alpha1.IPPoolSelector{
							{
								NamePrefix: "ippool",
								Region:     regionName,
								Role:       roleName,
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			err := k8sClient.Get(ctx, typeNamespacedIPPoolImportName, ipPoolImport)
			Expect(err).ToNot(HaveOccurred())

			By("delete IPPoolImport CR")
			Expect(k8sClient.Delete(ctx, ipPoolImport)).To(Succeed())
		})

		It("should successfully reconcile the CR", func() {
			// given
			netBoxMock := prepareNetboxMock()
			controllerReconciler := createIPPoolImportReconciler(netBoxMock, fileReaderMock)

			// when
			By("reconciling IPPoolImport CR")
			res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedIPPoolImportName})

			// then
			Expect(err).ToNot(HaveOccurred())
			Expect(res.RequeueAfter).To(Equal(reconcileInterval))

			Expect(netBoxMock.IPAMMock.(*mock.IPAMMock).GetPrefixesByRegionRoleCalls).To(Equal(1))

			expectStatus(argorav1alpha1.Ready, "")
		})

		It("should successfully create a GlobalInClusterIPPool CR", func() {
			// given
			netBoxMock := prepareNetboxMock()
			netBoxMock.IPAMMock.(*mock.IPAMMock).GetPrefixesByRegionRoleFunc = func(region, role string) ([]models.Prefix, error) {
				Expect(region).To(Equal(regionName))
				Expect(role).To(Equal(roleName))
				return []models.Prefix{
					{
						ID:     1,
						Prefix: iPPoolPrefix1,
						Site: models.Site{
							ID:   1,
							Name: iPPoolPrefixSite1,
							Slug: iPPoolPrefixSite1,
						},
					},
				}, nil
			}
			controllerReconciler := createIPPoolImportReconciler(netBoxMock, fileReaderMock)

			// when
			By("reconciling IPPoolImport CR")
			res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedIPPoolImportName})

			// then
			Expect(err).ToNot(HaveOccurred())
			Expect(res.RequeueAfter).To(Equal(reconcileInterval))

			pool := &ipamv1alpha2.GlobalInClusterIPPool{}
			err = k8sClient.Get(ctx, typeNamespacedIPPoolName1, pool)
			Expect(err).ToNot(HaveOccurred())

			expectIPPool(pool, iPPoolName1, iPPoolPrefix1, iPPoolPrefixMask1)
			expectStatus(argorav1alpha1.Ready, "")
		})

		It("should return an error if credentials reload fails", func() {
			// given
			netBoxMock := prepareNetboxMock()
			fileReaderMockToError := &mock.FileReaderMock{
				FileContent: make(map[string]string),
				ReturnError: true,
			}
			controllerReconciler := createIPPoolImportReconciler(netBoxMock, fileReaderMockToError)

			// when
			By("reconciling IPPoolImport CR")
			res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedIPPoolImportName})

			// then
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unable to read credentials.json: error"))
			Expect(res.RequeueAfter).To(Equal(0 * time.Second))

			expectStatus(argorav1alpha1.Error, "unable to read credentials.json: error")
		})

		It("should return an error if netbox reload fails", func() {
			// given
			controllerReconciler := createIPPoolImportReconciler(&mock.NetBoxMock{ReturnError: true}, fileReaderMock)

			// when
			By("reconciling IPPoolImport CR")
			res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedIPPoolImportName})

			// then
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unable to reload netbox"))
			Expect(res.RequeueAfter).To(Equal(0 * time.Second))

			expectStatus(argorav1alpha1.Error, "unable to reload netbox")
		})

		It("should return an error if GetPrefixesByRegionRole fails", func() {
			// given
			netBoxMock := prepareNetboxMock()
			netBoxMock.IPAMMock.(*mock.IPAMMock).GetPrefixesByRegionRoleFunc = func(region, role string) ([]models.Prefix, error) {
				Expect(region).To(Equal("region1"))
				Expect(role).To(Equal("role1"))
				return nil, errors.New("unable to find prefixes")
			}

			controllerReconciler := createIPPoolImportReconciler(netBoxMock, fileReaderMock)

			// when
			By("reconciling IPPoolImport CR")
			res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedIPPoolImportName})

			// then
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unable to find prefixes"))
			Expect(res.RequeueAfter).To(Equal(0 * time.Second))

			expectStatus(argorav1alpha1.Error, "unable to import prefix: unable to find prefixes")
		})
	})
})

func createIPPoolImportReconciler(netBoxMock *mock.NetBoxMock, fileReaderMock credentials.FileReader) *IPPoolImportReconciler {
	return &IPPoolImportReconciler{
		k8sClient:         k8sClient,
		scheme:            k8sClient.Scheme(),
		statusHandler:     status.NewIPPoolImportStatusHandler(k8sClient),
		netBox:            netBoxMock,
		credentials:       credentials.NewDefaultCredentials(fileReaderMock),
		reconcileInterval: reconcileInterval,
	}
}
