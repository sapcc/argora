// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"time"

	"github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sapcc/go-netbox-go/models"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/sapcc/argora/internal/controller/mock"
	"github.com/sapcc/argora/internal/credentials"
	"github.com/sapcc/argora/internal/networkdata"
)

var _ = Describe("Metal3 Controller", func() {
	const (
		clusterName      = "cluster1"
		clusterType      = "type1"
		clusterNamespace = "default"
		deviceName       = "device-name1"
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

	ctx := context.Background()

	typeNamespacedClusterName := types.NamespacedName{
		Name:      clusterName,
		Namespace: clusterNamespace,
	}

	typeNamespacedSecretName := types.NamespacedName{
		Name:      "bmc-secret-" + deviceName,
		Namespace: clusterNamespace,
	}

	typeNamespacedBareMetalHostName := types.NamespacedName{
		Name:      deviceName,
		Namespace: clusterNamespace,
	}

	typeNamespacedNDSecretName := types.NamespacedName{
		Name:      "networkdata-" + deviceName,
		Namespace: clusterNamespace,
	}

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: clusterNamespace,
		},
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
			Expect(name).To(Equal(clusterName))
			Expect(region).To(BeEmpty())
			Expect(clusterType).To(Equal(clusterType))

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
					Name:   deviceName,
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
						Model: "PowerEdge R640",
						Slug:  "poweredge-r640",
					},
					DeviceRole: models.NestedDeviceRole{
						Slug: "KVM",
					},
					PrimaryIP4: models.NestedIPAddress{
						Address: "192.168.1.1",
					},
					Tags: []models.NestedTag{
						{
							Name: "KVM",
						},
					},
				},
			}, nil
		}
		netBoxMock.DCIMMock.(*mock.DCIMMock).GetRegionForDeviceFunc = func(device *models.Device) (string, error) {
			Expect(device.Name).To(Equal(deviceName))
			return "region1", nil
		}
		netBoxMock.IPAMMock.(*mock.IPAMMock).GetIPAddressByAddressFunc = func(ipAddress string) (*models.IPAddress, error) {
			Expect(ipAddress).To(Equal("192.168.1.1"))
			return &models.IPAddress{
				AssignedInterface: models.NestedInterface{
					ID: 1,
				},
			}, nil
		}
		netBoxMock.DCIMMock.(*mock.DCIMMock).GetInterfaceByIDFunc = func(id int) (*models.Interface, error) {
			Expect(id).To(Equal(1))
			return &models.Interface{
				NestedInterface: models.NestedInterface{
					ID: 1,
				},
			}, nil
		}
		netBoxMock.DCIMMock.(*mock.DCIMMock).GetInterfacesByLagIDFunc = func(lagID int) ([]models.Interface, error) {
			Expect(lagID).To(Equal(1))
			return []models.Interface{
				{
					Name:       "interface1",
					MacAddress: "a1:b2:c3:d4:e5:f6",
				},
			}, nil
		}
		netBoxMock.DCIMMock.(*mock.DCIMMock).GetInterfaceForDeviceFunc = func(device *models.Device, name string) (*models.Interface, error) {
			Expect(device.Name).To(Equal(deviceName))
			Expect(name).To(Equal("LAG1"))
			return &models.Interface{
				NestedInterface: models.NestedInterface{
					ID: 1,
				},
			}, nil
		}
		netBoxMock.IPAMMock.(*mock.IPAMMock).GetIPAddressForInterfaceFunc = func(interfaceID int) (*models.IPAddress, error) {
			Expect(interfaceID).To(Equal(1))
			return &models.IPAddress{
				NestedIPAddress: models.NestedIPAddress{
					Address: "192.168.1.2/24",
				},
			}, nil
		}
		netBoxMock.IPAMMock.(*mock.IPAMMock).GetPrefixesContainingFunc = func(ipAddress string) ([]models.Prefix, error) {
			Expect(ipAddress).To(Equal("192.168.1.2/24"))
			return []models.Prefix{
				{
					ID: 1,
					Vlan: models.NestedVLAN{
						ID: 1,
					},
				},
			}, nil
		}

		return netBoxMock
	}

	expectBMCSecret := func(bmcSecret *corev1.Secret) {
		Expect(bmcSecret.Name).To(Equal("bmc-secret-" + deviceName))
		Expect(bmcSecret.Namespace).To(Equal(clusterNamespace))
		Expect(bmcSecret.Data).To(Equal(map[string][]byte{
			"username": []byte("user"),
			"password": []byte("password"),
		}))
	}

	expectBareMetalHost := func(bmh *v1alpha1.BareMetalHost) {
		Expect(bmh.Name).To(Equal(deviceName))
		Expect(bmh.Namespace).To(Equal(clusterNamespace))
		Expect(bmh.Labels).To(Equal(map[string]string{
			"topology.kubernetes.io/region":      "region1",
			"topology.kubernetes.io/zone":        "site1",
			"kubernetes.metal.cloud.sap/cluster": clusterName,
			"kubernetes.metal.cloud.sap/name":    deviceName,
			"kubernetes.metal.cloud.sap/bb":      "name1",
			"kubernetes.metal.cloud.sap/role":    "kvm",
		}))
		Expect(bmh.Spec.Architecture).To(Equal("x86_64"))
		Expect(bmh.Spec.AutomatedCleaningMode).To(Equal(v1alpha1.AutomatedCleaningMode("disabled")))
		Expect(bmh.Spec.Online).To(BeTrue())
		Expect(bmh.Spec.BMC.Address).To(Equal("redfish://192.168.1.1/redfish/v1/Systems/1"))
		Expect(bmh.Spec.BMC.CredentialsName).To(Equal("bmc-secret-" + deviceName))
		Expect(bmh.Spec.BMC.DisableCertificateVerification).To(BeTrue())
		Expect(bmh.Spec.BootMACAddress).To(Equal("a1:b2:c3:d4:e5:f6"))
		Expect(bmh.Spec.NetworkData.Name).To(Equal("networkdata-" + deviceName))
		Expect(bmh.Spec.NetworkData.Namespace).To(Equal(clusterNamespace))
		Expect(bmh.Spec.RootDeviceHints.Model).To(Equal("BOSS"))
	}

	expectNetworkDataSecret := func(ndSecret *corev1.Secret) {
		Expect(ndSecret.Name).To(Equal("networkdata-" + deviceName))
		Expect(ndSecret.Namespace).To(Equal(clusterNamespace))
		Expect(ndSecret.Data).To(HaveKey("networkData"))

		var networkData networkdata.NetworkData
		err := yaml.Unmarshal(ndSecret.Data["networkData"], &networkData)
		Expect(err).ToNot(HaveOccurred())

		Expect(networkData.Links).To(BeEmpty())
		Expect(networkData.Services).To(BeEmpty())
		Expect(networkData.Networks).To(HaveLen(1))
		Expect(networkData.Networks[0].ID).To(Equal(0))
		Expect(networkData.Networks[0].Type).To(Equal(networkdata.Ipv4))
		Expect(networkData.Networks[0].IPAddress).To(Equal(ptr.To("192.168.1.2/24")))
		Expect(networkData.Networks[0].Link).To(Equal("en*f0np*"))
		Expect(networkData.Networks[0].Netmask).To(Equal(ptr.To("255.255.255.0")))
		Expect(networkData.Networks[0].NetworkID).To(Equal(""))
		Expect(networkData.Networks[0].Routes).To(HaveLen(1))
		Expect(networkData.Networks[0].Routes[0].Gateway).To(Equal("192.168.1.1"))
		Expect(networkData.Networks[0].Routes[0].Netmask).To(Equal("0.0.0.0"))
		Expect(networkData.Networks[0].Routes[0].Network).To(Equal("0.0.0.0"))
		Expect(networkData.Networks[0].Routes[0].Services).To(BeEmpty())
	}

	Context("Reconcile", func() {
		BeforeEach(func() {
			By("create Cluster CR")
			err := k8sClient.Get(ctx, typeNamespacedClusterName, cluster)
			if err != nil && apierrors.IsNotFound(err) {
				resource := &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      clusterName,
						Namespace: clusterNamespace,
						Labels: map[string]string{
							"discovery.inf.sap.cloud/clusterRole": clusterType,
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			By("cleanup")
			bmcSecret := &corev1.Secret{}
			err := k8sClient.Get(ctx, typeNamespacedSecretName, bmcSecret)
			if err == nil {
				By("delete BMC Secret")
				Expect(k8sClient.Delete(ctx, bmcSecret)).To(Succeed())
			}

			bmh := &v1alpha1.BareMetalHost{}
			err = k8sClient.Get(ctx, typeNamespacedBareMetalHostName, bmh)
			if err == nil {
				By("delete BareMetalHost")
				Expect(k8sClient.Delete(ctx, bmh)).To(Succeed())
			}

			ndSecret := &corev1.Secret{}
			err = k8sClient.Get(ctx, typeNamespacedNDSecretName, ndSecret)
			if err == nil {
				By("delete Network Data Secret")
				Expect(k8sClient.Delete(ctx, ndSecret)).To(Succeed())
			}

			cluster := &clusterv1.Cluster{}
			err = k8sClient.Get(ctx, typeNamespacedClusterName, cluster)
			if err == nil {
				By("delete Cluster")
				Expect(k8sClient.Delete(ctx, cluster)).To(Succeed())
			}
		})

		It("should successfully reconcile", func() {
			// given
			netBoxMock := prepareNetboxMock()
			controllerReconciler := createMetal3Reconciler(k8sClient, netBoxMock, fileReaderMock)

			// when
			res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedClusterName,
			})

			// then
			Expect(err).ToNot(HaveOccurred())
			Expect(res.RequeueAfter).To(Equal(reconcileIntervalDefault))

			bmcSecret := &corev1.Secret{}
			err = k8sClient.Get(ctx, typeNamespacedSecretName, bmcSecret)
			Expect(err).ToNot(HaveOccurred())
			expectBMCSecret(bmcSecret)

			bmh := &v1alpha1.BareMetalHost{}
			err = k8sClient.Get(ctx, typeNamespacedBareMetalHostName, bmh)
			Expect(err).ToNot(HaveOccurred())
			expectBareMetalHost(bmh)

			ndSecret := &corev1.Secret{}
			err = k8sClient.Get(ctx, typeNamespacedNDSecretName, ndSecret)
			Expect(err).ToNot(HaveOccurred())
			expectNetworkDataSecret(ndSecret)

			Expect(netBoxMock.VirtualizationMock.(*mock.VirtualizationMock).GetClustersByNameRegionTypeCalls).To(Equal(1))
			Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).GetDevicesByClusterIDCalls).To(Equal(1))
			Expect(netBoxMock.DCIMMock.(*mock.DCIMMock).GetRegionForDeviceCalls).To(Equal(1))
		})

		It("should return an error if credentials reload fails", func() {
			// given
			netBoxMock := prepareNetboxMock()
			fileReaderMockToError := &mock.FileReaderMock{
				FileContent: make(map[string]string),
				ReturnError: true,
			}
			controllerReconciler := createMetal3Reconciler(k8sClient, netBoxMock, fileReaderMockToError)

			// when
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{})

			// then
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unable to read credentials.json: error"))
		})

		It("should return an error if netbox reload fails", func() {
			// given
			controllerReconciler := createMetal3Reconciler(k8sClient, &mock.NetBoxMock{ReturnError: true}, fileReaderMock)

			// when
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedClusterName,
			})

			// then
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unable to reload netbox"))
		})

		It("should return an error if GetClustersByNameRegionType fails", func() {
			// given
			netBoxMock := prepareNetboxMock()
			netBoxMock.VirtualizationMock.(*mock.VirtualizationMock).GetClustersByNameRegionTypeFunc = func(name, region, ctype string) ([]models.Cluster, error) {
				Expect(name).To(Equal(clusterName))
				Expect(region).To(BeEmpty())
				Expect(ctype).To(Equal(clusterType))

				return nil, errors.New("unable to find clusters")
			}

			controllerReconciler := createMetal3Reconciler(k8sClient, netBoxMock, fileReaderMock)

			// when
			res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedClusterName,
			})

			// then
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unable to find clusters"))
			Expect(res.RequeueAfter).To(Equal(0 * time.Second))
		})

		It("should return an error if GetClustersByNameRegionType returns multiple clusters", func() {
			// given
			netBoxMock := prepareNetboxMock()
			netBoxMock.VirtualizationMock.(*mock.VirtualizationMock).GetClustersByNameRegionTypeFunc = func(name, region, ctype string) ([]models.Cluster, error) {
				Expect(name).To(Equal(clusterName))
				Expect(region).To(BeEmpty())
				Expect(ctype).To(Equal(clusterType))

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

			controllerReconciler := createMetal3Reconciler(k8sClient, netBoxMock, fileReaderMock)

			// when
			res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedClusterName,
			})

			// then
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("multiple clusters found"))
			Expect(res.RequeueAfter).To(Equal(0 * time.Second))
		})

		It("should return an error if GetDevicesByClusterID fails", func() {
			// given
			netBoxMock := prepareNetboxMock()
			netBoxMock.VirtualizationMock.(*mock.VirtualizationMock).GetClustersByNameRegionTypeFunc = func(name, region, ctype string) ([]models.Cluster, error) {
				Expect(name).To(Equal(clusterName))
				Expect(region).To(BeEmpty())
				Expect(ctype).To(Equal(clusterType))

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

			controllerReconciler := createMetal3Reconciler(k8sClient, netBoxMock, fileReaderMock)

			// when
			res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedClusterName,
			})

			// then
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unable to find devices"))
			Expect(res.RequeueAfter).To(Equal(0 * time.Second))
		})

		It("should skip the device when the device is not active", func() {
			// given
			netBoxMock := prepareNetboxMock()
			netBoxMock.VirtualizationMock.(*mock.VirtualizationMock).GetClustersByNameRegionTypeFunc = func(name, region, ctype string) ([]models.Cluster, error) {
				Expect(name).To(Equal(clusterName))
				Expect(region).To(BeEmpty())
				Expect(ctype).To(Equal(clusterType))

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
						Name:   "device1",
						Status: models.DeviceStatus{Value: "inactive"},
					},
				}, nil
			}

			controllerReconciler := createMetal3Reconciler(k8sClient, netBoxMock, fileReaderMock)

			// when
			res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedClusterName,
			})

			// then
			Expect(err).ToNot(HaveOccurred())
			Expect(res.RequeueAfter).To(Equal(reconcileIntervalDefault))
		})

		It("should return an error if GetRegionForDevice fails", func() {
			// given
			netBoxMock := prepareNetboxMock()
			netBoxMock.DCIMMock.(*mock.DCIMMock).GetRegionForDeviceFunc = func(device *models.Device) (string, error) {
				return "", errors.New("unable to get region for device")
			}

			controllerReconciler := createMetal3Reconciler(k8sClient, netBoxMock, fileReaderMock)

			// when
			res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedClusterName,
			})

			// then
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unable to get region for device: unable to get region for device"))
			Expect(res.RequeueAfter).To(Equal(0 * time.Second))
		})

		It("should skip the device when BareMetalHost custom resource already exists", func() {
			// given
			netBoxMock := prepareNetboxMock()
			controllerReconciler := createMetal3Reconciler(k8sClient, netBoxMock, fileReaderMock)

			// create existing BareMetalHost
			bmh := &v1alpha1.BareMetalHost{
				ObjectMeta: ctrl.ObjectMeta{
					Name:      deviceName,
					Namespace: clusterNamespace,
				},
				Spec: v1alpha1.BareMetalHostSpec{
					BMC: v1alpha1.BMCDetails{
						Address: "redfish://192.168.1.1/redfish/v1/Systems/1",
					},
				},
			}
			err := k8sClient.Create(ctx, bmh)
			Expect(err).ToNot(HaveOccurred())

			// when
			res, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedClusterName,
			})

			// then
			Expect(err).ToNot(HaveOccurred())
			Expect(res.RequeueAfter).To(Equal(reconcileIntervalDefault))
		})
	})
})

func createMetal3Reconciler(k8sClient client.Client, netBoxMock *mock.NetBoxMock, fileReaderMock credentials.FileReader) *Metal3Reconciler {
	return &Metal3Reconciler{
		k8sClient:         k8sClient,
		scheme:            k8sClient.Scheme(),
		credentials:       credentials.NewDefaultCredentials(fileReaderMock),
		netBox:            netBoxMock,
		reconcileInterval: time.Minute,
	}
}
