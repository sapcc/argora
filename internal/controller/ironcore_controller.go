// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

// Package controller contains Argora operator controllers
package controller

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net"
	"strings"
	"time"

	argorav1alpha1 "github.com/sapcc/argora/api/v1alpha1"
	"github.com/sapcc/argora/internal/credentials"
	"github.com/sapcc/argora/internal/netbox"
	"github.com/sapcc/argora/internal/status"

	metalv1alpha1 "github.com/ironcore-dev/metal-operator/api/v1alpha1"
	"github.com/sapcc/go-netbox-go/models"

	"golang.org/x/time/rate"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	bmcProtocolRedfish = "Redfish"
	bmcPort            = 443
)

type IronCoreReconciler struct {
	k8sClient         client.Client
	scheme            *runtime.Scheme
	credentials       *credentials.Credentials
	statusHandler     status.ClusterImportStatus
	netBox            netbox.Netbox
	reconcileInterval time.Duration
	namespace         string
}

func NewIronCoreReconciler(mgr ctrl.Manager, creds *credentials.Credentials, statusHandler status.ClusterImportStatus, netBox netbox.Netbox, reconcileInterval time.Duration, namespace string) *IronCoreReconciler {
	return &IronCoreReconciler{
		k8sClient:         mgr.GetClient(),
		scheme:            mgr.GetScheme(),
		credentials:       creds,
		statusHandler:     statusHandler,
		netBox:            netBox,
		reconcileInterval: reconcileInterval,
		namespace:         namespace,
	}
}

func (r *IronCoreReconciler) SetupWithManager(mgr ctrl.Manager, rateLimiter RateLimiter) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&argorav1alpha1.ClusterImport{}).
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
		Named("ironcore").
		Complete(r)
}

// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=argora.cloud.sap,resources=clusterimports,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=argora.cloud.sap,resources=clusterimports/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=argora.cloud.sap,resources=clusterimports/finalizers,verbs=update
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metal.ironcore.dev,resources=servers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metal.ironcore.dev,resources=bmcs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metal.ironcore.dev,resources=bmcsecrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metal.ironcore.dev,resources=servernetworkconfigs,verbs=get;list;watch;create;update;patch

