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

		namePrefix = "ippool"
		regionName = "region1"
		roleName   = "role1"

		iPPoolName1       = "ippool-site-1a"
		iPPoolPrefix1     = "10.10.10.0/24"
		iPPoolPrefixMask1 = 24
		iPPoolPrefixSite1 = "site-1a"

		iPPoolName2       = "ippool-site-2a"
		iPPoolPrefix2     = "10.10.20.0/25"
		iPPoolPrefixMask2 = 25
		iPPoolPrefixSite2 = "site-2a"
	)

	var (
		excludeMask = 0
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

		expectIPPool := func(pool *ipamv1alpha2.GlobalInClusterIPPool, ipPoolName string, prefix string, mask int, excludePrefix []string) {
			Expect(pool.Name).To(Equal(ipPoolName))
			Expect(pool.ObjectMeta.OwnerReferences).To(BeEmpty())

			Expect(pool.Spec.Addresses).To(Equal([]string{prefix}))
			Expect(pool.Spec.Prefix).To(Equal(mask))
			if excludePrefix != nil {
				Expect(pool.Spec.ExcludedAddresses).To(ConsistOf(excludePrefix))
			}
		}

		BeforeEach(func() {
			By("create IPPoolImport CR")
			excludeMask = 0
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
								NamePrefix: namePrefix,
								Region:     regionName,
								Role:       roleName,
							},
						},
					},
				}
				if excludeMask != 0 {
					resource.Spec.IPPools[0].ExcludeMask = &excludeMask
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			err := k8sClient.Get(ctx, typeNamespacedIPPoolImportName, ipPoolImport)
			Expect(err).ToNot(HaveOccurred())

			globalInClusterIPPoolList := &ipamv1alpha2.GlobalInClusterIPPoolList{}
			err = k8sClient.List(ctx, globalInClusterIPPoolList)
			Expect(err).ToNot(HaveOccurred())

			By("delete created GlobalInClusterIPPools")
			for _, pool := range globalInClusterIPPoolList.Items {
				Expect(k8sClient.Delete(ctx, &pool)).To(Succeed())
			}

			By("delete IPPoolImport CR")
			Expect(k8sClient.Delete(ctx, ipPoolImport)).To(Succeed())
		})

		It("should successfully reconcile and create GlobalInClusterIPPool CRs", func() {
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

			// Fetch and validate first pool
			pool1 := &ipamv1alpha2.GlobalInClusterIPPool{}
			err = k8sClient.Get(ctx, typeNamespacedIPPoolName1, pool1)
			Expect(err).ToNot(HaveOccurred())
			expectIPPool(pool1, iPPoolName1, iPPoolPrefix1, iPPoolPrefixMask1, nil)

			// Fetch and validate second pool
			pool2 := &ipamv1alpha2.GlobalInClusterIPPool{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: iPPoolName2, Namespace: resourceNamespace}, pool2)
			Expect(err).ToNot(HaveOccurred())
			expectIPPool(pool2, iPPoolName2, iPPoolPrefix2, iPPoolPrefixMask2, nil)

			expectStatus(argorav1alpha1.Ready, typeNamespacedIPPoolImportName, "")
		})

		It("should successfully create a GlobalInClusterIPPool CR with Name Override", func() {
			// given
			netBoxMock := prepareNetboxMock()

			By("update IPPoolImport CR to add Name Override")
			err := k8sClient.Get(ctx, typeNamespacedIPPoolImportName, ipPoolImport)
			Expect(err).ToNot(HaveOccurred())

			iPPoolName := "ippool-override-name"
			ipPoolImport.Spec.IPPools[0].NameOverride = iPPoolName
			ipPoolImport.Spec.IPPools[0].NamePrefix = ""
			Expect(k8sClient.Update(ctx, ipPoolImport)).To(Succeed())

			controllerReconciler := createIPPoolImportReconciler(netBoxMock, fileReaderMock)

			// when
			By("reconciling IPPoolImport CR")
			res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedIPPoolImportName})

			// then
			Expect(err).ToNot(HaveOccurred())
			Expect(res.RequeueAfter).To(Equal(reconcileInterval))

			Expect(netBoxMock.IPAMMock.(*mock.IPAMMock).GetPrefixesByRegionRoleCalls).To(Equal(1))

			pool := &ipamv1alpha2.GlobalInClusterIPPool{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      iPPoolName,
				Namespace: resourceNamespace,
			}, pool)
			Expect(err).ToNot(HaveOccurred())

			expectIPPool(pool, iPPoolName, iPPoolPrefix1, iPPoolPrefixMask1, nil)
			expectStatus(argorav1alpha1.Ready, typeNamespacedIPPoolImportName, "")
		})

		It("should successfully create a GlobalInClusterIPPool CR with Excluded Mask field", func() {
			// given
			netBoxMock := prepareNetboxMock()
			excludeMask = 27
			excludePrefix := "10.10.10.0/27"

			By("update IPPoolImport CR to add ExcludeMask")
			err := k8sClient.Get(ctx, typeNamespacedIPPoolImportName, ipPoolImport)
			Expect(err).ToNot(HaveOccurred())

			ipPoolImport.Spec.IPPools[0].ExcludeMask = &excludeMask
			Expect(k8sClient.Update(ctx, ipPoolImport)).To(Succeed())

			controllerReconciler := createIPPoolImportReconciler(netBoxMock, fileReaderMock)

			// when
			By("reconciling IPPoolImport CR")
			res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedIPPoolImportName})

			// then
			Expect(err).ToNot(HaveOccurred())
			Expect(res.RequeueAfter).To(Equal(reconcileInterval))

			Expect(netBoxMock.IPAMMock.(*mock.IPAMMock).GetPrefixesByRegionRoleCalls).To(Equal(1))

			pool := &ipamv1alpha2.GlobalInClusterIPPool{}
			err = k8sClient.Get(ctx, typeNamespacedIPPoolName1, pool)
			Expect(err).ToNot(HaveOccurred())

			expectIPPool(pool, iPPoolName1, iPPoolPrefix1, iPPoolPrefixMask1, []string{excludePrefix})
			expectStatus(argorav1alpha1.Ready, typeNamespacedIPPoolImportName, "")
		})

		It("should successfully create a GlobalInClusterIPPool CR with Excluded Addresses field", func() {
			// given
			netBoxMock := prepareNetboxMock()
			excludedAddress1 := "10.10.10.29"
			excludedAddress2 := "10.10.10.30"
			excludedAddresses := []string{excludedAddress1, excludedAddress2}

			By("update IPPoolImport CR to add ExcludedAddress")
			err := k8sClient.Get(ctx, typeNamespacedIPPoolImportName, ipPoolImport)
			Expect(err).ToNot(HaveOccurred())

			ipPoolImport.Spec.IPPools[0].ExcludeMask = nil
			ipPoolImport.Spec.IPPools[0].ExcludedAddresses = excludedAddresses
			Expect(k8sClient.Update(ctx, ipPoolImport)).To(Succeed())

			controllerReconciler := createIPPoolImportReconciler(netBoxMock, fileReaderMock)

			// when
			By("reconciling IPPoolImport CR")
			res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedIPPoolImportName})

			// then
			Expect(err).ToNot(HaveOccurred())
			Expect(res.RequeueAfter).To(Equal(reconcileInterval))

			Expect(netBoxMock.IPAMMock.(*mock.IPAMMock).GetPrefixesByRegionRoleCalls).To(Equal(1))

			pool := &ipamv1alpha2.GlobalInClusterIPPool{}
			err = k8sClient.Get(ctx, typeNamespacedIPPoolName1, pool)
			Expect(err).ToNot(HaveOccurred())

			expectIPPool(pool, iPPoolName1, iPPoolPrefix1, iPPoolPrefixMask1, excludedAddresses)
			expectStatus(argorav1alpha1.Ready, typeNamespacedIPPoolImportName, "")
		})

		It("should successfully create a GlobalInClusterIPPool CR with Excluded Addresses and Excluded Last N Addresses fields", func() {
			// given
			netBoxMock := prepareNetboxMock()
			excludeMask = 27
			excludePrefix := "10.10.10.0/27"
			excludedLastNAddress := 3
			excludedLastAddresses := []string{"10.10.10.253", "10.10.10.254", "10.10.10.255"}
			excludePrefixes := append(excludedLastAddresses, excludePrefix)

			By("update IPPoolImport CR to add ExcludedLastNAddresses")
			err := k8sClient.Get(ctx, typeNamespacedIPPoolImportName, ipPoolImport)
			Expect(err).ToNot(HaveOccurred())

			ipPoolImport.Spec.IPPools[0].ExcludeMask = &excludeMask
			ipPoolImport.Spec.IPPools[0].ExcludeLastNAddresses = &excludedLastNAddress
			Expect(k8sClient.Update(ctx, ipPoolImport)).To(Succeed())

			controllerReconciler := createIPPoolImportReconciler(netBoxMock, fileReaderMock)

			// when
			By("reconciling IPPoolImport CR")
			res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedIPPoolImportName})

			// then
			Expect(err).ToNot(HaveOccurred())
			Expect(res.RequeueAfter).To(Equal(reconcileInterval))

			Expect(netBoxMock.IPAMMock.(*mock.IPAMMock).GetPrefixesByRegionRoleCalls).To(Equal(1))

			pool := &ipamv1alpha2.GlobalInClusterIPPool{}
			err = k8sClient.Get(ctx, typeNamespacedIPPoolName1, pool)
			Expect(err).ToNot(HaveOccurred())

			expectIPPool(pool, iPPoolName1, iPPoolPrefix1, iPPoolPrefixMask1, excludePrefixes)
			expectStatus(argorav1alpha1.Ready, typeNamespacedIPPoolImportName, "")
		})

		It("should gracefully handle Excluded Last N Addresses larger than usable Prefix size", func() {
			// given
			ctx := context.Background()
			typeNamespacedIPPoolImportName := types.NamespacedName{
				Name:      resourceName,
				Namespace: resourceNamespace,
			}
			typeNamespacedIPPoolName := types.NamespacedName{
				Name:      "ippool-site-1a",
				Namespace: resourceNamespace,
			}

			// Mock a /30 prefix to limit total addresses to 4
			netBoxMock := prepareNetboxMock()
			netBoxMock.IPAMMock.(*mock.IPAMMock).GetPrefixesByRegionRoleFunc = func(region, role string) ([]models.Prefix, error) {
				Expect(region).To(Equal(regionName))
				Expect(role).To(Equal(roleName))
				return []models.Prefix{
					{
						ID:     1,
						Prefix: "10.10.10.0/30",
						Site: models.Site{
							ID:   1,
							Name: iPPoolPrefixSite1,
							Slug: iPPoolPrefixSite1,
						},
					},
				}, nil
			}

			// Request excluding last 5 addresses (larger than /30 capacity)
			excludedLastNAddress := 5
			Expect(k8sClient.Get(ctx, typeNamespacedIPPoolImportName, ipPoolImport)).To(Succeed())
			ipPoolImport.Spec.IPPools[0].ExcludeMask = nil
			ipPoolImport.Spec.IPPools[0].ExcludedAddresses = nil
			ipPoolImport.Spec.IPPools[0].ExcludeLastNAddresses = &excludedLastNAddress
			Expect(k8sClient.Update(ctx, ipPoolImport)).To(Succeed())

			controllerReconciler := createIPPoolImportReconciler(netBoxMock, fileReaderMock)

			// when
			res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedIPPoolImportName})

			// then
			Expect(err).ToNot(HaveOccurred())
			Expect(res.RequeueAfter).To(Equal(reconcileInterval))
			Expect(netBoxMock.IPAMMock.(*mock.IPAMMock).GetPrefixesByRegionRoleCalls).To(Equal(1))

			pool := &ipamv1alpha2.GlobalInClusterIPPool{}
			Expect(k8sClient.Get(ctx, typeNamespacedIPPoolName, pool)).To(Succeed())

			// Only last 4 addresses of /30 are excluded
			Expect(pool.Spec.Addresses).To(Equal([]string{"10.10.10.0/30"}))
			Expect(pool.Spec.Prefix).To(Equal(30))
			Expect(pool.Spec.Gateway).To(Equal("10.10.10.1"))
			Expect(pool.Spec.ExcludedAddresses).To(ConsistOf([]string{"10.10.10.0", "10.10.10.1", "10.10.10.2", "10.10.10.3"}))

			expectStatus(argorav1alpha1.Ready, typeNamespacedIPPoolImportName, "")
		})

		It("should successfully create a GlobalInClusterIPPool CR with Excluded Addresses and Excluded Mask fields", func() {
			// given
			netBoxMock := prepareNetboxMock()
			excludeMask = 27
			excludePrefix := "10.10.10.0/27"
			excludedAddress := "10.10.10.30"
			excludePrefixes := []string{excludePrefix, excludedAddress}

			By("update IPPoolImport CR to add ExcludedAddress")
			err := k8sClient.Get(ctx, typeNamespacedIPPoolImportName, ipPoolImport)
			Expect(err).ToNot(HaveOccurred())

			ipPoolImport.Spec.IPPools[0].ExcludeMask = &excludeMask
			ipPoolImport.Spec.IPPools[0].ExcludedAddresses = []string{excludedAddress}
			Expect(k8sClient.Update(ctx, ipPoolImport)).To(Succeed())

			controllerReconciler := createIPPoolImportReconciler(netBoxMock, fileReaderMock)

			// when
			By("reconciling IPPoolImport CR")
			res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedIPPoolImportName})

			// then
			Expect(err).ToNot(HaveOccurred())
			Expect(res.RequeueAfter).To(Equal(reconcileInterval))

			Expect(netBoxMock.IPAMMock.(*mock.IPAMMock).GetPrefixesByRegionRoleCalls).To(Equal(1))

			pool := &ipamv1alpha2.GlobalInClusterIPPool{}
			err = k8sClient.Get(ctx, typeNamespacedIPPoolName1, pool)
			Expect(err).ToNot(HaveOccurred())

			expectIPPool(pool, iPPoolName1, iPPoolPrefix1, iPPoolPrefixMask1, excludePrefixes)
			expectStatus(argorav1alpha1.Ready, typeNamespacedIPPoolImportName, "")
		})

		It("should return an error if excluded address mask is equal/larger than prefix mask", func() {
			// given
			netBoxMock := prepareNetboxMock()
			excludeMask = 24

			By("update IPPoolImport CR to add ExcludeMask")
			err := k8sClient.Get(ctx, typeNamespacedIPPoolImportName, ipPoolImport)
			Expect(err).ToNot(HaveOccurred())

			ipPoolImport.Spec.IPPools[0].ExcludeMask = &excludeMask
			Expect(k8sClient.Update(ctx, ipPoolImport)).To(Succeed())

			controllerReconciler := createIPPoolImportReconciler(netBoxMock, fileReaderMock)

			// when
			By("reconciling IPPoolImport CR")
			res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedIPPoolImportName})

			Expect(netBoxMock.IPAMMock.(*mock.IPAMMock).GetPrefixesByRegionRoleCalls).To(Equal(1))

			// then
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("excludeMask (24) must be longer than prefix mask (24) for prefix 10.10.10.0/24"))
			Expect(res.RequeueAfter).To(Equal(0 * time.Second))

			expectStatus(argorav1alpha1.Error, types.NamespacedName{
				Name:      resourceName,
				Namespace: resourceNamespace},
				"unable to reconcile prefix 10.10.10.0/24 on ippool ippool: excludeMask (24) must be longer than prefix mask (24) for prefix 10.10.10.0/24")
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

			// Create IPPoolImport CR for compute name case
			computeIPPoolImportCR := &argorav1alpha1.IPPoolImport{
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
			Expect(k8sClient.Create(ctx, computeIPPoolImportCR)).To(Succeed())

			// Clean up after test
			defer func() {
				Expect(k8sClient.Delete(ctx, computeIPPoolImportCR)).To(Succeed())
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
			res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: computePoolName, Namespace: resourceNamespace}})

			Expect(netBoxMock.IPAMMock.(*mock.IPAMMock).GetPrefixesByRegionRoleCalls).To(Equal(1))

			// then
			Expect(err).ToNot(HaveOccurred())
			Expect(res.RequeueAfter).To(Equal(reconcileInterval))

			pool := &ipamv1alpha2.GlobalInClusterIPPool{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: computePoolName}, pool)
			Expect(err).ToNot(HaveOccurred())

			expectIPPool(pool, computePoolName, computePrefix, iPPoolPrefixMask1, nil)
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
