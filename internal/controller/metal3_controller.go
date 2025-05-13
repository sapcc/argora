// Copyright 2024 SAP SE
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"golang.org/x/time/rate"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"

	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/dspinhirne/netaddr-go/v2"
	"github.com/sapcc/go-netbox-go/models"
	"gopkg.in/yaml.v3"

	"github.com/sapcc/argora/internal/config"
	"github.com/sapcc/argora/internal/netbox"
	"github.com/sapcc/argora/internal/networkdata"
)

const ClusterRoleLabel = "discovery.inf.sap.cloud/clusterRole"

var (
	rootHintMap = map[string]string{
		"poweredge-r660":       "BOSS",
		"poweredge-r640":       "BOSS",
		"poweredge-r840":       "BOSS",
		"poweredge-r7615":      "BOSS",
		"thinksystem-sr650":    "ThinkSystem M.2 VD",
		"sr655-v3":             "NVMe 2-Bay",
		"thinksystem-sr650-v3": "NVMe 2-Bay",
		"proliant-dl320-gen11": "HPE NS204i-u Gen11 Boot Controller",
	}

	linkHintMapCeph = map[string]string{
		"ThinkSystem SR650":    "en*f0np*",
		"ThinkSystem SR650 v3": "en*1f*np*",
		"Thinksystem SR655 v3": "en*f*np*",
		"PowerEdge R640":       "en*f1np*",
		"PowerEdge R660":       "en*f1np*",
		"PowerEdge R7615":      "en*f*np*",
		"Proliant DL320 Gen11": "en*f1np*",
	}

	linkHintMapKvm = map[string]string{
		"PowerEdge R640": "en*f0np*",
		"PowerEdge R840": "en*f0np*",
	}
)

type Metal3Reconciler struct {
	k8sClient         client.Client
	scheme            *runtime.Scheme
	cfg               *config.Config
	netBox            netbox.Netbox
	reconcileInterval time.Duration
}

func NewMetal3Reconciler(k8sClient client.Client, scheme *runtime.Scheme, cfg *config.Config, netBox netbox.Netbox, reconcileInterval time.Duration) *Metal3Reconciler {
	return &Metal3Reconciler{
		k8sClient:         k8sClient,
		scheme:            scheme,
		cfg:               cfg,
		netBox:            netBox,
		reconcileInterval: reconcileInterval,
	}
}

func (r *Metal3Reconciler) SetupWithManager(mgr manager.Manager, rateLimiter RateLimiter) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&clusterv1.Cluster{}).
		WithEventFilter(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.AnnotationChangedPredicate{})).
		WithOptions(controller.Options{
			RateLimiter: workqueue.NewTypedMaxOfRateLimiter(
				workqueue.NewTypedItemExponentialFailureRateLimiter[ctrl.Request](rateLimiter.BaseDelay,
					rateLimiter.FailureMaxDelay),
				&workqueue.TypedBucketRateLimiter[ctrl.Request]{
					Limiter: rate.NewLimiter(rate.Limit(rateLimiter.Frequency), rateLimiter.Burst),
				},
			),
		}).
		Named("metal3").
		Complete(r)
}

// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metal3.io,resources=baremetalhosts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile looks up a cluster in netbox and creates baremetal hosts for it
func (r *Metal3Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("reconciling metal3")

	err := r.cfg.Reload()
	if err != nil {
		logger.Error(err, "unable to reload configuration")
		return ctrl.Result{}, err
	}

	logger.Info("configuration reloaded", "config", r.cfg)

	if r.cfg.ServerController != config.ControllerTypeMetal3 {
		logger.Info("metal3 controller not enabled")
		return ctrl.Result{}, nil
	}

	err = r.netBox.Reload(r.cfg.NetboxURL, r.cfg.NetboxToken, logger)
	if err != nil {
		logger.Error(err, "unable to reload netbox")
		return ctrl.Result{}, err
	}

	capiCluster := &clusterv1.Cluster{}
	err = r.k8sClient.Get(ctx, req.NamespacedName, capiCluster)
	if err != nil {
		logger.Error(err, "unable to get CAPI cluster")
		return ctrl.Result{}, err
	}

	clusterType := capiCluster.Labels[ClusterRoleLabel]
	logger.Info("fetching clusters data", "name", capiCluster.Name, "type", clusterType)

	clusters, err := r.netBox.Virtualization().GetClustersByNameRegionType(capiCluster.Name, "", clusterType)
	if err != nil {
		logger.Error(err, "unable to find cluster in netbox", "name", capiCluster.Name, "type", clusterType)
		return ctrl.Result{}, err
	}

	if len(clusters) > 1 {
		return ctrl.Result{}, errors.New("multiple clusters found")
	}

	for _, cluster := range clusters {
		logger.Info("reconciling cluster", "name", cluster.Name, "ID", cluster.ID)

		devices, err := r.netBox.DCIM().GetDevicesByClusterID(cluster.ID)
		if err != nil {
			logger.Error(err, "unable to find devices for cluster", "name", cluster.Name, "ID", cluster.ID)
			return ctrl.Result{}, err
		}

		for _, device := range devices {
			err = r.reconcileDevice(ctx, capiCluster, &device)
			if err != nil {
				logger.Error(err, "unable to reconcile device", "device", device.Name, "ID", device.ID)
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{RequeueAfter: r.reconcileInterval}, nil
}

func (r *Metal3Reconciler) reconcileDevice(ctx context.Context, cluster *clusterv1.Cluster, device *models.Device) error {
	logger := log.FromContext(ctx)
	logger.Info("reconciling device", "device", device.Name, "ID", device.ID)

	if device.Status.Value != "active" {
		logger.Info("device is not active", "status", device.Status.Value)
		return nil
	}

	bmh := &bmov1alpha1.BareMetalHost{}
	if err := r.k8sClient.Get(ctx, client.ObjectKey{Name: device.Name, Namespace: cluster.Namespace}, bmh); err == nil {
		logger.Info("BareMetalHost custom resource already exists, will skip", "host", bmh.Name)
		return nil
	}

	redfishURL, err := createRedFishURL(device)
	if err != nil {
		return errors.New("unable to create redfish url")
	}

	deviceNameParts := strings.Split(device.Name, "-")
	if len(deviceNameParts) != 2 {
		return fmt.Errorf("unable to split in two device name: %s", device.Name)
	}

	rootHint, err := createRootHint(device)
	if err != nil {
		return fmt.Errorf("unable to create root hint: %w", err)
	}

	mac, err := getMacForIP(r.netBox, device.PrimaryIP4.Address)
	if err != nil {
		logger.Info("unable to lookup mac for ip", "error", err)
		mac = ""
	}

	region, err := r.netBox.DCIM().GetRegionForDevice(device)
	if err != nil {
		return fmt.Errorf("unable to get region for device: %w", err)
	}

	bmcSecret, err := r.createBmcSecret(cluster, device)
	if err != nil {
		return fmt.Errorf("unable to create bmc secret: %w", err)
	}

	if err = r.k8sClient.Create(ctx, bmcSecret); err != nil {
		return fmt.Errorf("unable to upload bmc secret: %w", err)
	}

	logger.Info("created BMC Secret", "name", bmcSecret.Name)

	role, err := getRoleFromTags(device)
	if err != nil {
		return fmt.Errorf("unable to get role from tags: %w", err)
	}

	if role == device.DeviceRole.Slug {
		logger.Info("no role found in tags, using device role")
	} else {
		logger.Info("role found in tags", "role", role)
	}

	ndSecretName := "networkdata-" + device.Name
	bareMetalHost := &bmov1alpha1.BareMetalHost{
		ObjectMeta: ctrl.ObjectMeta{
			Name:      device.Name,
			Namespace: cluster.Namespace,
			Labels: map[string]string{
				"kubernetes.metal.cloud.sap/cluster": cluster.Name,
				"kubernetes.metal.cloud.sap/name":    device.Name,
				"kubernetes.metal.cloud.sap/bb":      deviceNameParts[1],
				"kubernetes.metal.cloud.sap/role":    role,
				"topology.kubernetes.io/region":      region,
				"topology.kubernetes.io/zone":        device.Site.Slug,
			},
		},

		Spec: bmov1alpha1.BareMetalHostSpec{
			Architecture:          "x86_64",
			AutomatedCleaningMode: "disabled",
			Online:                true,
			BMC: bmov1alpha1.BMCDetails{
				Address:                        redfishURL,
				CredentialsName:                bmcSecret.Name,
				DisableCertificateVerification: true,
			},
			BootMACAddress: mac,
			NetworkData: &corev1.SecretReference{
				Name:      ndSecretName,
				Namespace: cluster.Namespace,
			},
			RootDeviceHints: rootHint,
		},
	}

	if err = r.k8sClient.Create(ctx, bareMetalHost); err != nil {
		return fmt.Errorf("unable to create baremetal host: %w", err)
	}

	logger.Info("created BareMetalHost CR", "name", bareMetalHost.Name)

	if err = r.createNetworkDataSecret(ctx, bareMetalHost, cluster, device, role, ndSecretName); err != nil {
		return fmt.Errorf("unable to create network data: %w", err)
	}

	logger.Info("created NetworkData Secret", "name", ndSecretName)

	return nil
}

// CreateNetworkDataForDevice uses the device to get to the netbox interfaces and creates a secret containing the network data for this device
func (r *Metal3Reconciler) createNetworkDataSecret(ctx context.Context, bareMetalHost *bmov1alpha1.BareMetalHost, cluster *clusterv1.Cluster, device *models.Device, role, secretName string) error {
	iface, err := r.netBox.DCIM().GetInterfaceForDevice(device, "LAG1")
	if err != nil {
		return fmt.Errorf("unable to find interface LAG1 for device %s: %w", device.Name, err)
	}

	ip, err := r.netBox.IPAM().GetIPAddressForInterface(iface.ID)
	if err != nil {
		return fmt.Errorf("unable to get IP for interface ID %d: %w", iface.ID, err)
	}

	prefixes, err := r.netBox.IPAM().GetPrefixesContaining(ip.Address)
	if err != nil {
		return fmt.Errorf("unable to get prefixes containing IP %s: %w", ip.Address, err)
	}

	vlanID := 0
	for _, prefix := range prefixes {
		if prefix.Vlan.VID != 0 {
			vlanID = prefix.Vlan.VID
			break
		}
	}

	netw, err := netaddr.ParseIPv4Net(ip.Address)
	if err != nil {
		return fmt.Errorf("unable to parse IP address %s: %w", ip.Address, err)
	}

	linkHint, err := createLinkHint(device, role)
	if err != nil {
		return fmt.Errorf("unable to create link hint for device %s and role %s: %w", device.Name, role, err)
	}

	netMask := netw.Netmask().Extended()
	nwData := networkdata.NetworkData{
		Networks: []networkdata.L3{
			{
				ID:        vlanID,
				Type:      networkdata.Ipv4,
				IPAddress: &ip.Address,
				Link:      linkHint,
				Netmask:   &netMask,
				NetworkID: "",
				Routes: []networkdata.L3IPVRoutingConfigurationItem{
					{
						Gateway: netw.Nth(1).String(),
						Netmask: "0.0.0.0",
						Network: "0.0.0.0",
					},
				},
			},
		},
	}

	nwDataYaml, err := yaml.Marshal(nwData)
	if err != nil {
		return fmt.Errorf("unable to marshal network data: %w", err)
	}

	nwDataSecret := &corev1.Secret{
		ObjectMeta: ctrl.ObjectMeta{
			Name:      secretName,
			Namespace: cluster.Namespace,
		},
		Type: "",
		StringData: map[string]string{
			"networkData": string(nwDataYaml),
		},
	}

	if err := ctrl.SetControllerReference(bareMetalHost, nwDataSecret, r.scheme); err != nil {
		return fmt.Errorf("failed to set owner reference on networkdata secret: %w", err)
	}

	return r.k8sClient.Create(ctx, nwDataSecret)
}

func (r *Metal3Reconciler) createBmcSecret(cluster *clusterv1.Cluster, device *models.Device) (*corev1.Secret, error) {
	user := r.cfg.BMCUser
	password := r.cfg.BMCPassword

	if user == "" || password == "" {
		return nil, errors.New("bmc user or password not set")
	}

	return &corev1.Secret{
		ObjectMeta: ctrl.ObjectMeta{
			Name:      "bmc-secret-" + device.Name,
			Namespace: cluster.Namespace,
		},
		StringData: map[string]string{
			"username": user,
			"password": password,
		},
	}, nil
}

func createRedFishURL(device *models.Device) (string, error) {
	ip, _, err := net.ParseCIDR(device.OOBIp.Address)
	if err != nil {
		return "", err
	}
	switch device.DeviceType.Manufacturer.Slug {
	case "dell":
		return "idrac-redfish://" + ip.String() + "/redfish/v1/Systems/System.Embedded.1", nil
	default:
		return "redfish://" + ip.String() + "/redfish/v1/Systems/1", nil
	}
}

func createRootHint(device *models.Device) (*bmov1alpha1.RootDeviceHints, error) {
	if hint, ok := rootHintMap[device.DeviceType.Slug]; ok {
		return &bmov1alpha1.RootDeviceHints{
			Model: hint,
		}, nil
	}
	return nil, fmt.Errorf("unknown device model for root hint: %s", device.DeviceType.Model)
}

func createLinkHint(device *models.Device, role string) (string, error) {
	if role == "kvm" {
		if hint, ok := linkHintMapKvm[device.DeviceType.Model]; ok {
			return hint, nil
		}
	} else {
		if hint, ok := linkHintMapCeph[device.DeviceType.Model]; ok {
			return hint, nil
		}
	}

	return "", fmt.Errorf("unknown device model for link hint: %s", device.DeviceType.Model)
}

func getRoleFromTags(device *models.Device) (string, error) {
	tagsCount := 0
	deviceRole := device.DeviceRole.Slug

	for _, tag := range device.Tags {
		switch tag.Name {
		case "ceph-HDD":
			deviceRole = "ceph-osd"
			tagsCount++
		case "ceph-NVME":
			deviceRole = "ceph-osd"
			tagsCount++
		case "ceph-mon":
			deviceRole = "ceph-mon"
			tagsCount++
		case "KVM":
			deviceRole = "kvm"
			tagsCount++
		}
	}

	if tagsCount > 1 {
		return "", errors.New("device has multiple tags")
	}

	return deviceRole, nil
}

func getMacForIP(netBox netbox.Netbox, ipAddress string) (string, error) {
	ip, err := netBox.IPAM().GetIPAddressByAddress(ipAddress)
	if err != nil {
		return "", err
	}

	assignedIface, err := netBox.DCIM().GetInterfaceByID(ip.AssignedInterface.ID)
	if err != nil {
		return "", err
	}

	lagIfaces, err := netBox.DCIM().GetInterfacesByLagID(assignedIface.ID)
	if err != nil {
		return "", err
	}

	if len(lagIfaces) == 0 {
		return "", errors.New("no LAG interfaces found")
	}

	macs := make(map[string]string)
	names := []string{}
	for _, iface := range lagIfaces {
		macs[iface.Name] = iface.MacAddress
		names = append(names, iface.Name)
	}

	sort.Strings(names)
	return macs[names[0]], nil
}
