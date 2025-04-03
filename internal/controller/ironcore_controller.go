// Copyright 2024 SAP SE
// SPDX-License-Identifier: Apache-2.0

// Package controller contains Argora operator controllers
package controller

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	metalv1alpha1 "github.com/ironcore-dev/metal-operator/api/v1alpha1"
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

	"github.com/sapcc/argora/internal/config"
	"github.com/sapcc/argora/internal/controller/periodic"
	"github.com/sapcc/argora/internal/netbox"
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
	netBox            netbox.Netbox
	reconcileInterval time.Duration
	eventChannel      chan event.GenericEvent
}

func NewIronCoreReconciler(client client.Client, scheme *runtime.Scheme, config *config.Config, netBox netbox.Netbox, reconcileInterval time.Duration) *IronCoreReconciler {
	return &IronCoreReconciler{
		k8sClient:         client,
		scheme:            scheme,
		cfg:               config,
		netBox:            netBox,
		reconcileInterval: reconcileInterval,
		eventChannel:      make(chan event.GenericEvent),
	}
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

// +kubebuilder:rbac:groups=metal.ironcore.dev,resources=servers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metal.ironcore.dev,resources=bmcs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metal.ironcore.dev,resources=bmcsecrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete

func (r *IronCoreReconciler) Reconcile(ctx context.Context, _ ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("reconciling ironcore")

	err := r.cfg.Reload()
	if err != nil {
		logger.Error(err, "unable to reload configuration")
		return ctrl.Result{}, err
	}

	logger.Info("configuration reloaded", "config", r.cfg)

	if r.cfg.ServerController != config.ControllerTypeIroncore {
		logger.Info("ironcore controller not enabled")
		return ctrl.Result{}, nil
	}

	err = r.netBox.Reload(r.cfg.NetboxURL, r.cfg.NetboxToken, logger)
	if err != nil {
		logger.Error(err, "unable to reload netbox")
		return ctrl.Result{}, err
	}

	clusterTypes := []string{""}
	if r.cfg.IronCore.Types != "" {
		clusterTypes = strings.Split(r.cfg.IronCore.Types, ",")
	}

	for _, clusterType := range clusterTypes {
		err = r.reconcileCluster(ctx, clusterType)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *IronCoreReconciler) reconcileCluster(ctx context.Context, clusterType string) error {
	logger := log.FromContext(ctx)
	logger.Info("reconciling IronCore clusters", "name", r.cfg.IronCore.Name, "region", r.cfg.IronCore.Region, "type", clusterType)

	clusters, err := r.netBox.Virtualization().GetClustersByNameRegionType(r.cfg.IronCore.Name, r.cfg.IronCore.Region, clusterType)
	if err != nil {
		logger.Error(err, "unable to find cluster in netbox", "name", r.cfg.IronCore.Name, "region", r.cfg.IronCore.Region, "type", clusterType)
		return err
	}

	if len(clusters) > 1 {
		return errors.New("multiple clusters found")
	}

	for _, cluster := range clusters {
		logger.Info("reconciling cluster", "cluster", cluster.Name, "ID", cluster.ID)

		devices, err := r.netBox.DCIM().GetDevicesByClusterID(cluster.ID)
		if err != nil {
			logger.Error(err, "unable to find devices for cluster", "cluster", cluster.Name, "ID", cluster.ID)
			return err
		}

		for _, device := range devices {
			err = r.reconcileDevice(ctx, r.netBox, cluster.Name, &device)
			if err != nil {
				logger.Error(err, "unable to reconcile device", "device", device.Name, "ID", device.ID)
				return err
			}
		}
	}
	return nil
}

func (r *IronCoreReconciler) reconcileDevice(ctx context.Context, netBox netbox.Netbox, cluster string, device *models.Device) error {
	logger := log.FromContext(ctx)
	logger.Info("reconciling device", "device", device.Name, "ID", device.ID)

	if device.Status.Value != "active" {
		logger.Info("device is not active, will skip", "status", device.Status.Value)
		return nil
	}

	bmcObj := &metalv1alpha1.BMC{}
	if err := r.k8sClient.Get(ctx, client.ObjectKey{Name: device.Name, Namespace: defaultNamespace}, bmcObj); err == nil {
		logger.Info("BMC custom resource already exists, will skip", "bmc", device.Name)
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

	logger.Info("created BMC Secret", "name", bmcSecret.Name)

	bmc, err := r.createBmc(ctx, device, oobIP, bmcSecret, commonLabels)
	if err != nil {
		return fmt.Errorf("unable to create bmc: %w", err)
	}

	logger.Info("created BMC CR", "name", bmc.Name)

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
