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
		iPPoolName2       = "ippool-site-2a"
		iPPoolPrefix2     = "10.10.20.0/24"
		iPPoolPrefixMask2 = 24
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

		expectStatus := func(state argorav1alpha1.State, importCR types.NamespacedName, description string) {
			err := k8sClient.Get(ctx, importCR, ipPoolImport)
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
							ID:   2,
							Name: iPPoolPrefixSite2,
							Slug: iPPoolPrefixSite2,
						},
					},
				}, nil
			}
			return netBoxMock
		}

		expectIPPool := func(pool *ipamv1alpha2.GlobalInClusterIPPool, ipPoolName string, prefix string, mask int, excludePrefix string) {
			Expect(pool.Name).To(Equal(ipPoolName))
			Expect(pool.ObjectMeta.OwnerReferences).To(BeEmpty())

			Expect(pool.Spec.Addresses).To(Equal([]string{prefix}))
			Expect(pool.Spec.Prefix).To(Equal(mask))
			if excludePrefix != "" {
				Expect(pool.Spec.ExcludedAddresses).To(Equal([]string{excludePrefix}))
			}
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

			expectStatus(argorav1alpha1.Ready, typeNamespacedIPPoolImportName, "")
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

			// Fetch and validate first pool
			pool1 := &ipamv1alpha2.GlobalInClusterIPPool{}
			err = k8sClient.Get(ctx, typeNamespacedIPPoolName1, pool1)
			Expect(err).ToNot(HaveOccurred())
			expectIPPool(pool1, iPPoolName1, iPPoolPrefix1, iPPoolPrefixMask1, "")

			// Fetch and validate second pool
			pool2 := &ipamv1alpha2.GlobalInClusterIPPool{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: iPPoolName2, Namespace: resourceNamespace}, pool2)
			Expect(err).ToNot(HaveOccurred())
			expectIPPool(pool2, iPPoolName2, iPPoolPrefix2, iPPoolPrefixMask2, "")

			// Check the IPPoolImport CR status (not the IP pools)
			expectStatus(argorav1alpha1.Ready, typeNamespacedIPPoolImportName, "")
		})

		It("should successfully create a GlobalInClusterIPPool CR with Excluded Addresses field", func() {
			// given
			netBoxMock := prepareNetboxMock()

			transitPrefix := "10.10.30.0/26"
			transitPrefixMask := 26
			transitPrefixExclude := "10.10.30.0/27"
			transitPrefixExcludeMask := 27
			transitRole := "transit"
			transitSite := "site-3a"
			transitNamePrefix := "transit"
			transitPoolName := "transit-site-3a"
			excludeMask := transitPrefixExcludeMask

			// Create a dedicated CR for this test
			testCR := &argorav1alpha1.IPPoolImport{
				ObjectMeta: metav1.ObjectMeta{
					Name:      transitPoolName,
					Namespace: resourceNamespace,
				},
				Spec: argorav1alpha1.IPPoolImportSpec{
					IPPools: []*argorav1alpha1.IPPoolSelector{
						{
							NamePrefix:  transitNamePrefix,
							ExcludeMask: &excludeMask,
							Region:      regionName,
							Role:        transitRole,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, testCR)).To(Succeed())

			// Clean up after test
			defer func() {
				Expect(k8sClient.Delete(ctx, testCR)).To(Succeed())
			}()

			netBoxMock.IPAMMock.(*mock.IPAMMock).GetPrefixesByRegionRoleFunc = func(region, role string) ([]models.Prefix, error) {
				Expect(region).To(Equal(regionName))
				Expect(role).To(Equal(transitRole))
				return []models.Prefix{
					{
						ID:     1,
						Prefix: transitPrefix,
						Site: models.Site{
							ID:   1,
							Name: transitSite,
							Slug: transitSite,
						},
					},
				}, nil
			}
			controllerReconciler := createIPPoolImportReconciler(netBoxMock, fileReaderMock)

			// when
			By("reconciling IPPoolImport CR")
			res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: transitPoolName, Namespace: resourceNamespace}})

			// then
			Expect(err).ToNot(HaveOccurred())
			Expect(res.RequeueAfter).To(Equal(reconcileInterval))

			pool := &ipamv1alpha2.GlobalInClusterIPPool{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: transitPoolName}, pool)
			Expect(err).ToNot(HaveOccurred())

			expectIPPool(pool, transitPoolName, transitPrefix, transitPrefixMask, transitPrefixExclude)
			expectStatus(argorav1alpha1.Ready, types.NamespacedName{Name: transitPoolName, Namespace: resourceNamespace}, "")
		})

		It("should return an error if excluded address mask is larger than prefix mask", func() {
			// given
			netBoxMock := prepareNetboxMock()

			transitPrefix := "10.10.50.0/24"
			transitPrefixExcludeMask := 24
			transitRole := "transit"
			transitSite := "site-5a"
			transitNamePrefix := "transit"
			transitPoolName := "transit-site-5a"
			excludeMask := transitPrefixExcludeMask

			// Create a dedicated CR for this test
			testCR := &argorav1alpha1.IPPoolImport{
				ObjectMeta: metav1.ObjectMeta{
					Name:      transitPoolName,
					Namespace: resourceNamespace,
				},
				Spec: argorav1alpha1.IPPoolImportSpec{
					IPPools: []*argorav1alpha1.IPPoolSelector{
						{
							NamePrefix:  transitNamePrefix,
							ExcludeMask: &excludeMask,
							Region:      regionName,
							Role:        transitRole,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, testCR)).To(Succeed())

			// Clean up after test
			defer func() {
				Expect(k8sClient.Delete(ctx, testCR)).To(Succeed())
			}()

			netBoxMock.IPAMMock.(*mock.IPAMMock).GetPrefixesByRegionRoleFunc = func(region, role string) ([]models.Prefix, error) {
				Expect(region).To(Equal(regionName))
				Expect(role).To(Equal(transitRole))
				return []models.Prefix{
					{
						ID:     1,
						Prefix: transitPrefix,
						Site: models.Site{
							ID:   1,
							Name: transitSite,
							Slug: transitSite,
						},
					},
				}, nil
			}
			controllerReconciler := createIPPoolImportReconciler(netBoxMock, fileReaderMock)

			// when
			By("reconciling IPPoolImport CR")
			res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: transitPoolName, Namespace: resourceNamespace}})

			// then
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("excludeMask (24) must be longer than prefix mask (24) for prefix 10.10.50.0/24"))
			Expect(res.RequeueAfter).To(Equal(0 * time.Second))

			expectStatus(argorav1alpha1.Error, types.NamespacedName{
				Name:      transitPoolName,
				Namespace: resourceNamespace},
				"unable to reconcile prefix 10.10.50.0/24 on ippool transit: excludeMask (24) must be longer than prefix mask (24) for prefix 10.10.50.0/24")
		})

		It("should successfully create a GlobalInClusterIPPool CR with Compute-Specific Name", func() {
			// given
			netBoxMock := prepareNetboxMock()

			computePrefix := "10.10.40.0/24"
			computeRole := "test-kubernetes-compute-transit"
			computeSite := "test-eu-1a"
			computeRegion := "test-eu-1"
			computeNamePrefix := "compute"
			computePoolName := "compute-a0-test-eu-1"

			// Create a dedicated CR for this test
			testCR := &argorav1alpha1.IPPoolImport{
				ObjectMeta: metav1.ObjectMeta{
					Name:      computePoolName,
					Namespace: resourceNamespace,
				},
				Spec: argorav1alpha1.IPPoolImportSpec{
					IPPools: []*argorav1alpha1.IPPoolSelector{
						{
							NamePrefix: computeNamePrefix,
							Region:     computeRegion,
							Role:       computeRole,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, testCR)).To(Succeed())

			// Clean up after test
			defer func() {
				Expect(k8sClient.Delete(ctx, testCR)).To(Succeed())
			}()

			netBoxMock.IPAMMock.(*mock.IPAMMock).GetPrefixesByRegionRoleFunc = func(region, role string) ([]models.Prefix, error) {
				Expect(region).To(Equal(computeRegion))
				Expect(role).To(Equal(computeRole))
				return []models.Prefix{
					{
						ID:     1,
						Prefix: computePrefix,
						Role: models.Role{
							ID:   1,
							Name: computeRole,
							Slug: computeRole,
						},
						Site: models.Site{
							ID:   1,
							Name: computeSite,
							Slug: computeSite,
							Region: models.NestedRegion{
								ID:   1,
								Name: computeRegion,
								Slug: computeRegion,
							},
						},
						Vlan: models.NestedVLAN{
							ID:   1,
							Name: "Test Compute1 K8s Peering",
						},
					},
				}, nil
			}
			controllerReconciler := createIPPoolImportReconciler(netBoxMock, fileReaderMock)

			// when
			By("reconciling IPPoolImport CR")
			res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: computePoolName, Namespace: resourceNamespace}})

			// then
			Expect(err).ToNot(HaveOccurred())
			Expect(res.RequeueAfter).To(Equal(reconcileInterval))

			pool := &ipamv1alpha2.GlobalInClusterIPPool{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: computePoolName}, pool)
			Expect(err).ToNot(HaveOccurred())

			expectIPPool(pool, computePoolName, computePrefix, iPPoolPrefixMask1, "")
			expectStatus(argorav1alpha1.Ready, types.NamespacedName{Name: computePoolName, Namespace: resourceNamespace}, "")
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

			expectStatus(argorav1alpha1.Error, typeNamespacedIPPoolImportName, "unable to read credentials.json: error")
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

			expectStatus(argorav1alpha1.Error, typeNamespacedIPPoolImportName, "unable to reload netbox")
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

			expectStatus(argorav1alpha1.Error, typeNamespacedIPPoolImportName, "unable to import prefix: unable to find prefixes")
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
