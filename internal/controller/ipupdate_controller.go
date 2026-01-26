// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
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

// +kubebuilder:rbac:groups=ipam.cluster.x-k8s.io,resources=ipaddresses;ipaddressclaims,verbs=list;watch;update;patch
// +kubebuilder:rbac:groups=metal.ironcore.dev,resources=serverclaims;servers,verbs=list;watch;update;patch

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

	logger.Info("found interfaces", "interfaces", interfaces)

	targetInterface, err := r.findTargetInterface(interfaces)
	if err != nil {
		logger.Error(err, "unable to find target interface")
		return ctrl.Result{}, err
	}

	logger = logger.WithValues("interface", targetInterface.Name)
	logger.Info("target interface found")

	// ... perform update operations with r.netBox and ipAddress ...

	logger.Info("reconcile completed successfully")

	return ctrl.Result{}, nil
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
	found := false

	for _, iface := range interfaces {
		name := strings.ToUpper(iface.Name)

		if strings.HasPrefix(name, "LAG") {
			if !found || iface.Name > target.Name { // "LAG1" > "LAG0" returns true
				target = iface
				found = true
			}
		}
	}

	if !found {
		return models.Interface{}, fmt.Errorf("no LAG interface found for device")
	}

	return target, nil
}
