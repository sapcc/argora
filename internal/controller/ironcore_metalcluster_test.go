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

	Describe("buildGroups", func() {
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
			_, groups, _, err := buildGroups(ctx, devices, cluster, nb)
			Expect(err).ToNot(HaveOccurred())
			Expect(groups).To(BeEmpty())
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
			_, _, _, err := buildGroups(ctx, devices, cluster, nb)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unexpected device name format"))
		})

		It("builds groups with global, group, and per-server labels; oob/nic/dpu interface types", func() {
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

			globalLabels, groups, macs, err := buildGroups(ctx, devices, cluster, nb)
			Expect(err).ToNot(HaveOccurred())
			Expect(groups).To(HaveLen(1))

			// Global labels: cluster-wide fields
			Expect(globalLabels).To(HaveKeyWithValue("topology.kubernetes.io/region", "eu-de-1"))
			Expect(globalLabels).To(HaveKeyWithValue("kubernetes.metal.cloud.sap/cluster", "cluster-test"))
			Expect(globalLabels).To(HaveKeyWithValue("kubernetes.metal.cloud.sap/cluster-type", "test-type"))
			Expect(globalLabels).To(HaveKeyWithValue("kubernetes.metal.cloud.sap/role", "compute"))
			Expect(globalLabels).To(HaveKeyWithValue("kubernetes.metal.cloud.sap/platform", "ubuntu-22-04"))
			Expect(globalLabels).To(HaveKey("clusterctl.cluster.x-k8s.io/move"))

			// Group labels: BB-level fields
			g := groups[0]
			Expect(g.Name).To(Equal("bb001"))
			Expect(g.Labels).To(HaveKeyWithValue("topology.kubernetes.io/zone", "qa-de-1a"))
			Expect(g.Labels).To(HaveKeyWithValue("kubernetes.metal.cloud.sap/bb", "bb001"))
			Expect(g.Labels).To(HaveKeyWithValue("kubernetes.metal.cloud.sap/type", "powerserver-r2"))

			// Server labels: per-device fields only
			Expect(g.Servers).To(HaveLen(1))
			entry := g.Servers[0]
			Expect(entry.Name).To(Equal("node001-bb001"))
			Expect(entry.Serial).To(Equal("SN12345"))
			Expect(entry.RemoteboardMAC).To(Equal("aa:bb:cc:00:00:ff"))
			Expect(entry.Labels).To(HaveKeyWithValue("kubernetes.metal.cloud.sap/name", "node001-bb001"))
			Expect(entry.Labels).To(HaveKeyWithValue("kubernetes.metal.cloud.sap/nodename", "node001"))
			Expect(entry.Labels).ToNot(HaveKey("topology.kubernetes.io/region"))
			Expect(entry.Labels).ToNot(HaveKey("topology.kubernetes.io/zone"))

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

			// remoteboardMACs populated
			Expect(macs).To(HaveKeyWithValue("node001-bb001", "aa:bb:cc:00:00:ff"))
		})

		It("overrides type at server level when device type differs from the BB's dominant type", func() {
			devices := []models.Device{
				{
					Name:       "node001-bb001",
					Status:     models.DeviceStatus{Value: deviceStatusActive},
					Site:       models.NestedSite{Slug: "qa-de-1a"},
					DeviceType: models.NestedDeviceType{Slug: "dl380-gen10"},
					DeviceRole: models.NestedDeviceRole{Slug: "compute"},
					Platform:   models.NestedPlatform{Slug: "gardenlinux"},
					Serial:     "SN1",
				},
				{
					Name:       "node002-bb001",
					Status:     models.DeviceStatus{Value: deviceStatusActive},
					Site:       models.NestedSite{Slug: "qa-de-1a"},
					DeviceType: models.NestedDeviceType{Slug: "dl380-gen12"},
					DeviceRole: models.NestedDeviceRole{Slug: "compute"},
					Platform:   models.NestedPlatform{Slug: "gardenlinux"},
					Serial:     "SN2",
				},
			}
			nb := makeNetboxMock(nil, "AA:BB:CC:DD:EE:FF")

			_, groups, _, err := buildGroups(ctx, devices, cluster, nb)
			Expect(err).ToNot(HaveOccurred())
			Expect(groups).To(HaveLen(1))
			g := groups[0]
			// Group type is set from first device
			Expect(g.Labels["kubernetes.metal.cloud.sap/type"]).To(Equal("dl380-gen10"))
			// First server has no type override (matches group)
			Expect(g.Servers[0].Labels).ToNot(HaveKey("kubernetes.metal.cloud.sap/type"))
			// Second server gets a per-server type override
			Expect(g.Servers[1].Labels).To(HaveKeyWithValue("kubernetes.metal.cloud.sap/type", "dl380-gen12"))
		})

		It("calls GetRegionForDevice only once for multiple devices sharing the same site", func() {
			devices := []models.Device{
				{
					Name:       "node001-bb001",
					Status:     models.DeviceStatus{Value: deviceStatusActive},
					Site:       models.NestedSite{ID: 7, Slug: "qa-de-1a"},
					DeviceRole: models.NestedDeviceRole{Slug: "compute"},
					Platform:   models.NestedPlatform{Slug: "gardenlinux"},
				},
				{
					Name:       "node002-bb001",
					Status:     models.DeviceStatus{Value: deviceStatusActive},
					Site:       models.NestedSite{ID: 7, Slug: "qa-de-1a"},
					DeviceRole: models.NestedDeviceRole{Slug: "compute"},
					Platform:   models.NestedPlatform{Slug: "gardenlinux"},
				},
			}
			dcimMock := &mock.DCIMMock{
				GetRegionForDeviceFunc: func(device *models.Device) (string, error) {
					return "eu-de-1", nil
				},
				GetInterfaceForDeviceFunc: func(device *models.Device, ifaceName string) (*models.Interface, error) {
					return &models.Interface{Name: remoteboardInterfaceName, MacAddress: "AA:BB:CC:DD:EE:FF"}, nil
				},
				GetInterfacesForDeviceFunc: func(device *models.Device) ([]models.Interface, error) {
					return nil, nil
				},
			}
			nb := &mock.NetBoxMock{DCIMMock: dcimMock}

			_, groups, _, err := buildGroups(ctx, devices, cluster, nb)
			Expect(err).ToNot(HaveOccurred())
			Expect(groups[0].Servers).To(HaveLen(2))
			Expect(dcimMock.GetRegionForDeviceCalls).To(Equal(1), "region should be fetched once and cached for the same site ID")
		})
	})

	Describe("reconcileMetalCluster", func() {
		It("creates a MetalCluster on the first call", func() {
			fakeClient := createFakeClient()
			r := &IronCoreReconciler{k8sClient: fakeClient, scheme: fakeClient.Scheme()}

			globalLabels := map[string]string{"kubernetes.metal.cloud.sap/cluster": "cluster-test"}
			groups := []moov1alpha1.Group{
				{Name: "bb001", Servers: []moov1alpha1.ServerEntry{{Name: "node001-bb001", Serial: "SN1"}}},
			}
			err := r.reconcileMetalCluster(ctx, "cluster-test", "import-test", globalLabels, groups)
			Expect(err).ToNot(HaveOccurred())

			mc := &moov1alpha1.MetalCluster{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "cluster-test"}, mc)).To(Succeed())
			Expect(mc.Spec.Groups).To(HaveLen(1))
			Expect(mc.Spec.Groups[0].Servers[0].Name).To(Equal("node001-bb001"))
			Expect(mc.Spec.Labels).To(HaveKeyWithValue("kubernetes.metal.cloud.sap/cluster", "cluster-test"))
			Expect(mc.Annotations).To(HaveKeyWithValue(managedByAnnotation, managedByArgora))
			Expect(mc.Labels).To(HaveKeyWithValue(clusterImportLabel, "import-test"))
		})

		It("updates spec.groups when MetalCluster already exists", func() {
			existing := &moov1alpha1.MetalCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster-test"},
				Spec: moov1alpha1.MetalClusterSpec{
					Groups: []moov1alpha1.Group{{Name: "bb001", Servers: []moov1alpha1.ServerEntry{{Name: "old-node-bb001", Serial: "OLD"}}}},
				},
			}
			fakeClient := createFakeClient(existing)
			r := &IronCoreReconciler{k8sClient: fakeClient, scheme: fakeClient.Scheme()}

			newGroups := []moov1alpha1.Group{
				{Name: "bb001", Servers: []moov1alpha1.ServerEntry{
					{Name: "node001-bb001", Serial: "NEW1"},
					{Name: "node002-bb001", Serial: "NEW2"},
				}},
			}
			err := r.reconcileMetalCluster(ctx, "cluster-test", "import-test", nil, newGroups)
			Expect(err).ToNot(HaveOccurred())

			mc := &moov1alpha1.MetalCluster{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "cluster-test"}, mc)).To(Succeed())
			Expect(mc.Spec.Groups).To(HaveLen(1))
			Expect(mc.Spec.Groups[0].Servers).To(HaveLen(2))
			names := []string{mc.Spec.Groups[0].Servers[0].Name, mc.Spec.Groups[0].Servers[1].Name}
			Expect(names).To(ConsistOf("node001-bb001", "node002-bb001"))
			Expect(mc.Annotations).To(HaveKeyWithValue(managedByAnnotation, managedByArgora))
			Expect(mc.Labels).To(HaveKeyWithValue(clusterImportLabel, "import-test"))
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

		It("returns error for unparsable OOB address", func() {
			devices := []models.Device{
				{Name: "node001-bb001", OOBIp: models.NestedIPAddress{Address: "not-an-ip"}},
			}
			_, err := groupByOOBCIDR(devices)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("subnetName", func() {
		It("converts a /24 CIDR to the expected slug", func() {
			Expect(subnetName("10.1.2.0/24")).To(Equal("subnet-10-1-2-0-24"))
		})

		It("converts a /25 CIDR to the expected slug", func() {
			Expect(subnetName("10.1.2.128/25")).To(Equal("subnet-10-1-2-128-25"))
		})

		It("converts a /16 CIDR to the expected slug", func() {
			Expect(subnetName("192.168.0.0/16")).To(Equal("subnet-192-168-0-0-16"))
		})
	})

	Describe("buildStaticLeases", func() {
		It("builds leases with correct MAC, IP (host), and Hostname", func() {
			devices := []models.Device{
				{
					Name:  "node001-bb001",
					OOBIp: models.NestedIPAddress{Address: "10.1.2.10/24"},
				},
			}
			macs := map[string]string{"node001-bb001": "aa:bb:cc:dd:ee:ff"}

			leases, err := buildStaticLeases(ctx, devices, macs)
			Expect(err).ToNot(HaveOccurred())
			Expect(leases).To(HaveLen(1))
			Expect(leases[0].MAC).To(Equal("aa:bb:cc:dd:ee:ff"))
			Expect(leases[0].IP).To(Equal("10.1.2.10"))
			Expect(leases[0].Hostname).To(Equal("node001-bb001"))
		})

		It("skips devices with empty OOB IP", func() {
			devices := []models.Device{
				{Name: "node001-bb001", OOBIp: models.NestedIPAddress{Address: ""}},
			}
			macs := map[string]string{"node001-bb001": "aa:bb:cc:dd:ee:ff"}

			leases, err := buildStaticLeases(ctx, devices, macs)
			Expect(err).ToNot(HaveOccurred())
			Expect(leases).To(BeEmpty())
		})

		It("skips devices not present in the remoteboard MACs map", func() {
			devices := []models.Device{
				{Name: "node001-bb001", OOBIp: models.NestedIPAddress{Address: "10.1.2.10/24"}},
			}

			leases, err := buildStaticLeases(ctx, devices, map[string]string{})
			Expect(err).ToNot(HaveOccurred())
			Expect(leases).To(BeEmpty())
		})

		It("skips devices when remoteboard MAC is empty", func() {
			devices := []models.Device{
				{Name: "node001-bb001", OOBIp: models.NestedIPAddress{Address: "10.1.2.10/24"}},
			}
			macs := map[string]string{"node001-bb001": ""}

			leases, err := buildStaticLeases(ctx, devices, macs)
			Expect(err).ToNot(HaveOccurred())
			Expect(leases).To(BeEmpty())
		})
	})

	Describe("reconcileSubnet", func() {
		It("creates a Subnet with correct CIDR, gateway (first host), and leases", func() {
			fakeClient := createFakeClient()
			r := &IronCoreReconciler{k8sClient: fakeClient, scheme: fakeClient.Scheme()}

			leases := []metaldhcpv1alpha1.StaticLease{
				{MAC: "aa:bb:cc:dd:ee:01", IP: "10.1.2.10", Hostname: "node001-bb001"},
			}
			err := r.reconcileSubnet(ctx, "10.1.2.0/24", "cluster-test", leases)
			Expect(err).ToNot(HaveOccurred())

			subnet := &metaldhcpv1alpha1.Subnet{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "subnet-10-1-2-0-24"}, subnet)).To(Succeed())
			Expect(subnet.Spec.CIDR).To(Equal("10.1.2.0/24"))
			Expect(subnet.Spec.Gateway).To(Equal("10.1.2.1"))
			Expect(subnet.Spec.StaticLeases).To(HaveLen(1))
			Expect(subnet.Spec.StaticLeases[0].MAC).To(Equal("aa:bb:cc:dd:ee:01"))
			Expect(subnet.Annotations).To(HaveKeyWithValue(managedByAnnotation, managedByArgora))
			Expect(subnet.Labels).To(HaveKeyWithValue(clusterLabel, "cluster-test"))
		})

		It("takes over a helm-managed Subnet by updating the annotation to argora", func() {
			existing := &metaldhcpv1alpha1.Subnet{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "subnet-10-1-2-0-24",
					Annotations: map[string]string{managedByAnnotation: managedByHelm},
				},
				Spec: metaldhcpv1alpha1.SubnetSpec{
					CIDR:    "10.1.2.0/24",
					Gateway: "10.1.2.1",
				},
			}
			fakeClient := createFakeClient(existing)
			r := &IronCoreReconciler{k8sClient: fakeClient, scheme: fakeClient.Scheme()}

			err := r.reconcileSubnet(ctx, "10.1.2.0/24", "cluster-test", nil)
			Expect(err).ToNot(HaveOccurred())

			subnet := &metaldhcpv1alpha1.Subnet{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "subnet-10-1-2-0-24"}, subnet)).To(Succeed())
			Expect(subnet.Annotations).To(HaveKeyWithValue(managedByAnnotation, managedByArgora))
		})

		It("derives the correct gateway for a /25 subnet", func() {
			fakeClient := createFakeClient()
			r := &IronCoreReconciler{k8sClient: fakeClient, scheme: fakeClient.Scheme()}

			err := r.reconcileSubnet(ctx, "10.1.2.128/25", "cluster-test", nil)
			Expect(err).ToNot(HaveOccurred())

			subnet := &metaldhcpv1alpha1.Subnet{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "subnet-10-1-2-128-25"}, subnet)).To(Succeed())
			Expect(subnet.Spec.Gateway).To(Equal("10.1.2.129"))
		})

		It("updates spec.staticLeases when Subnet already exists", func() {
			existing := &metaldhcpv1alpha1.Subnet{
				ObjectMeta: metav1.ObjectMeta{Name: "subnet-10-1-2-0-24"},
				Spec: metaldhcpv1alpha1.SubnetSpec{
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
			err := r.reconcileSubnet(ctx, "10.1.2.0/24", "cluster-test", newLeases)
			Expect(err).ToNot(HaveOccurred())

			subnet := &metaldhcpv1alpha1.Subnet{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "subnet-10-1-2-0-24"}, subnet)).To(Succeed())
			Expect(subnet.Spec.StaticLeases).To(HaveLen(2))
		})

		It("does not set bootURL, dnsServers, or leaseTime", func() {
			fakeClient := createFakeClient()
			r := &IronCoreReconciler{k8sClient: fakeClient, scheme: fakeClient.Scheme()}

			err := r.reconcileSubnet(ctx, "10.1.2.0/24", "cluster-test", nil)
			Expect(err).ToNot(HaveOccurred())

			subnet := &metaldhcpv1alpha1.Subnet{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "subnet-10-1-2-0-24"}, subnet)).To(Succeed())
			Expect(subnet.Spec.BootURL).To(BeEmpty())
			Expect(subnet.Spec.DNSServers).To(BeEmpty())
			Expect(subnet.Spec.LeaseTime).To(BeNil())
		})
	})

	Describe("pruneSubnets", func() {
		makeSubnet := func(name, cidr, managedBy, cluster string) *metaldhcpv1alpha1.Subnet {
			annotations := map[string]string{}
			if managedBy != "" {
				annotations[managedByAnnotation] = managedBy
			}
			labels := map[string]string{}
			if cluster != "" {
				labels[clusterLabel] = cluster
			}
			return &metaldhcpv1alpha1.Subnet{
				ObjectMeta: metav1.ObjectMeta{Name: name, Annotations: annotations, Labels: labels},
				Spec:       metaldhcpv1alpha1.SubnetSpec{CIDR: cidr, Gateway: "10.1.2.1"},
			}
		}

		It("deletes argora-managed Subnets not in the active CIDR set", func() {
			stale := makeSubnet("subnet-10-1-2-0-24", "10.1.2.0/24", managedByArgora, "cluster-test")
			fakeClient := createFakeClient(stale)
			r := &IronCoreReconciler{k8sClient: fakeClient, scheme: fakeClient.Scheme()}

			err := r.pruneSubnets(ctx, map[string][]models.Device{}, "cluster-test")
			Expect(err).ToNot(HaveOccurred())

			subnet := &metaldhcpv1alpha1.Subnet{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "subnet-10-1-2-0-24"}, subnet)).ToNot(Succeed())
		})

		It("does not delete helm-managed Subnets", func() {
			helmSubnet := makeSubnet("subnet-10-1-2-0-24", "10.1.2.0/24", managedByHelm, "cluster-test")
			fakeClient := createFakeClient(helmSubnet)
			r := &IronCoreReconciler{k8sClient: fakeClient, scheme: fakeClient.Scheme()}

			err := r.pruneSubnets(ctx, map[string][]models.Device{}, "cluster-test")
			Expect(err).ToNot(HaveOccurred())

			subnet := &metaldhcpv1alpha1.Subnet{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "subnet-10-1-2-0-24"}, subnet)).To(Succeed())
		})

		It("does not delete unannotated Subnets", func() {
			unannotated := makeSubnet("subnet-10-1-2-0-24", "10.1.2.0/24", "", "cluster-test")
			fakeClient := createFakeClient(unannotated)
			r := &IronCoreReconciler{k8sClient: fakeClient, scheme: fakeClient.Scheme()}

			err := r.pruneSubnets(ctx, map[string][]models.Device{}, "cluster-test")
			Expect(err).ToNot(HaveOccurred())

			subnet := &metaldhcpv1alpha1.Subnet{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "subnet-10-1-2-0-24"}, subnet)).To(Succeed())
		})

		It("does not delete argora-managed Subnets that are still active", func() {
			active := makeSubnet("subnet-10-1-2-0-24", "10.1.2.0/24", managedByArgora, "cluster-test")
			fakeClient := createFakeClient(active)
			r := &IronCoreReconciler{k8sClient: fakeClient, scheme: fakeClient.Scheme()}

			activeCIDRs := map[string][]models.Device{"10.1.2.0/24": {}}
			err := r.pruneSubnets(ctx, activeCIDRs, "cluster-test")
			Expect(err).ToNot(HaveOccurred())

			subnet := &metaldhcpv1alpha1.Subnet{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "subnet-10-1-2-0-24"}, subnet)).To(Succeed())
		})

		It("does not delete argora-managed Subnets belonging to a different cluster", func() {
			otherCluster := makeSubnet("subnet-10-1-3-0-24", "10.1.3.0/24", managedByArgora, "other-cluster")
			fakeClient := createFakeClient(otherCluster)
			r := &IronCoreReconciler{k8sClient: fakeClient, scheme: fakeClient.Scheme()}

			err := r.pruneSubnets(ctx, map[string][]models.Device{}, "cluster-test")
			Expect(err).ToNot(HaveOccurred())

			subnet := &metaldhcpv1alpha1.Subnet{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "subnet-10-1-3-0-24"}, subnet)).To(Succeed())
		})
	})

	Describe("pruneMetalClusters", func() {
		makeMetalCluster := func(name, managedBy, importName string) *moov1alpha1.MetalCluster {
			annotations := map[string]string{}
			if managedBy != "" {
				annotations[managedByAnnotation] = managedBy
			}
			labels := map[string]string{}
			if importName != "" {
				labels[clusterImportLabel] = importName
			}
			return &moov1alpha1.MetalCluster{
				ObjectMeta: metav1.ObjectMeta{Name: name, Annotations: annotations, Labels: labels},
			}
		}

		It("deletes argora-managed MetalClusters not in the active set", func() {
			stale := makeMetalCluster("cluster-stale", managedByArgora, "import-test")
			fakeClient := createFakeClient(stale)
			r := &IronCoreReconciler{k8sClient: fakeClient, scheme: fakeClient.Scheme()}

			err := r.pruneMetalClusters(ctx, map[string]struct{}{}, "import-test")
			Expect(err).ToNot(HaveOccurred())

			mc := &moov1alpha1.MetalCluster{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "cluster-stale"}, mc)).ToNot(Succeed())
		})

		It("does not delete helm-managed MetalClusters", func() {
			helmMC := makeMetalCluster("cluster-helm", managedByHelm, "import-test")
			fakeClient := createFakeClient(helmMC)
			r := &IronCoreReconciler{k8sClient: fakeClient, scheme: fakeClient.Scheme()}

			err := r.pruneMetalClusters(ctx, map[string]struct{}{}, "import-test")
			Expect(err).ToNot(HaveOccurred())

			mc := &moov1alpha1.MetalCluster{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "cluster-helm"}, mc)).To(Succeed())
		})

		It("does not delete unannotated MetalClusters", func() {
			unannotated := makeMetalCluster("cluster-noanno", "", "import-test")
			fakeClient := createFakeClient(unannotated)
			r := &IronCoreReconciler{k8sClient: fakeClient, scheme: fakeClient.Scheme()}

			err := r.pruneMetalClusters(ctx, map[string]struct{}{}, "import-test")
			Expect(err).ToNot(HaveOccurred())

			mc := &moov1alpha1.MetalCluster{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "cluster-noanno"}, mc)).To(Succeed())
		})

		It("does not delete active argora-managed MetalClusters", func() {
			active := makeMetalCluster("cluster-active", managedByArgora, "import-test")
			fakeClient := createFakeClient(active)
			r := &IronCoreReconciler{k8sClient: fakeClient, scheme: fakeClient.Scheme()}

			err := r.pruneMetalClusters(ctx, map[string]struct{}{"cluster-active": {}}, "import-test")
			Expect(err).ToNot(HaveOccurred())

			mc := &moov1alpha1.MetalCluster{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "cluster-active"}, mc)).To(Succeed())
		})

		It("does not delete argora-managed MetalClusters belonging to a different ClusterImport", func() {
			otherImport := makeMetalCluster("cluster-other", managedByArgora, "other-import")
			fakeClient := createFakeClient(otherImport)
			r := &IronCoreReconciler{k8sClient: fakeClient, scheme: fakeClient.Scheme()}

			err := r.pruneMetalClusters(ctx, map[string]struct{}{}, "import-test")
			Expect(err).ToNot(HaveOccurred())

			mc := &moov1alpha1.MetalCluster{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "cluster-other"}, mc)).To(Succeed())
		})
	})
})
