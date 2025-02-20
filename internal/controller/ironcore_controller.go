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
	"github.com/sapcc/argora/internal/netbox/virtualization"
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
	client.Client
	scheme            *runtime.Scheme
	cfg               *config.Config
	reconcileInterval time.Duration
	eventChannel      chan event.GenericEvent
}

func NewIronCoreReconciler(client client.Client, scheme *runtime.Scheme, config *config.Config, reconcileInterval time.Duration) *IronCoreReconciler {
	eventChannel := make(chan event.GenericEvent)
	return &IronCoreReconciler{
		Client:            client,
		scheme:            scheme,
		cfg:               config,
		reconcileInterval: reconcileInterval,
		eventChannel:      eventChannel,
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

	nbc, err := netbox.NewNetboxClient(r.cfg.NetboxUrl, r.cfg.NetboxToken)
	if err != nil {
		logger.Error(err, "unable to create netbox client")
		return ctrl.Result{}, err
	}

	var clusterRoles []string
	if r.cfg.IronCoreRoles != "" {
		clusterRoles = strings.Split(r.cfg.IronCoreRoles, ",")
	}

	for _, clusterRole := range clusterRoles {
		logger.Info("reconciling IronCore cluster role " + clusterRole + " in " + r.cfg.IronCoreRegion)

		cluster, err := virtualization.NewVirtualization(nbc).GetClusterByNameRegionRole("", r.cfg.IronCoreRegion, clusterRole)
		if err != nil {
			logger.Error(err, "unable to find cluster in netbox", "region", r.cfg.IronCoreRegion, "role", clusterRole)
			return ctrl.Result{}, err
		}

		devices, err := dcim.NewDCIM(nbc).GetDevicesByClusterID(cluster.ID)
		if err != nil {
			logger.Error(err, "unable to find devices for cluster", "cluster", cluster.Name, "ID", cluster.ID)
			return ctrl.Result{}, err
		}

		for _, device := range devices {
			err = r.ReconcileDevice(ctx, nbc, cluster.Name, &device)
			if err != nil {
				logger.Error(err, "unable to reconcile device")
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *IronCoreReconciler) SetupWithManager(mgr manager.Manager) error {
	src := source.Channel(r.eventChannel, handler.Funcs{})
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

func (r *IronCoreReconciler) ReconcileDevice(ctx context.Context, nbc *netbox.NetboxClient, cluster string, device *models.Device) error {
	logger := log.FromContext(ctx)
	logger.Info("reconciling device", "node", device.Name)
	// check if device is active
	if device.Status.Value != "active" {
		logger.Info("device is not active")
		return nil
	}
	// check if the host already exists
	bmcName := device.Name
	bmcObj := &metalv1alpha1.BMC{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: bmcName, Namespace: defaultNamespace}, bmcObj)
	if err == nil {
		logger.Info("bmc already exists", "bmc", bmcName)
		return nil
	}
	// ugly hack to get the bb this is not easy to get to in netbox
	nameParts := strings.Split(device.Name, "-")
	if len(nameParts) != 2 {
		err = fmt.Errorf("invalid device name: %s", device.Name)
		logger.Error(err, "error splitting name")
		return err
	}
	// get the region
	region, err := dcim.NewDCIM(nbc).GetRegionForDevice(device)
	if err != nil {
		logger.Error(err, "unable to lookup region for device")
		return err
	}

	oobIP, err := r.getOobIP(device)
	if err != nil {
		logger.Error(err, "unable to get OOB IP")
		return err
	}

	commonLabels := map[string]string{
		"topology.kubernetes.io/region":      region,
		"topology.kubernetes.io/zone":        device.Site.Slug,
		"kubernetes.metal.cloud.sap/cluster": cluster,
		"kubernetes.metal.cloud.sap/name":    device.Name,
		"kubernetes.metal.cloud.sap/bb":      nameParts[1],
		"kubernetes.metal.cloud.sap/type":    device.DeviceType.Slug,
		"kubernetes.metal.cloud.sap/role":    device.DeviceRole.Slug,
	}

	bmcSecret, err := r.createBmcSecret(ctx, device, commonLabels)
	if err != nil {
		logger.Error(err, "unable to create bmc secret")
		return err
	}

	bmc, err := r.createBmc(ctx, device, oobIP, bmcSecret, commonLabels)
	if err != nil {
		logger.Error(err, "unable to create BMC")
		return err
	}

	if err := r.setOwnerReferenceAndPatch(ctx, bmc, bmcSecret); err != nil {
		logger.Error(err, "unable to set owner reference and patch bmc secret")
		return err
	}

	return nil
}

func (r *IronCoreReconciler) createBmcSecret(
	ctx context.Context,
	device *models.Device,
	labels map[string]string,
) (*metalv1alpha1.BMCSecret, error) {

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
	err := r.Client.Create(ctx, bmcSecret)
	if apierrors.IsAlreadyExists(err) {
		log.FromContext(ctx).Info("bmc secret already exists", "bmcSecret", bmcSecret.Name)
		return bmcSecret, nil
	}
	if err != nil {
		return nil, fmt.Errorf("unable to create bmc secret: %w", err)
	}
	return bmcSecret, nil
}

func (r *IronCoreReconciler) createBmc(
	ctx context.Context,
	device *models.Device,
	oobIP string,
	bmcSecret *metalv1alpha1.BMCSecret,
	labels map[string]string,
) (*metalv1alpha1.BMC, error) {

	bmc := &metalv1alpha1.BMC{
		ObjectMeta: ctrl.ObjectMeta{
			Name:   device.Name,
			Labels: labels,
		},
		Spec: metalv1alpha1.BMCSpec{
			Endpoint: &metalv1alpha1.InlineEndpoint{
				IP: metalv1alpha1.MustParseIP(oobIP),
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
	if err := r.Client.Create(ctx, bmc); err != nil {
		if apierrors.IsAlreadyExists(err) {
			log.FromContext(ctx).Info("BMC already exists", "BMC", bmc.Name)
			return bmc, nil
		}
		return nil, fmt.Errorf("unable to create BMC: %w", err)
	}
	return bmc, nil
}

func (r *IronCoreReconciler) getOobIP(
	device *models.Device,
) (string, error) {

	oobIP := device.OOBIp.Address
	ip, _, err := net.ParseCIDR(oobIP)
	if err != nil {
		return "", fmt.Errorf("uncable to parse Device OOB IP: %w", err)
	}
	return ip.String(), nil
}

func (r *IronCoreReconciler) setOwnerReferenceAndPatch(ctx context.Context, owner, object client.Object) error {
	deepCopiedObject := object.DeepCopyObject().(client.Object)
	if err := controllerutil.SetControllerReference(owner, deepCopiedObject, r.scheme); err != nil {
		return fmt.Errorf("unable to set owner reference: %w", err)
	}
	if err := r.Client.Patch(ctx, deepCopiedObject, client.MergeFrom(object)); err != nil {
		return fmt.Errorf("unable to patch object: %w", err)
	}
	return nil
}
