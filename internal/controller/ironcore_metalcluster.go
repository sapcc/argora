// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"net/netip"
	"strings"

	moov1alpha1 "github.com/SAP-cloud-infrastructure/metal-onboarding-operator/api/v1alpha1"
	metaldhcpv1alpha1 "github.com/SAP-cloud-infrastructure/metaldhcp/api/v1alpha1"
	"github.com/sapcc/go-netbox-go/models"

	"github.com/sapcc/argora/internal/netbox"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	dpuTagSmartNICMgmt = "SmartNIC-mgmt"
	dpuTagSmartNICNIC1 = "SmartNIC-NIC1"

	managedByAnnotation = "dhcp.metal.ironcore.dev/managed-by"
	managedByArgora     = "argora"
	managedByHelm       = "helm"

	clusterLabel       = "argora.cloud.sap/cluster"
	clusterImportLabel = "argora.cloud.sap/cluster-import"
)

// buildServerEntries converts a slice of Netbox devices into MOO ServerEntry objects.
// Inactive devices are skipped. For each active device it fetches interfaces from Netbox
// to populate the Interfaces list and builds the required label set.
func buildServerEntries(ctx context.Context, devices []models.Device, cluster *models.Cluster, nb netbox.Netbox) ([]moov1alpha1.ServerEntry, error) {
	logger := log.FromContext(ctx)
	entries := make([]moov1alpha1.ServerEntry, 0, len(devices))
	regionCache := make(map[int]string)

	for i := range devices {
		device := &devices[i]
		if device.Status.Value != deviceStatusActive {
			logger.Info("skipping inactive device", "device", device.Name, "status", device.Status.Value)
			continue
		}

		deviceNameParts := strings.Split(device.Name, "-")
		if len(deviceNameParts) != 2 {
			return nil, fmt.Errorf("unexpected device name format %q: expected exactly one dash", device.Name)
		}

		region, ok := regionCache[device.Site.ID]
		if !ok {
			var err error
			region, err = nb.DCIM().GetRegionForDevice(device)
			if err != nil {
				return nil, fmt.Errorf("unable to get region for device %s: %w", device.Name, err)
			}
			regionCache[device.Site.ID] = region
		}

		remoteboardMAC, remoteboardEntry, err := getRemoteboardEntry(nb, device)
		if err != nil {
			logger.Info("unable to get remoteboard interface, continuing without it", "device", device.Name, "error", err)
		}

		ifaces, err := nb.DCIM().GetInterfacesForDevice(device)
		if err != nil {
			return nil, fmt.Errorf("unable to get interfaces for device %s: %w", device.Name, err)
		}

		ifaceEntries := buildInterfaceEntries(ifaces)
		if remoteboardEntry != nil {
			ifaceEntries = append([]moov1alpha1.InterfaceEntry{*remoteboardEntry}, ifaceEntries...)
		}

		labels := map[string]string{
			"topology.kubernetes.io/region":           region,
			"topology.kubernetes.io/zone":             device.Site.Slug,
			"kubernetes.metal.cloud.sap/cluster":      cluster.Name,
			"kubernetes.metal.cloud.sap/cluster-type": cluster.Type.Slug,
			"kubernetes.metal.cloud.sap/name":         device.Name,
			"kubernetes.metal.cloud.sap/nodename":     deviceNameParts[0],
			"kubernetes.metal.cloud.sap/bb":           deviceNameParts[1],
			"kubernetes.metal.cloud.sap/type":         device.DeviceType.Slug,
			"kubernetes.metal.cloud.sap/role":         device.DeviceRole.Slug,
			"kubernetes.metal.cloud.sap/platform":     device.Platform.Slug,
			"clusterctl.cluster.x-k8s.io/move":        "",
		}

		entries = append(entries, moov1alpha1.ServerEntry{
			Name:           device.Name,
			Serial:         device.Serial,
			RemoteboardMAC: remoteboardMAC,
			Interfaces:     ifaceEntries,
			Labels:         labels,
		})
	}

	return entries, nil
}

// getRemoteboardEntry fetches the remoteboard interface MAC and returns both the MAC string
// and an InterfaceEntry with type oob. Returns empty string and nil entry on error (non-fatal).
func getRemoteboardEntry(nb netbox.Netbox, device *models.Device) (string, *moov1alpha1.InterfaceEntry, error) {
	iface, err := nb.DCIM().GetInterfaceForDevice(device, remoteboardInterfaceName)
	if err != nil {
		return "", nil, err
	}
	if iface.MacAddress == "" {
		return "", nil, nil
	}
	mac := strings.ToLower(iface.MacAddress)
	return mac, &moov1alpha1.InterfaceEntry{
		Type: moov1alpha1.InterfaceTypeOOB,
		MAC:  mac,
		Name: iface.Name,
	}, nil
}

