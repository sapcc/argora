/*
 * Copyright (c) 2024. Lorem ipsum dolor sit amet, consectetur adipiscing elit.
 * Morbi non lorem porttitor neque feugiat blandit. Ut vitae ipsum eget quam lacinia accumsan.
 * Etiam sed turpis ac ipsum condimentum fringilla. Maecenas magna.
 * Proin dapibus sapien vel ante. Aliquam erat volutpat. Pellentesque sagittis ligula eget metus.
 * Vestibulum commodo. Ut rhoncus gravida arcu.
 */

package controller

import (
	"context"
	"fmt"
	"github.com/dspinhirne/netaddr-go"
	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	"github.com/sapcc/argora/internal/netbox"
	"github.com/sapcc/argora/internal/networkdata"
	"github.com/sapcc/go-netbox-go/models"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"net"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"strconv"
	"strings"
)

const ClusterRoleLabel = "discovery.inf.sap.cloud/clusterRole"

type ClusterController struct {
	client.Client
	Nb *netbox.NetboxClient
}

// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metal3.io,resources=baremetalhosts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile looks up a cluster in netbox and creates baremetal hosts for it
func (c *ClusterController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("reconciling cluster")
	cluster := &clusterv1.Cluster{}
	err := c.Client.Get(ctx, req.NamespacedName, cluster)
	if client.IgnoreNotFound(err) != nil {
		logger.Error(err, "unable to get cluster")
		return ctrl.Result{}, err
	}
	role := cluster.Labels[ClusterRoleLabel]
	devices, err := c.Nb.LookupCluster(role, cluster.Name)
	if err != nil {
		logger.Error(err, "unable to lookup cluster in netbox")
		return ctrl.Result{}, err
	}
	for _, device := range devices {
		err = c.ReconcileDevice(ctx, *cluster, &device)
		if err != nil {
			logger.Error(err, "unable to reconcile device")
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

// CreateNetworkDataForDevice uses the device to get to the netbox interfaces and creates a secret containing the network data for this device
func (c *ClusterController) CreateNetworkDataForDevice(ctx context.Context, cluster clusterv1.Cluster, device *models.Device) error {
	vlan, ipStr, err := c.Nb.LookupVLANForDevice(device)
	if err != nil {
		log.FromContext(ctx).Error(err, "unable to lookup vlan for device")
		return err
	}
	netw, err := netaddr.ParseIPv4Net(ipStr)
	if err != nil {
		log.FromContext(ctx).Error(err, "unable to parse network")
		return err
	}
	netMask := netw.Netmask().Extended()
	linkHint, err := createLinkHint(device)
	if err != nil {
		log.FromContext(ctx).Error(err, "unable to create link hint")
		return err
	}
	nwdRaw := networkdata.NetworkData{
		Networks: []networkdata.L3{
			{
				ID:        strconv.Itoa(vlan),
				Type:      networkdata.Ipv4,
				IPAddress: &ipStr,
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
	ndwYaml, err := yaml.Marshal(nwdRaw)
	if err != nil {
		log.FromContext(ctx).Error(err, "unable to marshal network data")
		return err
	}
	result := &corev1.Secret{
		ObjectMeta: ctrl.ObjectMeta{
			Name:      "networkdata-" + device.Name,
			Namespace: cluster.Namespace,
		},
		Type: "",
		StringData: map[string]string{
			"networkData": string(ndwYaml),
		},
	}
	return c.Create(ctx, result)
}

func (c *ClusterController) ReconcileDevice(ctx context.Context, cluster clusterv1.Cluster, device *models.Device) error {
	logger := log.FromContext(ctx)
	logger.Info("reconciling device", "node", device.Name)
	// check if device is active
	if device.Status.Value != "active" {
		logger.Info("device is not active")
		return nil
	}
	// check if the host already exists
	bmh := &bmov1alpha1.BareMetalHost{}
	err := c.Client.Get(ctx, client.ObjectKey{Name: device.Name, Namespace: cluster.Namespace}, bmh)
	if err == nil {
		logger.Info("host already exists")
		return nil
	}
	redfishUrl, err := createRedFishUrl(device)
	if err != nil {
		logger.Error(err, "unable to create redfish url")
		return err
	}
	// ugly hack to get the role this is not easy to get to in netbox
	nameParts := strings.Split(device.Name, "-")
	if len(nameParts) != 2 {
		err = fmt.Errorf("invalid device name: %s", device.Name)
		logger.Error(err, "error splitting name")
		return err
	}
	// create the root device hint
	rootHint, err := createRootHint(device)
	if err != nil {
		logger.Error(err, "unable to create root hint")
		return err
	}
	// create the host
	diskFormat := "qcow2"
	mac, err := c.Nb.LookupMacForIp(device.PrimaryIp4.Address)
	if err != nil {
		logger.Error(err, "unable to lookup mac for ip")
		return err
	}
	host := &bmov1alpha1.BareMetalHost{
		ObjectMeta: ctrl.ObjectMeta{
			Name:      device.Name,
			Namespace: cluster.Namespace,
			Labels: map[string]string{
				"kubernetes.metal.cloud.sap/cluster": cluster.Name,
				"kubernetes.metal.cloud.sap/name":    device.Name,
				"kubernetes.metal.cloud.sap/bb":      nameParts[1],
				"kubernetes.metal.cloud.sap/role":    device.DeviceRole.Slug,
				"topology.kubernetes.io/region":      "",
				"topology.kubernetes.io/zone":        device.Site.Slug,
			},
		},

		Spec: bmov1alpha1.BareMetalHostSpec{
			Architecture:          "x86_64",
			AutomatedCleaningMode: "disabled",
			Online:                true,
			Image: &bmov1alpha1.Image{
				URL:        "https://repo.qa-de-1.cloud.sap/flatcar/stable/current/flatcar_production_openstack_image.img",
				Checksum:   "https://repo.qa-de-1.cloud.sap/flatcar/stable/current/flatcar_production_openstack_image.img.DIGESTS",
				DiskFormat: &diskFormat,
			},
			BMC: bmov1alpha1.BMCDetails{
				Address:                        redfishUrl,
				CredentialsName:                "bmc-secret-" + device.Name,
				DisableCertificateVerification: true,
			},
			BootMACAddress: mac,
			NetworkData: &corev1.SecretReference{
				Name:      "networkdata-" + device.Name,
				Namespace: cluster.Namespace,
			},
			RootDeviceHints: rootHint,
		},
	}
	err = c.Client.Create(ctx, host)
	if err != nil {
		logger.Error(err, "unable to create baremetal host")
		return err
	}

	err = c.CreateNetworkDataForDevice(ctx, cluster, device)
	if err != nil {
		logger.Error(err, "unable to create network data")
		return err
	}
	return nil
}

func (c *ClusterController) AddToManager(mgr manager.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).For(&clusterv1.Cluster{}).Complete(c)
}

func createRedFishUrl(device *models.Device) (string, error) {
	ip, _, err := net.ParseCIDR(device.PrimaryIp4.Address)
	if err != nil {
		return "", err
	}
	return "idrac-redfish://" + ip.String() + "/redfish/v1/Systems/System.Embedded.1", nil
}

func createRootHint(device *models.Device) (*bmov1alpha1.RootDeviceHints, error) {
	switch device.DeviceType.Model {
	case "PowerEdge R640":
		return &bmov1alpha1.RootDeviceHints{
			Model: "DELLBOSS VD",
		}, nil
	case "ThinkSystem SR650":
		return &bmov1alpha1.RootDeviceHints{
			Model: "ThinkSystem M.2 VD",
		}, nil
	default:
		return nil, fmt.Errorf("unknown device model for root hint: %s", device.DeviceType.Model)
	}
}

func createLinkHint(device *models.Device) (string, error) {
	switch device.DeviceType.Model {
	case "PowerEdge R640":

		return "ens*f1*", nil
	default:
		return "", fmt.Errorf("unknown device model for link hint: %s", device.DeviceType.Model)
	}
}
