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

	metalv1alpha1 "github.com/ironcore-dev/metal-operator/api/v1alpha1"
	"github.com/sapcc/go-netbox-go/models"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/ptr"
	ipamv1 "sigs.k8s.io/cluster-api/api/ipam/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/sapcc/argora/internal/credentials"
	"github.com/sapcc/argora/internal/netbox"
	"github.com/sapcc/argora/internal/netbox/ipam"

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

	prefix, err := getPrefix(ipAddress)
	if err != nil {
		logger.Error(err, "unable to get ip prefix")
		return ctrl.Result{}, err
	}

	logger = logger.WithValues("ipAddress", prefix.String())
	ctx = log.IntoContext(ctx, logger)

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
		if err = r.reconcileDelete(ctx, req.Namespace, ipAddress); err != nil {
			logger.Error(err, "unable to delete IP Address")
			return ctrl.Result{}, err
		}

		base := ipAddress.DeepCopy()
		if removed := controllerutil.RemoveFinalizer(ipAddress, ipAddressFinalizer); removed {
			if err := r.k8sClient.Patch(ctx, ipAddress, client.MergeFrom(base)); err != nil {
				logger.Error(err, "unable to remove finalizer")
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{}, nil
	}

	base := ipAddress.DeepCopy()
	if added := controllerutil.AddFinalizer(ipAddress, ipAddressFinalizer); added {
		if err := r.k8sClient.Patch(ctx, ipAddress, client.MergeFrom(base)); err != nil {
			logger.Error(err, "unable to add finalizer")
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	target, err := r.findNetboxTarget(ctx, req.Namespace, ipAddress)
	if err != nil {
		logger.Error(err, "unable to find target in NetBox")
		return ctrl.Result{}, err
	}

	logger = logger.WithValues("deviceName", target.device.Name, "interface", target.iface.Name)
	logger.Info("target device and interface are found")

	// ... perform update operations with r.netBox and ipAddress ...

	logger.Info("reconcile completed successfully")

	return ctrl.Result{}, nil
}

func getPrefix(address *ipamv1.IPAddress) (netip.Prefix, error) {
	addr, err := netip.ParseAddr(address.Spec.Address)
	if err != nil {
		return netip.Prefix{}, err
	}

	cidr := ptr.Deref(address.Spec.Prefix, 32)

	return netip.PrefixFrom(addr, int(cidr)), nil
}

func (r *IPUpdateReconciler) reconcileDelete(ctx context.Context, namespace string, ipAddr *ipamv1.IPAddress) error {
	logger := log.FromContext(ctx)
	logger.Info("deleting IP Address")

	prefix, err := getPrefix(ipAddr)
	if err != nil {
		return fmt.Errorf("unable to get ip prefix: %w", err)
	}

	ipStr := prefix.String()

	nbIP, err := r.netBox.IPAM().GetIPAddressByAddress(ipStr)
	if err != nil {
		if errors.Is(err, ipam.ErrNoObjectsFound) {
			logger.Info("IP not found in NetBox, nothing to delete")
			return nil
		}

		return fmt.Errorf("unable to find IP in NetBox: %w", err)
	}

	target, err := r.findNetboxTarget(ctx, namespace, ipAddr)
	if err != nil {
		logger.Info("could not determine expected interface, skipping safety check", "reason", err.Error())
		return nil
	}

	if nbIP.AssignedObjectID != target.iface.ID {
		logger.Info("IP is assigned to a different interface in NetBox; skipping deletion to prevent collapse",
			"actualInterfaceID", nbIP.AssignedObjectID, "expectedInterfaceID", target.iface.ID, "deviceName", target.device.Name,
		)
		return nil
	}

	if err = r.netBox.IPAM().DeleteIPAddress(nbIP.ID); err != nil {
		return fmt.Errorf("delete ip from netbox: %w", err)
	}

	return nil
}

type netboxTarget struct {
	device models.Device
	iface  models.Interface
}

func (r *IPUpdateReconciler) findNetboxTarget(ctx context.Context, namespace string, ipAddress *ipamv1.IPAddress) (*netboxTarget, error) {
	deviceName, err := r.findDeviceName(ctx, namespace, ipAddress)
	if err != nil {
		return nil, fmt.Errorf("unable to find device name: %w", err)
	}

	device, err := r.netBox.DCIM().GetDeviceByName(deviceName)
	if err != nil {
		return nil, fmt.Errorf("unable to find device by name: %w", err)
	}

	interfaces, err := r.netBox.DCIM().GetInterfacesForDevice(device)
	if err != nil {
		return nil, fmt.Errorf("unable to find interfaces for device %w", err)
	}

	targetInterface, err := r.findTargetInterface(interfaces)
	if err != nil {
		return nil, fmt.Errorf("unable to find target interface: %w", err)
	}

	return &netboxTarget{
		device: *device,
		iface:  targetInterface,
	}, nil
}

func (r *IPUpdateReconciler) findDeviceName(ctx context.Context, namespace string, ipAddr *ipamv1.IPAddress) (string, error) {
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
	if err := r.k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: serverClaimName}, serverClaim); err != nil {
		return "", fmt.Errorf("failed to get ServerClaim: %w", err)
	}

	if serverClaim.Spec.ServerRef == nil {
		return "", fmt.Errorf("ServerClaim %s not yet bound to a server", serverClaimName)
	}

	server := &metalv1alpha1.Server{}
	if err := r.k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: serverClaim.Spec.ServerRef.Name}, server); err != nil {
		return "", fmt.Errorf("failed to get Server referenced by ServerClaim: %w", err)
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
