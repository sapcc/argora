// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	metalv1alpha1 "github.com/ironcore-dev/metal-operator/api/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/sapcc/go-netbox-go/models"

	argorav1alpha1 "github.com/sapcc/argora/api/v1alpha1"
	"github.com/sapcc/argora/internal/controller/mock"
	"github.com/sapcc/argora/internal/credentials"
	"github.com/sapcc/argora/internal/status"
)

var _ = Describe("Ironcore Controller", func() {
	const (
		resourceName      = "test-resource"
		resourceNamespace = "default"
		bmcName1          = "device-name1"
		bmcName2          = "device-name2"
		clusterType1      = "type1"
		clusterType2      = "type2"
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

		typeNamespacedClusterImportName := types.NamespacedName{
			Name:      resourceName,
			Namespace: resourceNamespace,
		}
		typeNamespacedBMCName1 := types.NamespacedName{
			Name:      bmcName1,
			Namespace: resourceNamespace,
		}
		typeNamespacedBMCName2 := types.NamespacedName{
			Name:      bmcName2,
			Namespace: resourceNamespace,
		}

		clusterImport := &argorav1alpha1.ClusterImport{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: resourceNamespace,
			},
		}

		expectStatus := func(state argorav1alpha1.State, description string) {
			err := k8sClient.Get(ctx, typeNamespacedClusterImportName, clusterImport)
			Expect(err).ToNot(HaveOccurred())
			Expect(clusterImport.Status.State).To(Equal(state))
			Expect(clusterImport.Status.Description).To(Equal(description))
			Expect(clusterImport.Status.Conditions).ToNot(BeNil())
			Expect((*clusterImport.Status.Conditions)).To(HaveLen(1))
			if state == argorav1alpha1.Ready {
				Expect((*clusterImport.Status.Conditions)[0].Type).To(Equal(string(argorav1alpha1.ConditionTypeReady)))
				Expect((*clusterImport.Status.Conditions)[0].Status).To(Equal(metav1.ConditionTrue))
				Expect((*clusterImport.Status.Conditions)[0].Reason).To(Equal(string(argorav1alpha1.ConditionReasonClusterImportSucceeded)))
				Expect((*clusterImport.Status.Conditions)[0].Message).To(Equal(argorav1alpha1.ConditionReasonClusterImportSucceededMessage))
			} else {
				Expect((*clusterImport.Status.Conditions)[0].Type).To(Equal(string(argorav1alpha1.ConditionTypeReady)))
				Expect((*clusterImport.Status.Conditions)[0].Status).To(Equal(metav1.ConditionFalse))
				Expect((*clusterImport.Status.Conditions)[0].Reason).To(Equal(string(argorav1alpha1.ConditionReasonClusterImportFailed)))
				Expect((*clusterImport.Status.Conditions)[0].Message).To(Equal(argorav1alpha1.ConditionReasonClusterImportFailedMessage))
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

			netBoxMock.VirtualizationMock.(*mock.VirtualizationMock).GetClustersByNameRegionTypeFunc = func(name, region, clusterType string) ([]models.Cluster, error) {
				Expect(name).To(Equal("name1"))
				Expect(region).To(Equal("region1"))
				Expect(clusterType).To(Equal("type1"))

				return []models.Cluster{
					{
						ID:   1,
						Name: "cluster1",
						Type: models.NestedClusterType{
							Slug: clusterType1,
						},
					},
				}, nil
			}
			netBoxMock.DCIMMock.(*mock.DCIMMock).GetDevicesByClusterIDFunc = func(clusterID int) ([]models.Device, error) {
				Expect(clusterID).To(BeElementOf(1, 2))
				return []models.Device{
					{
						ID:     clusterID,
						Name:   fmt.Sprintf("device-name%d", clusterID),
						Status: models.DeviceStatus{Value: "active"},
						Platform: models.NestedPlatform{
							ID:   1,
							Slug: "platform1",
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
				Expect(device.Name).To(BeElementOf("device-name1", "device-name2"))
				return "region1", nil
			}

			return netBoxMock
		}

		expectLabels := func(labels map[string]string, bmcName string, clusterName string) {
			bb, _ := strings.CutPrefix(bmcName, "device-")
			Expect(labels).To(SatisfyAll(
				HaveKeyWithValue("topology.kubernetes.io/region", "region1"),
				HaveKeyWithValue("topology.kubernetes.io/zone", "site1"),
				HaveKeyWithValue("kubernetes.metal.cloud.sap/cluster", clusterName),
				HaveKeyWithValue("kubernetes.metal.cloud.sap/cluster-type", clusterType1),
				HaveKeyWithValue("kubernetes.metal.cloud.sap/name", bmcName),
				HaveKeyWithValue("kubernetes.metal.cloud.sap/bb", bb),
				HaveKeyWithValue("kubernetes.metal.cloud.sap/type", "type1"),
				HaveKeyWithValue("kubernetes.metal.cloud.sap/role", "role1"),
				HaveKeyWithValue("kubernetes.metal.cloud.sap/platform", "platform1"),
			))
		}

		expectBMCSecret := func(bmcSecret *metalv1alpha1.BMCSecret, bmcName string, clusterName string) {
			Expect(bmcSecret.Name).To(Equal(bmcName))
			expectLabels(bmcSecret.Labels, bmcName, clusterName)

			Expect(bmcSecret.ObjectMeta.OwnerReferences).To(HaveLen(1))
			Expect(bmcSecret.ObjectMeta.OwnerReferences[0].APIVersion).To(Equal("metal.ironcore.dev/v1alpha1"))
			Expect(bmcSecret.ObjectMeta.OwnerReferences[0].Kind).To(Equal("BMC"))
			Expect(bmcSecret.ObjectMeta.OwnerReferences[0].Name).To(Equal(bmcName))
			Expect(*bmcSecret.ObjectMeta.OwnerReferences[0].Controller).To(BeTrue())
			Expect(*bmcSecret.ObjectMeta.OwnerReferences[0].BlockOwnerDeletion).To(BeTrue())

			Expect(bmcSecret.Data).To(Equal(map[string][]byte{
				metalv1alpha1.BMCSecretUsernameKeyName: []byte("user"),
				metalv1alpha1.BMCSecretPasswordKeyName: []byte("password"),
			}))
		}

		expectBMC := func(bmc *metalv1alpha1.BMC, bmcName string, clusterName string) {
			Expect(bmc.Name).To(Equal(bmcName))
			expectLabels(bmc.Labels, bmcName, clusterName)
			Expect(bmc.ObjectMeta.OwnerReferences).To(BeEmpty())

			Expect(bmc.Spec.Endpoint.IP.String()).To(Equal("192.168.1.1"))
			Expect(bmc.Spec.Protocol.Name).To(Equal(metalv1alpha1.ProtocolNameRedfish))
			Expect(bmc.Spec.Protocol.Port).To(Equal(int32(443)))
			Expect(bmc.Spec.BMCSecretRef.Name).To(Equal(bmcName))
		}

		expectBMCResources := func(clusterNames []string) {
			bmcSecretList := &metalv1alpha1.BMCSecretList{}
			err := k8sClient.List(ctx, bmcSecretList)
			Expect(err).ToNot(HaveOccurred())
			Expect(bmcSecretList.Items).To(HaveLen(len(clusterNames)))

			bmcSecret := &metalv1alpha1.BMCSecret{}
			err = k8sClient.Get(ctx, typeNamespacedBMCName1, bmcSecret)
			Expect(err).ToNot(HaveOccurred())
			expectBMCSecret(bmcSecret, bmcName1, clusterNames[0])

			if len(clusterNames) == 2 {
				bmcSecret2 := &metalv1alpha1.BMCSecret{}
				err = k8sClient.Get(ctx, typeNamespacedBMCName2, bmcSecret2)
				Expect(err).ToNot(HaveOccurred())
				expectBMCSecret(bmcSecret2, bmcName2, clusterNames[1])
			}

			bmcList := &metalv1alpha1.BMCList{}
			err = k8sClient.List(ctx, bmcList)
			Expect(err).ToNot(HaveOccurred())
			Expect(bmcList.Items).To(HaveLen(len(clusterNames)))

			bmc := &metalv1alpha1.BMC{}
			err = k8sClient.Get(ctx, typeNamespacedBMCName1, bmc)
			Expect(err).ToNot(HaveOccurred())
			expectBMC(bmc, bmcName1, clusterNames[0])

			if len(clusterNames) == 2 {
				bmc2 := &metalv1alpha1.BMC{}
				err = k8sClient.Get(ctx, typeNamespacedBMCName2, bmc2)
				Expect(err).ToNot(HaveOccurred())
				expectBMC(bmc2, bmcName2, clusterNames[1])
			}
		}

		Context("Envtest", func() {
			BeforeEach(func() {
				By("create ClusterImport CR")
				err := k8sClient.Get(ctx, typeNamespacedClusterImportName, clusterImport)
				if err != nil && apierrors.IsNotFound(err) {
					resource := &argorav1alpha1.ClusterImport{
						ObjectMeta: metav1.ObjectMeta{
							Name:      resourceName,
							Namespace: resourceNamespace,
						},
						Spec: argorav1alpha1.ClusterImportSpec{
							Clusters: []*argorav1alpha1.ClusterSelector{
								{
									Name:   "name1",
									Region: "region1",
									Type:   "type1",
								},
							},
						},
					}
					Expect(k8sClient.Create(ctx, resource)).To(Succeed())
				}
			})

			AfterEach(func() {
				By("cleanup")
				bmcSecret := &metalv1alpha1.BMCSecret{}
				err := k8sClient.Get(ctx, typeNamespacedBMCName1, bmcSecret)
				if err == nil {
					By("delete BMC Secret 1")
					Expect(k8sClient.Delete(ctx, bmcSecret)).To(Succeed())
				}
				err = k8sClient.Get(ctx, typeNamespacedBMCName2, bmcSecret)
				if err == nil {
					By("delete BMC Secret 2")
					Expect(k8sClient.Delete(ctx, bmcSecret)).To(Succeed())
				}

				bmc := &metalv1alpha1.BMC{}
				err = k8sClient.Get(ctx, typeNamespacedBMCName1, bmc)
				if err == nil {
					By("delete BMC 1")
					Expect(k8sClient.Delete(ctx, bmc)).To(Succeed())
				}
				err = k8sClient.Get(ctx, typeNamespacedBMCName2, bmc)
				if err == nil {
					By("delete BMC 2")
					Expect(k8sClient.Delete(ctx, bmc)).To(Succeed())
				}

				err = k8sClient.Get(ctx, typeNamespacedClusterImportName, clusterImport)
				Expect(err).ToNot(HaveOccurred())

				By("delete Update CR")
				Expect(k8sClient.Delete(ctx, clusterImport)).To(Succeed())
			})

			It("should successfully reconcile", func() {
				// given
				netBoxMock := prepareNetboxMock()
				controllerReconciler := createIronCoreReconciler(k8sClient, netBoxMock, fileReaderMock)

				// when
				res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedClusterImportName})

				// then
				Expect(err).ToNot(HaveOccurred())
				Expect(res.RequeueAfter).To(Equal(reconcileInterval))

				expectBMCResources([]string{"cluster1"})

				Expect(netBoxMock.VirtualizationMock.(*mock.VirtualizationMock).GetClustersByNameRegionTypeCalls).To(Equal(1))
				Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).GetDevicesByClusterIDCalls).To(Equal(1))
				Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).GetRegionForDeviceCalls).To(Equal(1))

				expectStatus(argorav1alpha1.Ready, "")
			})

			It("should successfully reconcile if cluster selection is only by name", func() {
				// given
				netBoxMock := prepareNetboxMock()
				netBoxMock.VirtualizationMock.(*mock.VirtualizationMock).GetClustersByNameRegionTypeFunc = func(name, region, clusterType string) ([]models.Cluster, error) {
					Expect(name).To(Equal("name1"))
					Expect(region).To(BeEmpty())
					Expect(clusterType).To(BeEmpty())

					return []models.Cluster{
						{
							ID:   1,
							Name: "cluster1",
							Type: models.NestedClusterType{
								Slug: clusterType1,
							},
						},
					}, nil
				}
				fileReaderMockWithNameOnly := &mock.FileReaderMock{
					FileContent: make(map[string]string),
					ReturnError: false,
				}
				fileReaderMockWithNameOnly.FileContent["/etc/credentials/credentials.json"] = fileReaderMock.FileContent["/etc/credentials/credentials.json"]
				controllerReconciler := createIronCoreReconciler(k8sClient, netBoxMock, fileReaderMockWithNameOnly)

				err := k8sClient.Get(ctx, typeNamespacedClusterImportName, clusterImport)
				Expect(err).ToNot(HaveOccurred())

				clusterImport.Spec.Clusters = []*argorav1alpha1.ClusterSelector{
					{
						Name: "name1",
					},
				}
				Expect(k8sClient.Update(ctx, clusterImport)).To(Succeed())

				// when
				res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedClusterImportName})

				// then
				Expect(err).ToNot(HaveOccurred())
				Expect(res.RequeueAfter).To(Equal(reconcileInterval))

				expectBMCResources([]string{"cluster1"})

				Expect(netBoxMock.VirtualizationMock.(*mock.VirtualizationMock).GetClustersByNameRegionTypeCalls).To(Equal(1))
				Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).GetDevicesByClusterIDCalls).To(Equal(1))
				Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).GetRegionForDeviceCalls).To(Equal(1))

				expectStatus(argorav1alpha1.Ready, "")
			})

			It("should successfully reconcile if multiple clusters in the CR", func() {
				// given
				netBoxMock := prepareNetboxMock()
				netBoxMock.VirtualizationMock.(*mock.VirtualizationMock).GetClustersByNameRegionTypeFunc = func(name, region, clusterType string) ([]models.Cluster, error) {
					Expect(name).To(BeElementOf("name1", "name2"))
					Expect(region).To(Equal(""))
					Expect(clusterType).To(Equal(""))

					return []models.Cluster{
						{
							ID:   1,
							Name: "cluster1",
							Type: models.NestedClusterType{
								Slug: clusterType1,
							},
						},
					}, nil
				}

				fileReaderMockWithNameOnly := &mock.FileReaderMock{
					FileContent: make(map[string]string),
					ReturnError: false,
				}
				fileReaderMockWithNameOnly.FileContent["/etc/credentials/credentials.json"] = fileReaderMock.FileContent["/etc/credentials/credentials.json"]
				controllerReconciler := createIronCoreReconciler(k8sClient, netBoxMock, fileReaderMockWithNameOnly)

				err := k8sClient.Get(ctx, typeNamespacedClusterImportName, clusterImport)
				Expect(err).ToNot(HaveOccurred())

				clusterImport.Spec.Clusters = []*argorav1alpha1.ClusterSelector{
					{
						Name: "name1",
					},
					{
						Name: "name2",
					},
				}
				Expect(k8sClient.Update(ctx, clusterImport)).To(Succeed())

				// when
				res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedClusterImportName})

				// then
				Expect(err).ToNot(HaveOccurred())
				Expect(res.RequeueAfter).To(Equal(reconcileInterval))

				expectBMCResources([]string{"cluster1"})

				Expect(netBoxMock.VirtualizationMock.(*mock.VirtualizationMock).GetClustersByNameRegionTypeCalls).To(Equal(2))
				Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).GetDevicesByClusterIDCalls).To(Equal(2))
				Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).GetRegionForDeviceCalls).To(Equal(2))

				expectStatus(argorav1alpha1.Ready, "")
			})

			It("should succeed if multiple clusters fetched from netbox", func() {
				// given
				netBoxMock := prepareNetboxMock()
				netBoxMock.VirtualizationMock.(*mock.VirtualizationMock).GetClustersByNameRegionTypeFunc = func(name, region, clusterType string) ([]models.Cluster, error) {
					Expect(name).To(Equal(""))
					Expect(region).To(Equal(""))
					Expect(clusterType).To(Equal("type1"))

					return []models.Cluster{
						{
							ID:   1,
							Name: "cluster1",
							Type: models.NestedClusterType{
								Slug: clusterType1,
							},
						},
						{
							ID:   2,
							Name: "cluster2",
							Type: models.NestedClusterType{
								Slug: clusterType1,
							},
						},
					}, nil
				}

				fileReaderMockWithNameOnly := &mock.FileReaderMock{
					FileContent: make(map[string]string),
					ReturnError: false,
				}
				fileReaderMockWithNameOnly.FileContent["/etc/credentials/credentials.json"] = fileReaderMock.FileContent["/etc/credentials/credentials.json"]
				controllerReconciler := createIronCoreReconciler(k8sClient, netBoxMock, fileReaderMockWithNameOnly)

				err := k8sClient.Get(ctx, typeNamespacedClusterImportName, clusterImport)
				Expect(err).ToNot(HaveOccurred())

				clusterImport.Spec.Clusters = []*argorav1alpha1.ClusterSelector{
					{
						Type: "type1",
					},
				}
				Expect(k8sClient.Update(ctx, clusterImport)).To(Succeed())

				// when
				res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedClusterImportName})

				// then
				Expect(err).ToNot(HaveOccurred())
				Expect(res.RequeueAfter).To(Equal(reconcileInterval))

				expectBMCResources([]string{"cluster1", "cluster2"})

				Expect(netBoxMock.VirtualizationMock.(*mock.VirtualizationMock).GetClustersByNameRegionTypeCalls).To(Equal(1))
				Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).GetDevicesByClusterIDCalls).To(Equal(2))
				Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).GetRegionForDeviceCalls).To(Equal(2))

				expectStatus(argorav1alpha1.Ready, "")
			})

			It("should update labels if device change cluster", func() {
				// given
				netBoxMock := prepareNetboxMock()
				netBoxMock.VirtualizationMock.(*mock.VirtualizationMock).GetClustersByNameRegionTypeFunc = func(name, region, clusterType string) ([]models.Cluster, error) {
					Expect(name).To(Equal(""))
					Expect(region).To(Equal(""))
					Expect(clusterType).To(Equal("type1"))

					return []models.Cluster{
						{
							ID:   1,
							Name: "cluster1",
							Type: models.NestedClusterType{
								Slug: clusterType1,
							},
						},
						{
							ID:   2,
							Name: "cluster2",
							Type: models.NestedClusterType{
								Slug: clusterType1,
							},
						},
					}, nil
				}
				netBoxMock.DCIMMock.(*mock.DCIMMock).GetDevicesByClusterIDFunc = func(clusterID int) ([]models.Device, error) {
					Expect(clusterID).To(BeElementOf(1, 2))
					if clusterID == 1 {
						return []models.Device{
							{
								ID:     clusterID,
								Name:   "device-name1",
								Status: models.DeviceStatus{Value: "active"},
								Platform: models.NestedPlatform{
									ID:   1,
									Slug: "platform1",
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
					} else {
						return []models.Device{}, nil
					}
				}
				fileReaderMockWithNameOnly := &mock.FileReaderMock{
					FileContent: make(map[string]string),
					ReturnError: false,
				}
				fileReaderMockWithNameOnly.FileContent["/etc/credentials/credentials.json"] = fileReaderMock.FileContent["/etc/credentials/credentials.json"]
				controllerReconciler := createIronCoreReconciler(k8sClient, netBoxMock, fileReaderMockWithNameOnly)

				err := k8sClient.Get(ctx, typeNamespacedClusterImportName, clusterImport)
				Expect(err).ToNot(HaveOccurred())

				clusterImport.Spec.Clusters = []*argorav1alpha1.ClusterSelector{
					{
						Type: "type1",
					},
				}
				Expect(k8sClient.Update(ctx, clusterImport)).To(Succeed())

				// when
				res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedClusterImportName})

				// then
				Expect(err).ToNot(HaveOccurred())
				Expect(res.RequeueAfter).To(Equal(reconcileInterval))

				expectBMCResources([]string{"cluster1"})

				Expect(netBoxMock.VirtualizationMock.(*mock.VirtualizationMock).GetClustersByNameRegionTypeCalls).To(Equal(1))
				Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).GetDevicesByClusterIDCalls).To(Equal(2))
				Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).GetRegionForDeviceCalls).To(Equal(1))

				// and when device moved to cluster2

				netBoxMock.DCIMMock.(*mock.DCIMMock).GetDevicesByClusterIDFunc = func(clusterID int) ([]models.Device, error) {
					Expect(clusterID).To(BeElementOf(1, 2))
					if clusterID == 2 {
						return []models.Device{
							{
								ID:     clusterID,
								Name:   "device-name1",
								Status: models.DeviceStatus{Value: "active"},
								Platform: models.NestedPlatform{
									ID:   1,
									Slug: "platform1",
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
					} else {
						return []models.Device{}, nil
					}
				}

				res, err = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedClusterImportName})

				// then
				Expect(err).ToNot(HaveOccurred())
				Expect(res.RequeueAfter).To(Equal(reconcileInterval))

				expectBMCResources([]string{"cluster2"})

				Expect(netBoxMock.VirtualizationMock.(*mock.VirtualizationMock).GetClustersByNameRegionTypeCalls).To(Equal(2))
				Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).GetDevicesByClusterIDCalls).To(Equal(4))
				Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).GetRegionForDeviceCalls).To(Equal(2))

				expectStatus(argorav1alpha1.Ready, "")
			})

			It("should succeed if multiple devices fetched from netbox", func() {
				// given
				netBoxMock := prepareNetboxMock()
				netBoxMock.VirtualizationMock.(*mock.VirtualizationMock).GetClustersByNameRegionTypeFunc = func(name, region, clusterType string) ([]models.Cluster, error) {
					Expect(name).To(Equal("cluster1"))
					Expect(region).To(Equal(""))
					Expect(clusterType).To(Equal(""))

					return []models.Cluster{
						{
							ID:   1,
							Name: "cluster1",
							Type: models.NestedClusterType{
								Slug: clusterType1,
							},
						},
					}, nil
				}
				netBoxMock.DCIMMock.(*mock.DCIMMock).GetDevicesByClusterIDFunc = func(clusterID int) ([]models.Device, error) {
					Expect(clusterID).To(BeElementOf(1))
					devices := make([]models.Device, 0, 2)
					for _, deviceName := range []string{"device-name1", "device-name2"} {
						devices = append(devices, models.Device{
							ID:     1,
							Name:   deviceName,
							Status: models.DeviceStatus{Value: "active"},
							Platform: models.NestedPlatform{
								ID:   1,
								Slug: "platform1",
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
						})
					}
					return devices, nil
				}

				netBoxMock.DCIMMock.(*mock.DCIMMock).GetRegionForDeviceFunc = func(device *models.Device) (string, error) {
					Expect(device.Name).To(BeElementOf("device-name1", "device-name2"))
					return "region1", nil
				}

				fileReaderMockWithNameOnly := &mock.FileReaderMock{
					FileContent: make(map[string]string),
					ReturnError: false,
				}
				fileReaderMockWithNameOnly.FileContent["/etc/credentials/credentials.json"] = fileReaderMock.FileContent["/etc/credentials/credentials.json"]
				controllerReconciler := createIronCoreReconciler(k8sClient, netBoxMock, fileReaderMockWithNameOnly)

				err := k8sClient.Get(ctx, typeNamespacedClusterImportName, clusterImport)
				Expect(err).ToNot(HaveOccurred())

				clusterImport.Spec.Clusters = []*argorav1alpha1.ClusterSelector{
					{
						Name: "cluster1",
					},
				}
				Expect(k8sClient.Update(ctx, clusterImport)).To(Succeed())

				// when
				res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedClusterImportName})

				// then
				Expect(err).ToNot(HaveOccurred())
				Expect(res.RequeueAfter).To(Equal(reconcileInterval))

				expectBMCResources([]string{"cluster1", "cluster1"})

				Expect(netBoxMock.VirtualizationMock.(*mock.VirtualizationMock).GetClustersByNameRegionTypeCalls).To(Equal(1))
				Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).GetDevicesByClusterIDCalls).To(Equal(1))
				Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).GetRegionForDeviceCalls).To(Equal(2))

				expectStatus(argorav1alpha1.Ready, "")
			})

			It("should return an error if credentials reload fails", func() {
				// given
				netBoxMock := prepareNetboxMock()
				fileReaderMockToError := &mock.FileReaderMock{
					FileContent: make(map[string]string),
					ReturnError: true,
				}
				controllerReconciler := createIronCoreReconciler(k8sClient, netBoxMock, fileReaderMockToError)

				// when
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedClusterImportName})

				// then
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("unable to read credentials.json: error"))

				expectStatus(argorav1alpha1.Error, "unable to read credentials.json: error")
			})

			It("should return an error if netbox reload fails", func() {
				// given
				controllerReconciler := createIronCoreReconciler(k8sClient, &mock.NetBoxMock{ReturnError: true}, fileReaderMock)

				// when
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedClusterImportName})

				// then
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("unable to reload netbox"))

				expectStatus(argorav1alpha1.Error, "unable to reload netbox")
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
				res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedClusterImportName})

				// then
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("unable to find clusters"))
				Expect(res.RequeueAfter).To(Equal(0 * time.Second))

				expectStatus(argorav1alpha1.Error, "unable to reconcile cluster: unable to find clusters")
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
							Type: models.NestedClusterType{
								Slug: clusterType1,
							},
						},
					}, nil
				}
				netBoxMock.DCIMMock.(*mock.DCIMMock).GetDevicesByClusterIDFunc = func(clusterID int) ([]models.Device, error) {
					Expect(clusterID).To(Equal(1))
					return nil, errors.New("unable to find devices")
				}

				controllerReconciler := createIronCoreReconciler(k8sClient, netBoxMock, fileReaderMock)

				// when
				res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedClusterImportName})

				// then
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("unable to find devices"))
				Expect(res.RequeueAfter).To(Equal(0 * time.Second))

				expectStatus(argorav1alpha1.Error, "unable to reconcile devices on cluster cluster1 (1): unable to find devices")
			})

			It("should return an error if GetRegionForDevice fails", func() {
				// given
				netBoxMock := prepareNetboxMock()
				netBoxMock.DCIMMock.(*mock.DCIMMock).GetRegionForDeviceFunc = func(device *models.Device) (string, error) {
					return "", errors.New("unable to get region for device")
				}

				controllerReconciler := createIronCoreReconciler(k8sClient, netBoxMock, fileReaderMock)

				// when
				res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedClusterImportName})

				// then
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("unable to get region for device: unable to get region for device"))
				Expect(res.RequeueAfter).To(Equal(0 * time.Second))

				expectStatus(argorav1alpha1.Error, "unable to reconcile device device-name1 (1) on cluster cluster1 (1): unable to get region for device: unable to get region for device")
			})

			It("should skip the device when the device is not active", func() {
				// given
				netBoxMock := prepareNetboxMock()
				netBoxMock.VirtualizationMock.(*mock.VirtualizationMock).GetClustersByNameRegionTypeFunc = func(name, region, clusterType string) ([]models.Cluster, error) {
					return []models.Cluster{
						{
							ID:   1,
							Name: "cluster1",
							Type: models.NestedClusterType{
								Slug: clusterType1,
							},
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
				res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedClusterImportName})

				// then
				Expect(err).ToNot(HaveOccurred())
				Expect(res.RequeueAfter).To(Equal(reconcileInterval))

				expectStatus(argorav1alpha1.Ready, "")
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
				Expect(err).ToNot(HaveOccurred())

				// when
				res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedClusterImportName})

				// then
				Expect(err).ToNot(HaveOccurred())
				Expect(res.RequeueAfter).To(Equal(reconcileInterval))

				expectStatus(argorav1alpha1.Ready, "")
			})
		})

		Context("Fake Client", func() {
			clusterImportCR := &argorav1alpha1.ClusterImport{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: resourceNamespace,
				},
				Spec: argorav1alpha1.ClusterImportSpec{
					Clusters: []*argorav1alpha1.ClusterSelector{
						{
							Name:   "name1",
							Region: "region1",
							Type:   "type1",
						},
					},
				},
			}

			It("should return an error if createBmcSecret fails", func() {
				// given
				netBoxMock := prepareNetboxMock()

				fakeClient := createFakeClient(clusterImportCR)
				failClient := &shouldFailClient{fakeClient, "BMCSecret"}

				controllerReconciler := createIronCoreReconciler(failClient, netBoxMock, fileReaderMock)

				// when
				res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedClusterImportName})

				// then
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("unable to create bmc secret: intentionally failing client on client.Create for BMCSecret"))
				Expect(res.RequeueAfter).To(Equal(0 * time.Second))
			})

			It("should return an error if createBmc fails", func() {
				// given
				netBoxMock := prepareNetboxMock()

				fakeClient := createFakeClient(clusterImportCR)
				failClient := &shouldFailClient{fakeClient, "BMC"}

				controllerReconciler := createIronCoreReconciler(failClient, netBoxMock, fileReaderMock)

				// when
				res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedClusterImportName})

				// then
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("unable to create bmc: unable to create BMC: intentionally failing client on client.Create for BMC"))
				Expect(res.RequeueAfter).To(Equal(0 * time.Second))
			})
		})
	})
})

func createIronCoreReconciler(k8sClient client.Client, netBoxMock *mock.NetBoxMock, fileReaderMock credentials.FileReader) *IronCoreReconciler {
	return &IronCoreReconciler{
		k8sClient:         k8sClient,
		scheme:            k8sClient.Scheme(),
		credentials:       credentials.NewDefaultCredentials(fileReaderMock),
		statusHandler:     status.NewClusterImportStatusHandler(k8sClient),
		netBox:            netBoxMock,
		reconcileInterval: reconcileInterval,
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
