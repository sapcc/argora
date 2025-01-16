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
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime"

	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/dspinhirne/netaddr-go/v2"
	"github.com/sapcc/go-netbox-go/models"
	"gopkg.in/yaml.v3"

	"github.com/sapcc/argora/internal/netbox"
	"github.com/sapcc/argora/internal/networkdata"
)

const ClusterRoleLabel = "discovery.inf.sap.cloud/clusterRole"

type ClusterController struct {
	client.Client
	Nb          *netbox.NetboxClient
	Scheme      *runtime.Scheme
	BMCUser     string
	BMCPassword string
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
	return ctrl.Result{RequeueAfter: 300 * time.Second}, nil
	// return ctrl.Result{}, nil
}

// CreateNetworkDataForDevice uses the device to get to the netbox interfaces and creates a secret containing the network data for this device
func (c *ClusterController) CreateNetworkDataForDevice(ctx context.Context, host *bmov1alpha1.BareMetalHost, cluster clusterv1.Cluster, device *models.Device, role string) error {
	vlan, ipStr, err := c.Nb.LookupVLANForDevice(device, role)
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
	linkHint, err := createLinkHint(device, role)
	if err != nil {
		log.FromContext(ctx).Error(err, "unable to create link hint")
		return err
	}
	nwdRaw := networkdata.NetworkData{
		Networks: []networkdata.L3{
			{
				ID:        vlan,
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
	if err := ctrl.SetControllerReference(host, result, c.Scheme); err != nil {
		log.FromContext(ctx).Error(err, "failed to set owner reference on networkdata secret")
		return err
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
		logger.Info("host already exists", "host", bmh.Name)
		return nil
	}
	redfishURL, err := createRedFishURL(device)
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
	mac, err := c.Nb.LookupMacForIP(device.PrimaryIp4.Address)
	if err != nil {
		logger.Info("unable to lookup mac for ip", err)
		mac = ""
	}
	// get the region
	region, err := c.Nb.GetRegionForDevice(device)
	if err != nil {
		logger.Error(err, "unable to lookup region for device")
		return err
	}
	bmcSecret, err := c.createBmcSecret(cluster, device)
	if err != nil {
		logger.Error(err, "unable to create bmc secret")
		return err
	}
	err = c.Client.Create(ctx, bmcSecret)
	if err != nil {
		logger.Error(err, "unable to upload bmc secret")
		return err
	}
	labelRole := getRoleFromTags(device)
	if labelRole == device.DeviceRole.Slug {
		logger.Info("no role found in tags, using device role")
	} else {
		logger.Info("role found in tags", "role", labelRole)
	}

	host := &bmov1alpha1.BareMetalHost{
		ObjectMeta: ctrl.ObjectMeta{
			Name:      device.Name,
			Namespace: cluster.Namespace,
			Labels: map[string]string{
				"kubernetes.metal.cloud.sap/cluster": cluster.Name,
				"kubernetes.metal.cloud.sap/name":    device.Name,
				"kubernetes.metal.cloud.sap/bb":      nameParts[1],
				"kubernetes.metal.cloud.sap/role":    labelRole,
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

	err = c.CreateNetworkDataForDevice(ctx, host, cluster, device, labelRole)
	if err != nil {
		logger.Error(err, "unable to create network data")
		return err
	}
	return nil
}

func (c *ClusterController) AddToManager(mgr manager.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).For(&clusterv1.Cluster{}).Complete(c)
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

func (c *ClusterController) createBmcSecret(cluster clusterv1.Cluster, device *models.Device) (*corev1.Secret, error) {
	user := c.BMCUser
	password := c.BMCPassword
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

var rootHintMap = map[string]string{
	"poweredge-r660":       "BOSS",
	"poweredge-r640":       "BOSS",
	"poweredge-r840":       "BOSS",
	"poweredge-r7615":      "BOSS",
	"thinksystem-sr650":    "ThinkSystem M.2 VD",
	"sr655-v3":             "NVMe 2-Bay",
	"thinksystem-sr650-v3": "NVMe 2-Bay",
	"proliant-dl320-gen11": "HPE NS204i-u Gen11 Boot Controller",
}

var linkHintMapCeph = map[string]string{
	"ThinkSystem SR650":    "en*f0np*",
	"ThinkSystem SR650 v3": "en*1f*np*",
	"Thinksystem SR655 v3": "en*f*np*",
	"PowerEdge R640":       "en*f1np*",
	"PowerEdge R660":       "en*f1np*",
	"PowerEdge R7615":      "en*f*np*",
	"Proliant DL320 Gen11": "en*f1np*",
}

var linkHintMapKvm = map[string]string{
	"PowerEdge R640": "en*f0np*",
	"PowerEdge R840": "en*f0np*",
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
	var linkHintMap map[string]string
	if role == "kvm" {
		linkHintMap = linkHintMapKvm
	} else {
		linkHintMap = linkHintMapCeph
	}
	if hint, ok := linkHintMap[device.DeviceType.Model]; ok {
		return hint, nil
	}
	return "", fmt.Errorf("unknown device model for link hint: %s", device.DeviceType.Model)
}

func getRoleFromTags(device *models.Device) string {
	nTags := 0
	dRole := ""
	for _, tag := range device.Tags {
		switch tag.Name {
		case "ceph-HDD":
			dRole = "ceph-osd"
			nTags++
		case "ceph-NVME":
			dRole = "ceph-osd"
			nTags++
		case "ceph-mon":
			dRole = "ceph-mon"
			nTags++
		case "KVM":
			dRole = "kvm"
			nTags++
		}
	}
	if nTags != 1 {
		return device.DeviceRole.Slug
	}
	return dRole
}