func (r *IronCoreReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("reconciling cluster import")

	clusterImportCR := &argorav1alpha1.ClusterImport{}
	err := r.k8sClient.Get(ctx, req.NamespacedName, clusterImportCR)
	if err != nil {
		logger.Error(err, "unable to get ClusterImport CR")
		return ctrl.Result{}, err
	}

	err = r.credentials.Reload()
	if err != nil {
		logger.Error(err, "unable to reload credentials")

		r.statusHandler.SetCondition(clusterImportCR, argorav1alpha1.NewReasonWithMessage(argorav1alpha1.ConditionReasonClusterImportFailed))
		if errUpdateStatus := r.statusHandler.UpdateToError(ctx, clusterImportCR, err); errUpdateStatus != nil {
			return ctrl.Result{}, errUpdateStatus
		}

		return ctrl.Result{}, err
	}

	logger.Info("credentials reloaded", "credentials", r.credentials)

	err = r.netBox.Reload(r.credentials.NetboxToken, logger)
	if err != nil {
		logger.Error(err, "unable to reload netbox")

		r.statusHandler.SetCondition(clusterImportCR, argorav1alpha1.NewReasonWithMessage(argorav1alpha1.ConditionReasonClusterImportFailed))
		if errUpdateStatus := r.statusHandler.UpdateToError(ctx, clusterImportCR, err); errUpdateStatus != nil {
			return ctrl.Result{}, errUpdateStatus
		}

		return ctrl.Result{}, err
	}

	for _, clusterSelector := range clusterImportCR.Spec.Clusters {
		err = r.reconcileClusterSelection(ctx, clusterImportCR, clusterSelector)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	r.statusHandler.SetCondition(clusterImportCR, argorav1alpha1.NewReasonWithMessage(argorav1alpha1.ConditionReasonClusterImportSucceeded))
	if errUpdateStatus := r.statusHandler.UpdateToReady(ctx, clusterImportCR); errUpdateStatus != nil {
		return ctrl.Result{}, errUpdateStatus
	}

	return ctrl.Result{RequeueAfter: r.reconcileInterval}, nil
}

func (r *IronCoreReconciler) reconcileClusterSelection(ctx context.Context, clusterImportCR *argorav1alpha1.ClusterImport, clusterSelector *argorav1alpha1.ClusterSelector) error {
	logger := log.FromContext(ctx)
	logger.Info("fetching clusters data", "name", clusterSelector.Name, "region", clusterSelector.Region, "type", clusterSelector.Type)

	clusters, err := r.netBox.Virtualization().GetClustersByNameRegionType(clusterSelector.Name, clusterSelector.Region, clusterSelector.Type)
	if err != nil {
		logger.Error(err, "unable to find clusters in netbox", "name", clusterSelector.Name, "region", clusterSelector.Region, "type", clusterSelector.Type)

		r.statusHandler.SetCondition(clusterImportCR, argorav1alpha1.NewReasonWithMessage(argorav1alpha1.ConditionReasonClusterImportFailed))
		if errUpdateStatus := r.statusHandler.UpdateToError(ctx, clusterImportCR, fmt.Errorf("unable to reconcile cluster: %w", err)); errUpdateStatus != nil {
			return errUpdateStatus
		}

		return err
	}

	for _, cluster := range clusters {
		logger.Info("reconciling cluster", "cluster", cluster.Name, "ID", cluster.ID)

		devices, err := r.netBox.DCIM().GetDevicesByClusterID(cluster.ID)
		if err != nil {
			logger.Error(err, "unable to find devices for cluster", "cluster", cluster.Name, "ID", cluster.ID)

			r.statusHandler.SetCondition(clusterImportCR, argorav1alpha1.NewReasonWithMessage(argorav1alpha1.ConditionReasonClusterImportFailed))
			if errUpdateStatus := r.statusHandler.UpdateToError(ctx, clusterImportCR, fmt.Errorf("unable to reconcile devices on cluster %s (%d): %w", cluster.Name, cluster.ID, err)); errUpdateStatus != nil {
				return errUpdateStatus
			}

			return err
		}

		for _, device := range devices {
			err = r.reconcileDevice(ctx, r.netBox, &cluster, &device)
			if err != nil {
				logger.Error(err, "unable to reconcile device", "device", device.Name, "ID", device.ID)

				r.statusHandler.SetCondition(clusterImportCR, argorav1alpha1.NewReasonWithMessage(argorav1alpha1.ConditionReasonClusterImportFailed))
				if errUpdateStatus := r.statusHandler.UpdateToError(ctx, clusterImportCR, fmt.Errorf("unable to reconcile device %s (%d) on cluster %s (%d): %w", device.Name, device.ID, cluster.Name, cluster.ID, err)); errUpdateStatus != nil {
					return errUpdateStatus
				}

				return err
			}
		}
	}

	return nil
}

func (r *IronCoreReconciler) reconcileDevice(ctx context.Context, netBox netbox.Netbox, cluster *models.Cluster, device *models.Device) error {
	logger := log.FromContext(ctx)
	logger.Info("reconciling device", "device", device.Name, "ID", device.ID)

	if device.Status.Value != "active" {
		logger.Info("device is not active, will skip", "status", device.Status.Value)
		return nil
	}

	deviceNameParts := strings.Split(device.Name, "-")
	if len(deviceNameParts) != 2 {
		return fmt.Errorf("unable to split in two device name: %s", device.Name)
	}

	region, err := netBox.DCIM().GetRegionForDevice(device)
	if err != nil {
		return fmt.Errorf("unable to get region for device: %w", err)
	}

	oobIP, err := getOobIP(device)
	if err != nil {
		return fmt.Errorf("unable to get OOB IP: %w", err)
	}

	commonLabels := map[string]string{
		"topology.kubernetes.io/region":           region,
		"topology.kubernetes.io/zone":             device.Site.Slug,
		"kubernetes.metal.cloud.sap/cluster":      cluster.Name,
		"kubernetes.metal.cloud.sap/cluster-type": cluster.Type.Slug,
		"kubernetes.metal.cloud.sap/name":         device.Name,
		"kubernetes.metal.cloud.sap/bb":           deviceNameParts[1],
		"kubernetes.metal.cloud.sap/type":         device.DeviceType.Slug,
		"kubernetes.metal.cloud.sap/role":         device.DeviceRole.Slug,
		"kubernetes.metal.cloud.sap/platform":     device.Platform.Slug,
	}

	bmcObj := &metalv1alpha1.BMC{}
	if err := r.k8sClient.Get(ctx, client.ObjectKey{Name: device.Name}, bmcObj); err == nil {
		if err := r.patchBMCLabels(ctx, bmcObj, commonLabels); err != nil {
			return fmt.Errorf("unable to patch BMC labels: %w", err)
		}

		if err := r.updateServerNetworkConfig(ctx, device, commonLabels); err != nil {
			return fmt.Errorf("unable to update ServerNetworkConfig: %w", err)
		}

		logger.Info("BMC custom resource already exists, will skip", "bmc", device.Name)
		return nil
	}

	bmcSecret, err := r.createBmcSecret(ctx, device, commonLabels)
	if err != nil {
		return fmt.Errorf("unable to create bmc secret: %w", err)
	}

	logger.Info("created BMC Secret", "name", bmcSecret.Name)

	bmc, err := r.createBmc(ctx, device, oobIP, bmcSecret, commonLabels)
	if err != nil {
		return fmt.Errorf("unable to create bmc: %w", err)
	}

	logger.Info("created BMC CR", "name", bmc.Name)

	if err := r.createServerNetworkConfig(ctx, device, commonLabels); err != nil {
		return fmt.Errorf("unable to create ServerNetworkConfig: %w", err)
	}

	if err := r.patchOwnerReference(ctx, bmc, bmcSecret); err != nil {
		return err
	}
	return nil
}

func (r *IronCoreReconciler) createBmcSecret(ctx context.Context, device *models.Device, labels map[string]string) (*metalv1alpha1.BMCSecret, error) {
	logger := log.FromContext(ctx)

	user := r.credentials.BMCUser
	password := r.credentials.BMCPassword

	if user == "" || password == "" {
		return nil, errors.New("bmc user or password not set")
	}

	bmcSecret := &metalv1alpha1.BMCSecret{
		TypeMeta: ctrl.TypeMeta{
			APIVersion: metalv1alpha1.GroupVersion.String(),
			Kind:       "BMCSecret",
		},
		ObjectMeta: ctrl.ObjectMeta{
			Name:   device.Name,
			Labels: labels,
		},
		Data: map[string][]byte{
			metalv1alpha1.BMCSecretUsernameKeyName: []byte(user),
			metalv1alpha1.BMCSecretPasswordKeyName: []byte(password),
		},
	}

	if err := r.k8sClient.Create(ctx, bmcSecret); err != nil {
		if apierrors.IsAlreadyExists(err) { // TODO: if its already exists, can we assume that the secret is correct?
			logger.Info("bmc secret already exists", "bmcSecret", bmcSecret.Name)
			return bmcSecret, nil
		}
		return nil, err
	}

	return bmcSecret, nil
}

func (r *IronCoreReconciler) createBmc(ctx context.Context, device *models.Device, oobIP string, bmcSecret *metalv1alpha1.BMCSecret, labels map[string]string) (*metalv1alpha1.BMC, error) {
	logger := log.FromContext(ctx)

	ip, err := metalv1alpha1.ParseIP(oobIP)
	if err != nil {
		return nil, fmt.Errorf("unable to parse OOB IP: %w", err)
	}

	bmc := &metalv1alpha1.BMC{
		TypeMeta: ctrl.TypeMeta{
			APIVersion: metalv1alpha1.GroupVersion.String(),
			Kind:       "BMC",
		},
		ObjectMeta: ctrl.ObjectMeta{
			Name:   device.Name,
			Labels: labels,
		},
		Spec: metalv1alpha1.BMCSpec{
			Endpoint: &metalv1alpha1.InlineEndpoint{
				IP: ip,
			},
			Protocol: metalv1alpha1.Protocol{
				Name: bmcProtocolRedfish,
				Port: bmcPort,
			},
			BMCSecretRef: corev1.LocalObjectReference{
				Name: bmcSecret.Name,
			},
		},
	}

	if err := r.k8sClient.Create(ctx, bmc); err != nil {
		if apierrors.IsAlreadyExists(err) { // TODO: if its already exists, can we assume that the BMC is correct?
			logger.Info("BMC already exists", "BMC", bmc.Name)
			return bmc, nil
		}
		return nil, fmt.Errorf("unable to create BMC: %w", err)
	}

	return bmc, nil
}

func (r *IronCoreReconciler) patchOwnerReference(ctx context.Context, bmc *metalv1alpha1.BMC, bmcSecret *metalv1alpha1.BMCSecret) error {
	bmcSecretBase := bmcSecret.DeepCopy()
	if err := controllerutil.SetControllerReference(bmc, bmcSecret, r.scheme); err != nil {
		return fmt.Errorf("unable to set owner reference: %w", err)
	}

	if err := r.k8sClient.Patch(ctx, bmcSecret, client.MergeFrom(bmcSecretBase)); err != nil {
		return fmt.Errorf("unable to patch object: %w", err)
	}

	return nil
}

func (r *IronCoreReconciler) buildExpectedInterfaces(device *models.Device) ([]metalv1alpha1.ExpectedNetworkInterface, error) {
	ifaces, err := r.netBox.DCIM().GetInterfacesForDevice(device)
	if err != nil {
		return nil, fmt.Errorf("failed to get interfaces for device %s: %w", device.Name, err)
	}

	var expected []metalv1alpha1.ExpectedNetworkInterface
	for _, iface := range ifaces {
		if iface.MacAddress == "" {
			continue
		}
		if iface.Type.Value == "lag" {
			continue
		}
		if iface.MgmtOnly {
			continue
		}
		if len(iface.ConnectedEndpoints) == 0 {
			continue
		}

		endpoint := iface.ConnectedEndpoints[0]
		if endpoint.Device.Name == "" {
			continue
		}

		expected = append(expected, metalv1alpha1.ExpectedNetworkInterface{
			Name:       iface.Name,
			MACAddress: iface.MacAddress,
			Switch:     endpoint.Device.Name,
			Port:       endpoint.Name,
		})
	}
	return expected, nil
}

func (r *IronCoreReconciler) createServerNetworkConfig(ctx context.Context, device *models.Device, labels map[string]string) error {
	ifaces, err := r.buildExpectedInterfaces(device)
	if err != nil {
		return err
	}

	snc := &metalv1alpha1.ServerNetworkConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: metalv1alpha1.GroupVersion.String(),
			Kind:       "ServerNetworkConfig",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      device.Name,
			Namespace: r.namespace,
			Labels:    labels,
		},
		Spec: metalv1alpha1.ServerNetworkConfigSpec{
			ServerRef:  corev1.LocalObjectReference{Name: device.Name},
			Interfaces: ifaces,
		},
	}

	if err := r.k8sClient.Create(ctx, snc); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create ServerNetworkConfig for %s: %w", device.Name, err)
	}
	return nil
}

