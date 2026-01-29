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
	"github.com/sapcc/go-netbox-go/models"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ipamv1 "sigs.k8s.io/cluster-api/api/ipam/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/sapcc/argora/internal/credentials"
	"github.com/sapcc/argora/internal/netbox"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const ipAddressFinalizer = "ipupdate.argora.cloud.sap.com/finalizer"

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
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "unable to get IP Address CR")
		return ctrl.Result{}, err
	}

	logger = logger.WithValues("ipAddress", getIPWithMask(ipAddress))

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

	if !ipAddress.DeletionTimestamp.IsZero() {
		ctx = log.IntoContext(ctx, logger)

		if err = r.reconcileDelete(ctx, ipAddress); err != nil {
			logger.Error(err, "unable to delete IP Address")
			return ctrl.Result{}, err
		}

		controllerutil.RemoveFinalizer(ipAddress, ipAddressFinalizer)
		if err = r.k8sClient.Update(ctx, ipAddress); err != nil {
			logger.Error(err, "unable to update CR after removing finalizer")
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	if notExists := controllerutil.AddFinalizer(ipAddress, ipAddressFinalizer); notExists {
		if err := r.k8sClient.Update(ctx, ipAddress); err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	deviceName, err := r.findDeviceName(ctx, req.Namespace, ipAddress)
	if err != nil {
		logger.Error(err, "unable to find device name")
		return ctrl.Result{}, err
	}

	logger = logger.WithValues("deviceName", deviceName)
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

	// ... perform update operations with r.netBox and ipAddress ...

	logger.Info("reconcile completed successfully")

	return ctrl.Result{}, nil
}

func getIPWithMask(ipaddr *ipamv1.IPAddress) string {
	if ipaddr == nil || ipaddr.Spec.Prefix == nil {
		return ""
	}

	return fmt.Sprintf("%s/%d", ipaddr.Spec.Address, *ipaddr.Spec.Prefix)
}

func (r *IPUpdateReconciler) reconcileDelete(ctx context.Context, ipAddr *ipamv1.IPAddress) error {
	logger := log.FromContext(ctx)
	logger.Info("deleting IP Address")

	if !controllerutil.ContainsFinalizer(ipAddr, ipAddressFinalizer) {
		return nil
	}

	ipStr := getIPWithMask(ipAddr)

	nbIP, err := r.netBox.IPAM().GetIPAddressByAddress(ipStr)
	if err != nil {
		logger.Error(err, "unable to find IP netbox ip address, NetBox IP will not be removed")
		return nil
	}

	if err = r.netBox.IPAM().DeleteIPAddress(nbIP.ID); err != nil {
		return fmt.Errorf("delete ip from netbox: %w", err)
	}

	return nil
}

func (r *IPUpdateReconciler) findDeviceName(ctx context.Context, namespace string, ipAddr *ipamv1.IPAddress) (string, error) {
	claimName := ipAddr.Spec.ClaimRef.Name

	ipClaim := &ipamv1.IPAddressClaim{}
	if err := r.k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: claimName}, ipClaim); err != nil {
		return "", fmt.Errorf("get IpAddressClaim: %w", err)
	}

	serverClaimName := getOwnerByKind(ipClaim.OwnerReferences, "ServerClaim")
	if serverClaimName == "" {
		return "", fmt.Errorf("no ServerClaim owner found for IpAddressClaim %s", ipClaim.Name)
	}

	serverClaim := &metalv1alpha1.ServerClaim{}
	if err := r.k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: serverClaimName}, serverClaim); err != nil {
		return "", fmt.Errorf("get ServerClaim: %w", err)
	}

	if serverClaim.Spec.ServerRef == nil {
		return "", fmt.Errorf("ServerClaim %s not yet bound to a server", serverClaimName)
	}

	server := &metalv1alpha1.Server{}
	if err := r.k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: serverClaim.Spec.ServerRef.Name}, server); err != nil {
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
