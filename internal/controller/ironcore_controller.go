// Copyright 2024 SAP SE
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	metalv1alpha1 "github.com/ironcore-dev/metal-operator/api/v1alpha1"
	"github.com/sapcc/argora/internal/config"
	"github.com/sapcc/argora/internal/controller/periodic"
	"github.com/sapcc/argora/internal/netbox"
	"github.com/sapcc/argora/internal/netbox/dcim"
	"github.com/sapcc/go-netbox-go/models"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	bmcProtocolRedfish = "Redfish"
	bmcPort            = 443
	defaultNamespace   = "default"
)

type IronCoreReconciler struct {
	k8sClient         client.Client
	scheme            *runtime.Scheme
	cfg               *config.Config
	reconcileInterval time.Duration
	eventChannel      chan event.GenericEvent
}

func NewIronCoreReconciler(client client.Client, scheme *runtime.Scheme, config *config.Config, reconcileInterval time.Duration) *IronCoreReconciler {
	return &IronCoreReconciler{
		k8sClient:         client,
		scheme:            scheme,
		cfg:               config,
		reconcileInterval: reconcileInterval,
		eventChannel:      make(chan event.GenericEvent),
	}
}

// +kubebuilder:rbac:groups=metal.ironcore.dev,resources=servers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metal.ironcore.dev,resources=bmcs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metal.ironcore.dev,resources=bmcsecrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete

func (r *IronCoreReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("reconciling ironcore")

	err := r.cfg.Reload()
	if err != nil {
		logger.Error(err, "unable to reload configuration")
		return ctrl.Result{}, err
	}

	logger.Info("configuration reloaded", "config", r.cfg)

	if r.cfg.ServerController != "ironcore" {
		logger.Info("controller not enabled")
		return ctrl.Result{}, nil
	}

	// netBox, err := netbox.NewNetbox(r.cfg.NetboxUrl, r.cfg.NetboxToken)
	// if err != nil {
	// 	logger.Error(err, "unable to create netbox client")
	// 	return ctrl.Result{}, err
	// }

	// var roles []string
	// if r.cfg.IronCoreRoles != "" {
	// 	roles = strings.Split(r.cfg.IronCoreRoles, ",")
	// }

	// for _, role := range roles {
	// 	logger.Info("reconciling IronCore cluster role " + role + " in " + r.cfg.IronCoreRegion)

	// 	cluster, err := virtualization.NewVirtualization(netBox.Virtualization).GetClusterByNameRegionType("", r.cfg.IronCoreRegion, role)
	// 	if err != nil {
	// 		logger.Error(err, "unable to find cluster in netbox", "region", r.cfg.IronCoreRegion, "type", role)
	// 		return ctrl.Result{}, err
	// 	}

	// 	devices, err := dcim.NewDCIM(netBox.DCIM).GetDevicesByClusterID(cluster.ID)
	// 	if err != nil {
	// 		logger.Error(err, "unable to find devices for cluster", "cluster", cluster.Name, "ID", cluster.ID)
	// 		return ctrl.Result{}, err
	// 	}

	// 	for _, device := range devices {
	// 		err = r.ReconcileDevice(ctx, netBox, cluster.Name, &device)
	// 		if err != nil {
	// 			logger.Error(err, "unable to reconcile device", "device", device.Name, "deviceID", device.ID)
	// 			return ctrl.Result{}, err
	// 		}
	// 	}
	// }

	return ctrl.Result{}, nil
}

func (r *IronCoreReconciler) SetupWithManager(mgr manager.Manager) error {
	src := source.Channel(r.eventChannel, &handler.EnqueueRequestForObject{})
	runner, err := periodic.NewRunner(
		periodic.WithClient(mgr.GetClient()),
		periodic.WithInterval(r.reconcileInterval),
		periodic.WithEventChannel(r.eventChannel),
	)

	if err != nil {
		return fmt.Errorf("unable to create periodic runner: %w", err)
	}

	if err := mgr.Add(runner); err != nil {
		return fmt.Errorf("unable to add periodic runner: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named("ironCore").
		WatchesRawSource(src).
		Complete(r)
}

func (r *IronCoreReconciler) ReconcileDevice(ctx context.Context, netBox *netbox.Netbox, cluster string, device *models.Device) error {
	logger := log.FromContext(ctx)
	logger.Info("reconciling device", "device", device.Name)

	if device.Status.Value != "active" {
		logger.Info("device is not active", "status", device.Status.Value)
		return nil
	}

	bmcObj := &metalv1alpha1.BMC{}
	if err := r.k8sClient.Get(ctx, client.ObjectKey{Name: device.Name, Namespace: defaultNamespace}, bmcObj); err == nil {
		logger.Info("bmc already exists", "bmc", device.Name)
		return nil
	}

	deviceNameParts := strings.Split(device.Name, "-")
	if len(deviceNameParts) != 2 {
		return fmt.Errorf("unable to split in two device name: %s", device.Name)
	}

	region, err := dcim.NewDCIM(netBox.DCIM).GetRegionForDevice(device)
	if err != nil {
		return fmt.Errorf("unable to get region for device: %w", err)
	}

	oobIP, err := getOobIP(device)
	if err != nil {
		return fmt.Errorf("unable to get OOB IP: %w", err)
	}

	commonLabels := map[string]string{
		"topology.kubernetes.io/region":      region,
		"topology.kubernetes.io/zone":        device.Site.Slug,
		"kubernetes.metal.cloud.sap/cluster": cluster,
		"kubernetes.metal.cloud.sap/name":    device.Name,
		"kubernetes.metal.cloud.sap/bb":      deviceNameParts[1],
		"kubernetes.metal.cloud.sap/type":    device.DeviceType.Slug,
		"kubernetes.metal.cloud.sap/role":    device.DeviceRole.Slug,
	}

	bmcSecret, err := r.createBmcSecret(ctx, device, commonLabels)
	if err != nil {
		return fmt.Errorf("unable to create bmc secret: %w", err)
	}

	bmc, err := r.createBmc(ctx, device, oobIP, bmcSecret, commonLabels)
	if err != nil {
		return fmt.Errorf("unable to create bmc: %w", err)
	}

	if err := r.setOwnerReferenceAndPatch(ctx, bmc, bmcSecret); err != nil {
		return err
	}
	return nil
}

func (r *IronCoreReconciler) createBmcSecret(ctx context.Context, device *models.Device, labels map[string]string) (*metalv1alpha1.BMCSecret, error) {
	logger := log.FromContext(ctx)

	user := r.cfg.BMCUser
	password := r.cfg.BMCPassword

	if user == "" || password == "" {
		return nil, errors.New("bmc user or password not set")
	}

	bmcSecret := &metalv1alpha1.BMCSecret{
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

func (r *IronCoreReconciler) setOwnerReferenceAndPatch(ctx context.Context, bmc *metalv1alpha1.BMC, bmcSecret *metalv1alpha1.BMCSecret) error {
	bmcSecretBase := bmcSecret.DeepCopy()
	if err := controllerutil.SetControllerReference(bmc, bmcSecret, r.scheme); err != nil {
		return fmt.Errorf("unable to set owner reference: %w", err)
	}

	if err := r.k8sClient.Patch(ctx, bmcSecret, client.MergeFrom(bmcSecretBase)); err != nil {
		return fmt.Errorf("unable to patch object: %w", err)
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