// buildInterfaceEntries maps non-remoteboard, non-mgmtonly, non-lag, non-empty-MAC interfaces
// to MOO InterfaceEntry objects with types nic or dpu.
func buildInterfaceEntries(ifaces []models.Interface) []moov1alpha1.InterfaceEntry {
	entries := make([]moov1alpha1.InterfaceEntry, 0, len(ifaces))
	for _, iface := range ifaces {
		if iface.MgmtOnly {
			continue
		}
		if strings.ToLower(iface.Type.Value) == interfaceTypeLag {
			continue
		}
		if iface.MacAddress == "" {
			continue
		}
		if iface.Name == remoteboardInterfaceName {
			continue
		}
		ifaceType := moov1alpha1.InterfaceTypeNIC
		for _, tag := range iface.Tags {
			if tag.Name == dpuTagSmartNICMgmt || tag.Name == dpuTagSmartNICNIC1 {
				ifaceType = moov1alpha1.InterfaceTypeDPU
				break
			}
		}
		entries = append(entries, moov1alpha1.InterfaceEntry{
			Type: ifaceType,
			MAC:  strings.ToLower(iface.MacAddress),
			Name: iface.Name,
		})
	}
	return entries
}

// reconcileMetalCluster creates or updates the MetalCluster CR for a cluster.
// It stamps annotation managedByAnnotation and label clusterImportLabel so the pruner can scope deletions.
func (r *IronCoreReconciler) reconcileMetalCluster(ctx context.Context, clusterName, clusterImportName string, servers []moov1alpha1.ServerEntry) error {
	logger := log.FromContext(ctx)
	logger.Info("reconciling MetalCluster", "cluster", clusterName)

	mc := &moov1alpha1.MetalCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterName,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.k8sClient, mc, func() error {
		if mc.Annotations == nil {
			mc.Annotations = make(map[string]string)
		}
		mc.Annotations[managedByAnnotation] = managedByArgora
		if mc.Labels == nil {
			mc.Labels = make(map[string]string)
		}
		mc.Labels[clusterImportLabel] = clusterImportName
		mc.Spec.Servers = servers
		return nil
	})
	if err != nil {
		return fmt.Errorf("unable to create or update MetalCluster %s: %w", clusterName, err)
	}

	return nil
}

// groupByOOBCIDR groups devices by the network address of their OOB IP (derived by masking).
// The map key is the CIDR string (e.g. "10.1.2.0/24").
func groupByOOBCIDR(devices []models.Device) (map[string][]models.Device, error) {
	result := make(map[string][]models.Device)
	for _, device := range devices {
		if device.OOBIp.Address == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(device.OOBIp.Address)
		if err != nil {
			return nil, fmt.Errorf("unable to parse OOB IP %q for device %s: %w", device.OOBIp.Address, device.Name, err)
		}
		cidr := prefix.Masked().String()
		result[cidr] = append(result[cidr], device)
	}
	return result, nil
}

// oobSubnetName converts a CIDR string into the OOBSubnet CR name.
// E.g. "10.1.2.0/24" → "oobsubnet-10-1-2-0-24".
func oobSubnetName(cidr string) string {
	r := strings.NewReplacer(".", "-", "/", "-")
	return "oobsubnet-" + r.Replace(cidr)
}

// buildStaticLeases builds the static lease list for an OOBSubnet from a device slice.
// remoteboardMACs is a device-name → MAC map pre-populated by buildServerEntries, so no
// additional Netbox calls are needed here. Devices with no entry or an empty MAC are skipped.
func buildStaticLeases(ctx context.Context, devices []models.Device, remoteboardMACs map[string]string) ([]metaldhcpv1alpha1.StaticLease, error) {
	logger := log.FromContext(ctx)
	leases := make([]metaldhcpv1alpha1.StaticLease, 0, len(devices))

	for i := range devices {
		device := &devices[i]
		if device.OOBIp.Address == "" {
			continue
		}

		prefix, err := netip.ParsePrefix(device.OOBIp.Address)
		if err != nil {
			return nil, fmt.Errorf("unable to parse OOB IP %q for device %s: %w", device.OOBIp.Address, device.Name, err)
		}
		hostIP := prefix.Addr().String()

		mac := remoteboardMACs[device.Name]
		if mac == "" {
			logger.Info("no remoteboard MAC for device, skipping static lease", "device", device.Name)
			continue
		}

		leases = append(leases, metaldhcpv1alpha1.StaticLease{
			MAC:      mac,
			IP:       hostIP,
			Hostname: device.Name,
		})
	}

	return leases, nil
}