func (r *IronCoreReconciler) updateServerNetworkConfig(ctx context.Context, device *models.Device, labels map[string]string) error {
	ifaces, err := r.buildExpectedInterfaces(device)
	if err != nil {
		return err
	}

	snc := &metalv1alpha1.ServerNetworkConfig{}
	if err := r.k8sClient.Get(ctx, client.ObjectKey{Name: device.Name, Namespace: r.namespace}, snc); err != nil {
		if apierrors.IsNotFound(err) {
			return r.createServerNetworkConfig(ctx, device, labels)
		}
		return fmt.Errorf("failed to get ServerNetworkConfig for %s: %w", device.Name, err)
	}

	sncBase := snc.DeepCopy()
	snc.Spec.Interfaces = ifaces
	if snc.Labels == nil {
		snc.Labels = make(map[string]string)
	}
	maps.Copy(snc.Labels, labels)

	if err := r.k8sClient.Patch(ctx, snc, client.MergeFrom(sncBase)); err != nil {
		return fmt.Errorf("failed to patch ServerNetworkConfig for %s: %w", device.Name, err)
	}
	return nil
}

func getOobIP(device *models.Device) (string, error) {
	oobIP := device.OOBIp.Address
	ip, _, err := net.ParseCIDR(oobIP)

	if err != nil {
		return "", fmt.Errorf("uncable to parse device OOB IP: %w", err)
	}

	return ip.String(), nil
}

