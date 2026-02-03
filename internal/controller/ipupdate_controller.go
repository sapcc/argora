// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"regexp"
	"strings"

	"github.com/go-logr/logr"
	metalv1alpha1 "github.com/ironcore-dev/metal-operator/api/v1alpha1"
	"github.com/sapcc/argora/internal/credentials"
	"github.com/sapcc/argora/internal/netbox"
	"github.com/sapcc/argora/internal/netbox/ipam"
	"github.com/sapcc/go-netbox-go/models"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ipamv1 "sigs.k8s.io/cluster-api/api/ipam/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var ErrIPAssignedAsAnotherDevicePrimaryAddress = errors.New("ip is already assigned as primary to another device")
var ErrIPAssignToAnotherDevice = errors.New("ip assigned to another device")
var ErrIPAssignedToAnotherInteface = errors.New("ip assigned to another interface")

// IPUpdateReconciler reconciles a ipam.cluster.x-k8s.io.IPAddress object
type IPUpdateReconciler struct {
	k8sClient   client.Client
	scheme      *runtime.Scheme
	credentials *credentials.Credentials
	netBox      netbox.Netbox
}

func NewIPUpdateReconciler(mgr ctrl.Manager, creds *credentials.Credentials, netBox netbox.Netbox) *IPUpdateReconciler {
	return &IPUpdateReconciler{
		k8sClient:   mgr.GetClient(),
		scheme:      mgr.GetScheme(),
		credentials: creds,
		netBox:      netBox,
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *IPUpdateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Watches(&ipamv1.IPAddress{}, &handler.EnqueueRequestForObject{}).
		Named("ipupdate").
		Complete(r)
}

// +kubebuilder:rbac:groups=ipam.cluster.x-k8s.io,resources=ipaddresses,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=ipam.cluster.x-k8s.io,resources=ipaddressclaims,verbs=get;list;watch
// +kubebuilder:rbac:groups=metal.ironcore.dev,resources=serverclaims;servers,verbs=list;get;watch

func (r *IPUpdateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("reconciling update")

	ipAddress := &ipamv1.IPAddress{}
	err := r.k8sClient.Get(ctx, req.NamespacedName, ipAddress)
	if err != nil {
		logger.Error(err, "unable to get IP Address CR")
		return ctrl.Result{}, err
	}

	err = r.credentials.Reload()
	if err != nil {
		logger.Error(err, "unable to reload credentials")
		return ctrl.Result{}, err
	}

	logger.Info("credentials reloaded", "credentials", r.credentials)

	err = r.netBox.Reload(r.credentials.NetboxToken, logger)
	if err != nil {
		logger.Error(err, "unable to reload netbox")
		return ctrl.Result{}, err
	}

	deviceName, err := r.findDeviceName(ctx, req.Namespace, ipAddress)
	if err != nil {
		logger.Error(err, "unable to find device name")
		return ctrl.Result{}, err
	}

	logger = logger.WithValues("deviceName", deviceName, "deviceIP", ipAddress.Spec.Address)
	logger.Info("device found")

	device, err := r.netBox.DCIM().GetDeviceByName(deviceName)
	if err != nil {
		logger.Error(err, "unable to find device by name")
		return ctrl.Result{}, err
	}

	interfaces, err := r.netBox.DCIM().GetInterfacesForDevice(device)
	if err != nil {
		logger.Error(err, "unable to find interfaces by name")
		return ctrl.Result{}, err
	}

	targetInterface, err := r.findTargetInterface(interfaces)
	if err != nil {
		logger.Error(err, "unable to find target interface")
		return ctrl.Result{}, err
	}

	logger = logger.WithValues("interface", targetInterface.Name)
	logger.Info("target interface found")

	err = r.reconcileNetbox(targetInterface, device, ipAddress, logger)
	if err != nil {
		logger.Error(err, "netbox ip reconciliation")
		return ctrl.Result{}, err
	}

	logger.Info("reconcile completed successfully")

	return ctrl.Result{}, nil
}

func (r *IPUpdateReconciler) reconcileNetbox(
	iface models.Interface,
	device *models.Device,
	ipAddr *ipamv1.IPAddress,
	logger logr.Logger,
) error {
	addr, err := r.reconcileNetboxAddressIP(iface, ipAddr, device, logger)
	if err != nil {
		return err
	}

	err = r.reconcileDevicePrimaryIP(addr, device, logger)
	if err != nil {
		return err
	}

	return nil
}

func getPrefix(addres *ipamv1.IPAddress) (netip.Prefix, error) {
	addr, err := netip.ParseAddr(addres.Spec.Address)
	if err != nil {
		return netip.Prefix{}, err
	}

	cidr := 32
	if addres.Spec.Prefix != nil {
		cidr = int(*addres.Spec.Prefix)
	}

	return netip.PrefixFrom(addr, cidr), nil
}

func (r *IPUpdateReconciler) reconcileNetboxAddressIP(
	iface models.Interface,
	ipAddr *ipamv1.IPAddress,
	newDevice *models.Device,
	logger logr.Logger,
) (*models.IPAddress, error) {
	prefix, err := getPrefix(ipAddr)
	if err != nil {
		return nil, err
	}
	logger = logger.WithValues("ip", prefix.String())

	addr, err := r.netBox.IPAM().GetIPAddressByAddress(prefix.String())
	if err != nil {
		logger.Info("no ip address found, creating ip")
		addr, err = r.createIPAddress(prefix, iface.ID, newDevice.Tenant.ID, logger)
		if err != nil {
			return nil, fmt.Errorf("create ip adddres: %w", err)
		}
		logger.Info("ip address created", "ip", prefix)

		return addr, nil
	}

	if addr.AssignedInterface.ID != iface.ID {
		return addr, fmt.Errorf("%w, old interface %d, addr %d",
			ErrIPAssignedToAnotherInteface, addr.AssignedInterface.ID, addr.ID)
	}

	logger.Info("ip is assigned to needed interface")

	oldDeviceID := addr.AssignedInterface.Device.ID
	if newDevice.ID != oldDeviceID {
		err := fmt.Errorf("%w old device %d, newDevice %d, addr %d",
			ErrIPAssignToAnotherDevice, addr.AssignedInterface.Device.ID, newDevice.ID, addr.ID)
		return nil, err
	}

	logger.Info("ip is assigned to needed device")

	return addr, nil
}

func (r *IPUpdateReconciler) createIPAddress(prefix netip.Prefix, ifaceID, tenantID int, logger logr.Logger) (*models.IPAddress, error) {
	netboxPrefixes, err := r.netBox.IPAM().GetPrefixesByPrefix(prefix.Masked().String())
	if err != nil {
		return nil, err
	}

	var vrfID int
	if len(netboxPrefixes) == 1 {
		vrfID = netboxPrefixes[0].Vrf.ID
	} else {
		logger.Info("cannot determine right prefix, fallback to default VRF",
			"prefix", prefix.Masked().String(), "found_amount", len(netboxPrefixes))
	}

	ipParams := ipam.CreateIPAddressParams{
		Address:     prefix.String(),
		TenantID:    tenantID,
		InterfaceID: ifaceID,
		VrfID:       vrfID,
	}

	address, err := r.netBox.IPAM().CreateIPAddress(ipParams)
	if err != nil {
		return nil, err
	}

	return address, nil
}

func (r *IPUpdateReconciler) reconcileDevicePrimaryIP(
	addr *models.IPAddress,
	device *models.Device,
	logger logr.Logger,
) error {
	if device.PrimaryIP.ID == addr.ID {
		logger.Info("primary device id is same", "ipaddres_id", addr.ID)
		return nil
	}

	wDevice := device.Writeable()
	wDevice.PrimaryIP4 = addr.ID

	_, err := r.netBox.DCIM().UpdateDevice(wDevice)
	if err != nil {
		return err
	}

	logger.Info("primary device id updated", "device_id", device.ID, "address_id", addr.ID)

	return nil
}

func (r *IPUpdateReconciler) findDeviceName(
	ctx context.Context,
	namespace string,
	ipAddr *ipamv1.IPAddress,
) (string, error) {
	claimName := ipAddr.Spec.ClaimRef.Name

	ipClaim := &ipamv1.IPAddressClaim{}
	if err := r.k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: claimName}, ipClaim); err != nil {
		return "", fmt.Errorf("failed to get IPAddressClaim for IPAddress: %w", err)
	}

	serverClaimName := getOwnerByKind(ipClaim.OwnerReferences, "ServerClaim")
	if serverClaimName == "" {
		return "", fmt.Errorf("no ServerClaim owner found for IpAddressClaim %s", ipClaim.Name)
	}

	serverClaim := &metalv1alpha1.ServerClaim{}
	err := r.k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: serverClaimName}, serverClaim)
	if err != nil {
		return "", fmt.Errorf("get ServerClaim: %w", err)
	}

	if serverClaim.Spec.ServerRef == nil {
		return "", fmt.Errorf("ServerClaim %s not yet bound to a server", serverClaimName)
	}

	server := &metalv1alpha1.Server{}
	err = r.k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: serverClaim.Spec.ServerRef.Name}, server)
	if err != nil {
		return "", fmt.Errorf("get Server: %w", err)
	}

	if server.Spec.BMCRef.Name == "" {
		return "", fmt.Errorf("server %s has no bmcRef name", server.Name)
	}

	return server.Spec.BMCRef.Name, nil
}

func getOwnerByKind(owners []metav1.OwnerReference, kind string) string {
	for _, o := range owners {
		if o.Kind == kind {
			return o.Name
		}
	}

	return ""
}

func (r *IPUpdateReconciler) findTargetInterface(interfaces []models.Interface) (models.Interface, error) {
	var target models.Interface
	maxLag := ""

	re := regexp.MustCompile(`^LAG(\d)$`)

	for _, iface := range interfaces {
		if strings.ToLower(iface.Type.Value) != "lag" {
			continue
		}

		m := re.FindStringSubmatch(iface.Name)
		if len(m) != 2 {
			continue
		}

		lag := m[1]

		if lag > maxLag {
			maxLag = lag
			target = iface
		}
	}

	if maxLag == "" {
		return models.Interface{}, errors.New("no LAG interface found for device")
	}

	return target, nil
}