// reconcileOOBSubnet creates or updates an OOBSubnet CR for the given CIDR. It sets only
// spec.cidr, spec.gateway, and spec.staticLeases — other fields are left for external management.
// The subnet is labelled with clusterName so the pruner can scope deletions per cluster.
func (r *IronCoreReconciler) reconcileOOBSubnet(ctx context.Context, cidr, clusterName string, leases []metaldhcpv1alpha1.StaticLease) error {
	logger := log.FromContext(ctx)
	name := oobSubnetName(cidr)
	logger.Info("reconciling OOBSubnet", "cidr", cidr, "name", name)

	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return fmt.Errorf("unable to parse CIDR %q: %w", cidr, err)
	}
	gateway := prefix.Masked().Addr().Next().String()

	subnet := &metaldhcpv1alpha1.OOBSubnet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	_, err = controllerutil.CreateOrUpdate(ctx, r.k8sClient, subnet, func() error {
		if subnet.Annotations == nil {
			subnet.Annotations = make(map[string]string)
		}
		subnet.Annotations[managedByAnnotation] = managedByArgora
		if subnet.Labels == nil {
			subnet.Labels = make(map[string]string)
		}
		subnet.Labels[clusterLabel] = clusterName
		subnet.Spec.CIDR = cidr
		subnet.Spec.Gateway = gateway
		subnet.Spec.StaticLeases = leases
		return nil
	})
	if err != nil {
		return fmt.Errorf("unable to create or update OOBSubnet %s: %w", name, err)
	}

	return nil
}

// reconcileMetalClusterMode handles the syncMetalCluster branch for one Netbox cluster.
// It builds the MetalCluster and OOBSubnet CRs from the full device list.
func (r *IronCoreReconciler) reconcileMetalClusterMode(ctx context.Context, clusterImportName string, cluster *models.Cluster, devices []models.Device, nb netbox.Netbox) error {
	servers, err := buildServerEntries(ctx, devices, cluster, nb)
	if err != nil {
		return fmt.Errorf("unable to build server entries for cluster %s: %w", cluster.Name, err)
	}

	if err := r.reconcileMetalCluster(ctx, cluster.Name, clusterImportName, servers); err != nil {
		return err
	}

	cidrMap, err := groupByOOBCIDR(devices)
	if err != nil {
		return fmt.Errorf("unable to group devices by OOB CIDR for cluster %s: %w", cluster.Name, err)
	}

	remoteboardMACs := make(map[string]string, len(servers))
	for _, entry := range servers {
		remoteboardMACs[entry.Name] = entry.RemoteboardMAC
	}

	for cidr, cidrDevices := range cidrMap {
		leases, err := buildStaticLeases(ctx, cidrDevices, remoteboardMACs)
		if err != nil {
			return fmt.Errorf("unable to build static leases for CIDR %s: %w", cidr, err)
		}
		if err := r.reconcileOOBSubnet(ctx, cidr, cluster.Name, leases); err != nil {
			return err
		}
	}

	return r.pruneOOBSubnets(ctx, cidrMap, cluster.Name)
}

// pruneOOBSubnets deletes any argora-managed OOBSubnets for the given cluster whose CIDR is
// no longer in the active set. Subnets belonging to other clusters, or managed by helm or any
// other owner, are never touched.
func (r *IronCoreReconciler) pruneOOBSubnets(ctx context.Context, activeCIDRs map[string][]models.Device, clusterName string) error {
	logger := log.FromContext(ctx)
	existing := &metaldhcpv1alpha1.OOBSubnetList{}
	if err := r.k8sClient.List(ctx, existing, client.MatchingLabels{clusterLabel: clusterName}); err != nil {
		return fmt.Errorf("unable to list OOBSubnets: %w", err)
	}
	for i := range existing.Items {
		subnet := &existing.Items[i]
		if subnet.Annotations[managedByAnnotation] != managedByArgora {
			continue
		}
		if _, stillNeeded := activeCIDRs[subnet.Spec.CIDR]; stillNeeded {
			continue
		}
		logger.Info("deleting stale OOBSubnet", "name", subnet.Name, "cidr", subnet.Spec.CIDR)
		if err := r.k8sClient.Delete(ctx, subnet); err != nil {
			return fmt.Errorf("unable to delete stale OOBSubnet %s: %w", subnet.Name, err)
		}
	}
	return nil
}

// pruneMetalClusters deletes any argora-managed MetalClusters owned by the given ClusterImport
// whose name is no longer in the active set. MetalClusters owned by other ClusterImports, or
// managed by any owner other than argora, are never touched.
func (r *IronCoreReconciler) pruneMetalClusters(ctx context.Context, activeClusters map[string]struct{}, clusterImportName string) error {
	logger := log.FromContext(ctx)
	existing := &moov1alpha1.MetalClusterList{}
	if err := r.k8sClient.List(ctx, existing, client.MatchingLabels{clusterImportLabel: clusterImportName}); err != nil {
		return fmt.Errorf("unable to list MetalClusters: %w", err)
	}
	for i := range existing.Items {
		mc := &existing.Items[i]
		if mc.Annotations[managedByAnnotation] != managedByArgora {
			continue
		}
		if _, stillNeeded := activeClusters[mc.Name]; stillNeeded {
			continue
		}
		logger.Info("deleting stale MetalCluster", "name", mc.Name)
		if err := r.k8sClient.Delete(ctx, mc); err != nil {
			return fmt.Errorf("unable to delete stale MetalCluster %s: %w", mc.Name, err)
		}
	}
	return nil
}
