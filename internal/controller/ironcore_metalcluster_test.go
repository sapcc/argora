// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"

	moov1alpha1 "github.com/SAP-cloud-infrastructure/metal-onboarding-operator/api/v1alpha1"
	metaldhcpv1alpha1 "github.com/SAP-cloud-infrastructure/metaldhcp/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sapcc/go-netbox-go/models"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/sapcc/argora/internal/controller/mock"
)

var _ = Describe("MetalCluster helpers", func() {
	ctx := context.Background()

	Describe("buildInterfaceEntries", func() {
		It("includes a standard NIC interface as type nic", func() {
			ifaces := []models.Interface{
				{
					Name:       "eth0",
					MacAddress: "AA:BB:CC:DD:EE:01",
					Type:       models.InterfaceType{Value: "1000base-t"},
					MgmtOnly:   false,
				},
			}
			entries := buildInterfaceEntries(ifaces)
			Expect(entries).To(HaveLen(1))
			Expect(entries[0].Type).To(Equal(moov1alpha1.InterfaceTypeNIC))
			Expect(entries[0].MAC).To(Equal("aa:bb:cc:dd:ee:01"))
			Expect(entries[0].Name).To(Equal("eth0"))
		})

		It("includes a SmartNIC-mgmt tagged interface as type dpu", func() {
			ifaces := []models.Interface{
				{
					Name:       "smartnic0",
					MacAddress: "AA:BB:CC:DD:EE:02",
					Type:       models.InterfaceType{Value: "1000base-t"},
					Tags:       []models.NestedTag{{Name: dpuTagSmartNICMgmt}},
				},
			}
			entries := buildInterfaceEntries(ifaces)
			Expect(entries).To(HaveLen(1))
			Expect(entries[0].Type).To(Equal(moov1alpha1.InterfaceTypeDPU))
		})

		It("includes a SmartNIC-NIC1 tagged interface as type dpu", func() {
			ifaces := []models.Interface{
				{
					Name:       "smartnic1",
					MacAddress: "AA:BB:CC:DD:EE:03",
					Type:       models.InterfaceType{Value: "1000base-t"},
					Tags:       []models.NestedTag{{Name: dpuTagSmartNICNIC1}},
				},
			}
			entries := buildInterfaceEntries(ifaces)
			Expect(entries).To(HaveLen(1))
			Expect(entries[0].Type).To(Equal(moov1alpha1.InterfaceTypeDPU))
		})

		It("skips MgmtOnly interfaces", func() {
			ifaces := []models.Interface{
				{
					Name:       "mgmt0",
					MacAddress: "AA:BB:CC:DD:EE:04",
					MgmtOnly:   true,
				},
			}
			entries := buildInterfaceEntries(ifaces)
			Expect(entries).To(BeEmpty())
		})

		It("skips LAG interfaces", func() {
			ifaces := []models.Interface{
				{
					Name:       "bond0",
					MacAddress: "AA:BB:CC:DD:EE:05",
					Type:       models.InterfaceType{Value: interfaceTypeLag},
				},
			}
			entries := buildInterfaceEntries(ifaces)
			Expect(entries).To(BeEmpty())
		})

		It("skips interfaces with empty MAC", func() {
			ifaces := []models.Interface{
				{
					Name:       "eth1",
					MacAddress: "",
					Type:       models.InterfaceType{Value: "1000base-t"},
				},
			}
			entries := buildInterfaceEntries(ifaces)
			Expect(entries).To(BeEmpty())
		})

		It("skips the remoteboard interface", func() {
			ifaces := []models.Interface{
				{
					Name:       remoteboardInterfaceName,
					MacAddress: "AA:BB:CC:DD:EE:06",
					Type:       models.InterfaceType{Value: "1000base-t"},
				},
			}
			entries := buildInterfaceEntries(ifaces)
			Expect(entries).To(BeEmpty())
		})
	})

	Describe("getRemoteboardEntry", func() {
		It("returns MAC and oob entry when remoteboard interface has a MAC", func() {
			dcimMock := &mock.DCIMMock{
				GetInterfaceForDeviceFunc: func(device *models.Device, ifaceName string) (*models.Interface, error) {
					Expect(ifaceName).To(Equal(remoteboardInterfaceName))
					return &models.Interface{
						Name:       remoteboardInterfaceName,
						MacAddress: "AA:BB:CC:DD:EE:FF",
					}, nil
				},
			}
			nb := &mock.NetBoxMock{DCIMMock: dcimMock}
			device := &models.Device{Name: "node001-bb001"}

			mac, entry, err := getRemoteboardEntry(nb, device)
			Expect(err).ToNot(HaveOccurred())
			Expect(mac).To(Equal("aa:bb:cc:dd:ee:ff"))
			Expect(entry).ToNot(BeNil())
			Expect(entry.Type).To(Equal(moov1alpha1.InterfaceTypeOOB))
			Expect(entry.MAC).To(Equal("aa:bb:cc:dd:ee:ff"))
			Expect(entry.Name).To(Equal(remoteboardInterfaceName))
		})

		It("returns empty string and nil entry when MAC is empty", func() {
			dcimMock := &mock.DCIMMock{
				GetInterfaceForDeviceFunc: func(device *models.Device, ifaceName string) (*models.Interface, error) {
					return &models.Interface{
						Name:       remoteboardInterfaceName,
						MacAddress: "",
					}, nil
				},
			}
			nb := &mock.NetBoxMock{DCIMMock: dcimMock}
			device := &models.Device{Name: "node001-bb001"}

			mac, entry, err := getRemoteboardEntry(nb, device)
			Expect(err).ToNot(HaveOccurred())
			Expect(mac).To(Equal(""))
			Expect(entry).To(BeNil())
		})

		It("returns error when GetInterfaceForDevice fails", func() {
			dcimMock := &mock.DCIMMock{
				GetInterfaceForDeviceFunc: func(device *models.Device, ifaceName string) (*models.Interface, error) {
					return nil, errors.New("remoteboard not found")
				},
			}
			nb := &mock.NetBoxMock{DCIMMock: dcimMock}
			device := &models.Device{Name: "node001-bb001"}

			mac, entry, err := getRemoteboardEntry(nb, device)
			Expect(err).To(HaveOccurred())
			Expect(mac).To(Equal(""))
			Expect(entry).To(BeNil())
		})
	})

	Describe("buildServerEntries", func() {
		cluster := &models.Cluster{
			ID:   1,
			Name: "cluster-test",
			Type: models.NestedClusterType{Slug: "test-type"},
		}

		makeNetboxMock := func(ifaces []models.Interface, remoteMAC string) *mock.NetBoxMock {
			dcimMock := &mock.DCIMMock{
				GetRegionForDeviceFunc: func(device *models.Device) (string, error) {
					return "eu-de-1", nil
				},
				GetInterfaceForDeviceFunc: func(device *models.Device, ifaceName string) (*models.Interface, error) {
					return &models.Interface{
						Name:       remoteboardInterfaceName,
						MacAddress: remoteMAC,
					}, nil
				},
				GetInterfacesForDeviceFunc: func(device *models.Device) ([]models.Interface, error) {
					return ifaces, nil
				},
			}
			return &mock.NetBoxMock{DCIMMock: dcimMock}
		}

		It("skips inactive devices", func() {
			devices := []models.Device{
				{
					Name:   "node001-bb001",
					Status: models.DeviceStatus{Value: "offline"},
					OOBIp:  models.NestedIPAddress{Address: "10.0.0.1/24"},
				},
			}
			nb := makeNetboxMock(nil, "")
			entries, err := buildServerEntries(ctx, devices, cluster, nb)
			Expect(err).ToNot(HaveOccurred())
			Expect(entries).To(BeEmpty())
		})

		It("returns error for unexpected device name format", func() {
			devices := []models.Device{
				{
					Name:   "nodeonly",
					Status: models.DeviceStatus{Value: deviceStatusActive},
					OOBIp:  models.NestedIPAddress{Address: "10.0.0.1/24"},
				},
			}
			nb := makeNetboxMock(nil, "AA:BB:CC:DD:EE:FF")
			_, err := buildServerEntries(ctx, devices, cluster, nb)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unexpected device name format"))
		})

		It("builds server entries with oob, nic, and dpu interface types", func() {
			devices := []models.Device{
				{
					Name:       "node001-bb001",
					Status:     models.DeviceStatus{Value: deviceStatusActive},
					OOBIp:      models.NestedIPAddress{Address: "10.0.0.1/24"},
					Site:       models.NestedSite{Slug: "qa-de-1a"},
					DeviceType: models.NestedDeviceType{Slug: "powerserver-r2"},
					DeviceRole: models.NestedDeviceRole{Slug: "compute"},
					Platform:   models.NestedPlatform{Slug: "ubuntu-22-04"},
					Serial:     "SN12345",
				},
			}
			ifaces := []models.Interface{
				{
					Name:       "eth0",
					MacAddress: "AA:BB:CC:00:00:01",
					Type:       models.InterfaceType{Value: "1000base-t"},
				},
				{
					Name:       "smartnic0",
					MacAddress: "AA:BB:CC:00:00:02",
					Type:       models.InterfaceType{Value: "1000base-t"},
					Tags:       []models.NestedTag{{Name: dpuTagSmartNICMgmt}},
				},
			}
			nb := makeNetboxMock(ifaces, "AA:BB:CC:00:00:FF")

			entries, err := buildServerEntries(ctx, devices, cluster, nb)
			Expect(err).ToNot(HaveOccurred())
			Expect(entries).To(HaveLen(1))

			entry := entries[0]
			Expect(entry.Name).To(Equal("node001-bb001"))
			Expect(entry.Serial).To(Equal("SN12345"))
			Expect(entry.RemoteboardMAC).To(Equal("aa:bb:cc:00:00:ff"))

			// First interface should be oob (remoteboard prepended)
			Expect(entry.Interfaces[0].Type).To(Equal(moov1alpha1.InterfaceTypeOOB))
			Expect(entry.Interfaces[0].MAC).To(Equal("aa:bb:cc:00:00:ff"))

			// Find nic and dpu in remaining entries
			typeMap := map[moov1alpha1.InterfaceType]bool{}
			for _, iface := range entry.Interfaces[1:] {
				typeMap[iface.Type] = true
			}
			Expect(typeMap[moov1alpha1.InterfaceTypeNIC]).To(BeTrue())
			Expect(typeMap[moov1alpha1.InterfaceTypeDPU]).To(BeTrue())

			// All 11 label keys must be present
			expectedLabelKeys := []string{
				"topology.kubernetes.io/region",
				"topology.kubernetes.io/zone",
				"kubernetes.metal.cloud.sap/cluster",
				"kubernetes.metal.cloud.sap/cluster-type",
				"kubernetes.metal.cloud.sap/name",
				"kubernetes.metal.cloud.sap/nodename",
				"kubernetes.metal.cloud.sap/bb",
				"kubernetes.metal.cloud.sap/type",
				"kubernetes.metal.cloud.sap/role",
				"kubernetes.metal.cloud.sap/platform",
				"clusterctl.cluster.x-k8s.io/move",
			}
			for _, key := range expectedLabelKeys {
				Expect(entry.Labels).To(HaveKey(key), "missing label key: "+key)
			}
			Expect(entry.Labels["topology.kubernetes.io/region"]).To(Equal("eu-de-1"))
			Expect(entry.Labels["topology.kubernetes.io/zone"]).To(Equal("qa-de-1a"))
			Expect(entry.Labels["kubernetes.metal.cloud.sap/cluster"]).To(Equal("cluster-test"))
			Expect(entry.Labels["kubernetes.metal.cloud.sap/nodename"]).To(Equal("node001"))
			Expect(entry.Labels["kubernetes.metal.cloud.sap/bb"]).To(Equal("bb001"))
		})
	})

	Describe("reconcileMetalCluster", func() {
		It("creates a MetalCluster on the first call", func() {
			fakeClient := createFakeClient()
			r := &IronCoreReconciler{k8sClient: fakeClient, scheme: fakeClient.Scheme()}

			servers := []moov1alpha1.ServerEntry{
				{Name: "node001-bb001", Serial: "SN1"},
			}
			err := r.reconcileMetalCluster(ctx, "cluster-test", servers)
			Expect(err).ToNot(HaveOccurred())

			mc := &moov1alpha1.MetalCluster{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "cluster-test"}, mc)).To(Succeed())
			Expect(mc.Spec.Servers).To(HaveLen(1))
			Expect(mc.Spec.Servers[0].Name).To(Equal("node001-bb001"))
		})

		It("updates spec.servers when MetalCluster already exists", func() {
			existing := &moov1alpha1.MetalCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster-test"},
				Spec: moov1alpha1.MetalClusterSpec{
					Servers: []moov1alpha1.ServerEntry{{Name: "old-node-bb001", Serial: "OLD"}},
				},
			}
			fakeClient := createFakeClient(existing)
			r := &IronCoreReconciler{k8sClient: fakeClient, scheme: fakeClient.Scheme()}

			newServers := []moov1alpha1.ServerEntry{
				{Name: "node001-bb001", Serial: "NEW1"},
				{Name: "node002-bb001", Serial: "NEW2"},
			}
			err := r.reconcileMetalCluster(ctx, "cluster-test", newServers)
			Expect(err).ToNot(HaveOccurred())

			mc := &moov1alpha1.MetalCluster{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "cluster-test"}, mc)).To(Succeed())
			Expect(mc.Spec.Servers).To(HaveLen(2))
			names := []string{mc.Spec.Servers[0].Name, mc.Spec.Servers[1].Name}
			Expect(names).To(ConsistOf("node001-bb001", "node002-bb001"))
		})
	})

	Describe("groupByOOBCIDR", func() {
		It("groups devices in the same /24 into one entry", func() {
			devices := []models.Device{
				{Name: "node001-bb001", OOBIp: models.NestedIPAddress{Address: "10.1.2.10/24"}},
				{Name: "node002-bb001", OOBIp: models.NestedIPAddress{Address: "10.1.2.20/24"}},
			}
			result, err := groupByOOBCIDR(devices)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(HaveLen(1))
			Expect(result).To(HaveKey("10.1.2.0/24"))
			Expect(result["10.1.2.0/24"]).To(HaveLen(2))
		})

		It("produces separate entries for different subnets", func() {
			devices := []models.Device{
				{Name: "node001-bb001", OOBIp: models.NestedIPAddress{Address: "10.1.2.10/24"}},
				{Name: "node002-bb002", OOBIp: models.NestedIPAddress{Address: "10.1.3.20/24"}},
			}
			result, err := groupByOOBCIDR(devices)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(HaveLen(2))
			Expect(result).To(HaveKey("10.1.2.0/24"))
			Expect(result).To(HaveKey("10.1.3.0/24"))
		})

		It("skips devices with empty OOB IP", func() {
			devices := []models.Device{
				{Name: "node001-bb001", OOBIp: models.NestedIPAddress{Address: ""}},
				{Name: "node002-bb001", OOBIp: models.NestedIPAddress{Address: "10.1.2.20/24"}},
			}
			result, err := groupByOOBCIDR(devices)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(HaveLen(1))
			Expect(result["10.1.2.0/24"]).To(HaveLen(1))
		})

		It("returns error for unparseable OOB address", func() {
			devices := []models.Device{
				{Name: "node001-bb001", OOBIp: models.NestedIPAddress{Address: "not-an-ip"}},
			}
			_, err := groupByOOBCIDR(devices)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("oobSubnetName", func() {
		It("converts a /24 CIDR to the expected slug", func() {
			Expect(oobSubnetName("10.1.2.0/24")).To(Equal("oobsubnet-10-1-2-0-24"))
		})

		It("converts a /25 CIDR to the expected slug", func() {
			Expect(oobSubnetName("10.1.2.128/25")).To(Equal("oobsubnet-10-1-2-128-25"))
		})

		It("converts a /16 CIDR to the expected slug", func() {
			Expect(oobSubnetName("192.168.0.0/16")).To(Equal("oobsubnet-192-168-0-0-16"))
		})
	})

	Describe("buildStaticLeases", func() {
		It("builds leases with correct MAC, IP (host), and Hostname", func() {
			dcimMock := &mock.DCIMMock{
				GetInterfaceForDeviceFunc: func(device *models.Device, ifaceName string) (*models.Interface, error) {
					return &models.Interface{
						Name:       remoteboardInterfaceName,
						MacAddress: "AA:BB:CC:DD:EE:FF",
					}, nil
				},
			}
			nb := &mock.NetBoxMock{DCIMMock: dcimMock}

			devices := []models.Device{
				{
					Name:  "node001-bb001",
					OOBIp: models.NestedIPAddress{Address: "10.1.2.10/24"},
				},
			}

			leases, err := buildStaticLeases(ctx, devices, nb)
			Expect(err).ToNot(HaveOccurred())
			Expect(leases).To(HaveLen(1))
			Expect(leases[0].MAC).To(Equal("aa:bb:cc:dd:ee:ff"))
			Expect(leases[0].IP).To(Equal("10.1.2.10"))
			Expect(leases[0].Hostname).To(Equal("node001-bb001"))
		})

		It("skips devices with empty OOB IP", func() {
			dcimMock := &mock.DCIMMock{
				GetInterfaceForDeviceFunc: func(device *models.Device, ifaceName string) (*models.Interface, error) {
					return &models.Interface{MacAddress: "AA:BB:CC:DD:EE:FF"}, nil
				},
			}
			nb := &mock.NetBoxMock{DCIMMock: dcimMock}

			devices := []models.Device{
				{Name: "node001-bb001", OOBIp: models.NestedIPAddress{Address: ""}},
			}

			leases, err := buildStaticLeases(ctx, devices, nb)
			Expect(err).ToNot(HaveOccurred())
			Expect(leases).To(BeEmpty())
		})

		It("skips devices when remoteboard MAC retrieval fails", func() {
			dcimMock := &mock.DCIMMock{
				GetInterfaceForDeviceFunc: func(device *models.Device, ifaceName string) (*models.Interface, error) {
					return nil, errors.New("not found")
				},
			}
			nb := &mock.NetBoxMock{DCIMMock: dcimMock}

			devices := []models.Device{
				{Name: "node001-bb001", OOBIp: models.NestedIPAddress{Address: "10.1.2.10/24"}},
			}

			leases, err := buildStaticLeases(ctx, devices, nb)
			Expect(err).ToNot(HaveOccurred())
			Expect(leases).To(BeEmpty())
		})

		It("skips devices when remoteboard MAC is empty", func() {
			dcimMock := &mock.DCIMMock{
				GetInterfaceForDeviceFunc: func(device *models.Device, ifaceName string) (*models.Interface, error) {
					return &models.Interface{
						Name:       remoteboardInterfaceName,
						MacAddress: "",
					}, nil
				},
			}
			nb := &mock.NetBoxMock{DCIMMock: dcimMock}

			devices := []models.Device{
				{Name: "node001-bb001", OOBIp: models.NestedIPAddress{Address: "10.1.2.10/24"}},
			}

			leases, err := buildStaticLeases(ctx, devices, nb)
			Expect(err).ToNot(HaveOccurred())
			Expect(leases).To(BeEmpty())
		})
	})

	Describe("reconcileOOBSubnet", func() {
		It("creates an OOBSubnet with correct CIDR, gateway (first host), and leases", func() {
			fakeClient := createFakeClient()
			r := &IronCoreReconciler{k8sClient: fakeClient, scheme: fakeClient.Scheme()}

			leases := []metaldhcpv1alpha1.StaticLease{
				{MAC: "aa:bb:cc:dd:ee:01", IP: "10.1.2.10", Hostname: "node001-bb001"},
			}
			err := r.reconcileOOBSubnet(ctx, "10.1.2.0/24", leases)
			Expect(err).ToNot(HaveOccurred())

			subnet := &metaldhcpv1alpha1.OOBSubnet{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "oobsubnet-10-1-2-0-24"}, subnet)).To(Succeed())
			Expect(subnet.Spec.CIDR).To(Equal("10.1.2.0/24"))
			Expect(subnet.Spec.Gateway).To(Equal("10.1.2.1"))
			Expect(subnet.Spec.StaticLeases).To(HaveLen(1))
			Expect(subnet.Spec.StaticLeases[0].MAC).To(Equal("aa:bb:cc:dd:ee:01"))
		})

		It("derives the correct gateway for a /25 subnet", func() {
			fakeClient := createFakeClient()
			r := &IronCoreReconciler{k8sClient: fakeClient, scheme: fakeClient.Scheme()}

			err := r.reconcileOOBSubnet(ctx, "10.1.2.128/25", nil)
			Expect(err).ToNot(HaveOccurred())

			subnet := &metaldhcpv1alpha1.OOBSubnet{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "oobsubnet-10-1-2-128-25"}, subnet)).To(Succeed())
			Expect(subnet.Spec.Gateway).To(Equal("10.1.2.129"))
		})

		It("updates spec.staticLeases when OOBSubnet already exists", func() {
			existing := &metaldhcpv1alpha1.OOBSubnet{
				ObjectMeta: metav1.ObjectMeta{Name: "oobsubnet-10-1-2-0-24"},
				Spec: metaldhcpv1alpha1.OOBSubnetSpec{
					CIDR:    "10.1.2.0/24",
					Gateway: "10.1.2.1",
					StaticLeases: []metaldhcpv1alpha1.StaticLease{
						{MAC: "aa:bb:cc:dd:ee:00", IP: "10.1.2.9", Hostname: "old-node"},
					},
				},
			}
			fakeClient := createFakeClient(existing)
			r := &IronCoreReconciler{k8sClient: fakeClient, scheme: fakeClient.Scheme()}

			newLeases := []metaldhcpv1alpha1.StaticLease{
				{MAC: "aa:bb:cc:dd:ee:01", IP: "10.1.2.10", Hostname: "node001-bb001"},
				{MAC: "aa:bb:cc:dd:ee:02", IP: "10.1.2.11", Hostname: "node002-bb001"},
			}
			err := r.reconcileOOBSubnet(ctx, "10.1.2.0/24", newLeases)
			Expect(err).ToNot(HaveOccurred())

			subnet := &metaldhcpv1alpha1.OOBSubnet{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "oobsubnet-10-1-2-0-24"}, subnet)).To(Succeed())
			Expect(subnet.Spec.StaticLeases).To(HaveLen(2))
		})

		It("does not set bootURL, dnsServers, or leaseTime", func() {
			fakeClient := createFakeClient()
			r := &IronCoreReconciler{k8sClient: fakeClient, scheme: fakeClient.Scheme()}

			err := r.reconcileOOBSubnet(ctx, "10.1.2.0/24", nil)
			Expect(err).ToNot(HaveOccurred())

			subnet := &metaldhcpv1alpha1.OOBSubnet{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "oobsubnet-10-1-2-0-24"}, subnet)).To(Succeed())
			Expect(subnet.Spec.BootURL).To(BeEmpty())
			Expect(subnet.Spec.DNSServers).To(BeEmpty())
			Expect(subnet.Spec.LeaseTime).To(BeNil())
		})
	})
})
