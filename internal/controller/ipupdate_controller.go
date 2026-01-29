// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	metalv1alpha1 "github.com/ironcore-dev/metal-operator/api/v1alpha1"
	"github.com/sapcc/argora/internal/credentials"
	"github.com/sapcc/argora/internal/netbox"
	"github.com/sapcc/go-netbox-go/models"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
	ipamv1 "sigs.k8s.io/cluster-api/api/ipam/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

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
	logger.Info("reconciling update", "namespace_name", req.NamespacedName, "namespace", req.Name)

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

	deviceName, err := r.findDeviceName(ctx, ipAddress)
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

	err = r.reconcileNetboxIP(ctx, targetInterface, ipAddress)
	if err != nil {
		logger.Error(err, "netbox ip reconciliation")
		return ctrl.Result{}, err
	}

	logger.Info("reconcile completed successfully")

	return ctrl.Result{}, nil
}

func (r *IPUpdateReconciler) reconcileNetboxIP(ctx context.Context, iface models.Interface, ipAddr *ipamv1.IPAddress) error {
	neededAddress := ipAddr.Spec.Address
	netboxAddresses, err := r.netBox.IPAM().GetIPAddressesForInterface(iface.ID)
	if err != nil {
		return err
	}

	switch len(netboxAddresses) {
	case 0: // no addresses -> create
		wIpAddr := models.WriteableIPAddress{
			NestedIPAddress: models.NestedIPAddress{
				ID:      0,
				URL:     "",
				Family:  nil,
				Address: neededAddress,
			},
			Vrf:                0,
			Tenant:             0,
			Status:             "Active",
			AssignedObjectType: "dcim.interface",
			AssignedObjectID:   iface.ID,
			NatInside:          0,
			NatOutside:         0,
			DNSName:            "",
			Description:        "",
			Tags:               []models.NestedTag{},
			CustomFields:       nil,
			Created:            "",
			LastUpdated:        "",
		}
		_, err = r.netBox.IPAM().CreateIPAddress(wIpAddr)
		if err != nil {
			return err
		}
	case 1: // one ip -> either it's correct or update it
		currAddr := netboxAddresses[0]
		if currAddr.Address == neededAddress {
			return nil
		}

		wIPAddr := models.WriteableIPAddress{
			NestedIPAddress: models.NestedIPAddress{
				ID:      currAddr.ID,
				URL:     currAddr.URL,
				Family:  currAddr.Family,
				Address: neededAddress,
			},
		}

		_, err = r.netBox.IPAM().UpdateIPAddress(wIPAddr)
		if err != nil {
			return err
		}
	default:
		// edge case more then one ip, we cannot clarify which to update
		// but if one address is equal to needed, then we can skip this
		for _, addr := range netboxAddresses {
			if addr.Address == neededAddress {
				return nil
			}
		}

		return fmt.Errorf("for interface %q exists more then 1 address, cannot clarify what to do", iface.Name)
	}

	return nil
}

func (r *IPUpdateReconciler) findDeviceName(ctx context.Context, ipAddr *ipamv1.IPAddress) (string, error) {
	claimName := getOwnerByKind(ipAddr.OwnerReferences, "IPAddressClaim")
	if claimName == "" {
		return "", fmt.Errorf("no IPAddressClaim owner found for IPAddress %s", ipAddr.Name)
	}

	ipClaim := &ipamv1.IPAddressClaim{}
	if err := r.k8sClient.Get(ctx, client.ObjectKey{Namespace: ipAddr.Namespace, Name: claimName}, ipClaim); err != nil {
		return "", fmt.Errorf("get IpAddressClaim: %w", err)
	}

	serverClaimName := getOwnerByKind(ipClaim.OwnerReferences, "ServerClaim")
	if serverClaimName == "" {
		return "", fmt.Errorf("no ServerClaim owner found for IpAddressClaim %s", ipClaim.Name)
	}

	serverClaim := &metalv1alpha1.ServerClaim{}
	if err := r.k8sClient.Get(ctx, client.ObjectKey{Namespace: ipClaim.Namespace, Name: serverClaimName}, serverClaim); err != nil {
		return "", fmt.Errorf("get ServerClaim: %w", err)
	}

	if serverClaim.Spec.ServerRef == nil {
		return "", fmt.Errorf("ServerClaim %s not yet bound to a server", serverClaimName)
	}

	server := &metalv1alpha1.Server{}
	if err := r.k8sClient.Get(ctx, client.ObjectKey{Namespace: serverClaim.Namespace, Name: serverClaim.Spec.ServerRef.Name}, server); err != nil {
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