func (r *IronCoreReconciler) patchBMCLabels(ctx context.Context, bmc *metalv1alpha1.BMC, labels map[string]string) error {
	logger := log.FromContext(ctx)
	logger.Info("patching BMC labels", "bmc", bmc.Name)

	bmcBase := bmc.DeepCopy()
	if bmc.Labels == nil {
		bmc.Labels = make(map[string]string)
	}
	maps.Copy(bmc.Labels, labels)

	if err := r.k8sClient.Patch(ctx, bmc, client.MergeFrom(bmcBase)); err != nil {
		logger.Error(err, "failed to patch BMC labels")
		return err
	}

	if bmc.Spec.BMCSecretRef.Name != "" {
		bmcSecret := &metalv1alpha1.BMCSecret{}

		if err := r.k8sClient.Get(ctx, client.ObjectKey{Name: bmc.Spec.BMCSecretRef.Name}, bmcSecret); err != nil {
			logger.Error(err, "failed to get BMC secret")
			return err
		}

		bmcSecretBase := bmcSecret.DeepCopy()
		if bmcSecret.Labels == nil {
			bmcSecret.Labels = make(map[string]string)
		}
		maps.Copy(bmcSecret.Labels, labels)

		if err := r.k8sClient.Patch(ctx, bmcSecret, client.MergeFrom(bmcSecretBase)); err != nil {
			logger.Error(err, "failed to patch BMC secret labels")
			return err
		}
	}

	return nil
}
