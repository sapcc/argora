// Copyright 2024 SAP SE
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	metalv1alpha1 "github.com/ironcore-dev/metal-operator/api/v1alpha1"
	"github.com/sapcc/argora/internal/netbox"
	"github.com/sapcc/go-netbox-go/models"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"net"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"strings"
	"time"
)

const (
	bmcProtocolRedfish = "Redfish"
	bmcPort            = 443
	defaultNamespace   = "default"
)

type IronCoreServerController struct {
	client.Client
	Nb             *netbox.Client
	Scheme         *runtime.Scheme
	BMCUser        string
	BMCPassword    string
	IronCoreRegion string
	IronCoreRoles  string
}

// +kubebuilder:rbac:groups=metal.ironcore.dev,resources=servers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metal.ironcore.dev,resources=bmcs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metal.ironcore.dev,resources=bmcsecrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile looks up IronCore clusters in Netbox and creates Servers for it
func (c *IronCoreServerController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("reconciling IronCore cluster")

	var clusterRoles []string
	if c.IronCoreRoles != "" {
		clusterRoles = strings.Split(c.IronCoreRoles, ",")
	}

	for _, clusterRole := range clusterRoles {
		devices, cluster, err := c.Nb.LookupCluster(clusterRole, c.IronCoreRegion, "")
		if err != nil {
			logger.Error(err, "unable to lookup cluster in netbox")
			return ctrl.Result{}, err
		}
		for _, device := range devices {
			err = c.ReconcileDevice(ctx, cluster, &device)
			if err != nil {
				logger.Error(err, "unable to reconcile device")
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

type PeriodicReconciler struct {
	controller *IronCoreServerController
}

func (r *PeriodicReconciler) Start(ctx context.Context) error {
	// Trigger the Reconcile method immediately
	_, err := r.controller.Reconcile(ctx, ctrl.Request{})
	if err != nil {
		return err
	}

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			_, err := r.controller.Reconcile(ctx, ctrl.Request{})
			if err != nil {
				return err
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func (c *IronCoreServerController) AddToManager(mgr manager.Manager) error {
	// Add the custom Runnable to the manager
	return mgr.Add(&PeriodicReconciler{controller: c})
}

func (c *IronCoreServerController) ReconcileDevice(ctx context.Context, cluster string, device *models.Device) error {
	logger := log.FromContext(ctx)
	logger.Info("reconciling device", "node", device.Name)
	// check if device is active
	if device.Status.Value != "active" {
		logger.Info("device is not active")
		return nil
	}
	// check if the host already exists
	server := &metalv1alpha1.Server{}
	err := c.Client.Get(ctx, client.ObjectKey{Name: device.Name, Namespace: defaultNamespace}, server)
	if err == nil {
		logger.Info("host already exists", "host", server.Name)
		return nil
	}
	// ugly hack to get the role this is not easy to get to in netbox
	nameParts := strings.Split(device.Name, "-")
	if len(nameParts) != 2 {
		err = fmt.Errorf("invalid device name: %s", device.Name)
		logger.Error(err, "error splitting name")
		return err
	}
	// get the region
	region, err := c.Nb.GetRegionForDevice(device)
	if err != nil {
		logger.Error(err, "unable to lookup region for device")
		return err
	}

	oobIP, err := c.getOobIP(device)
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
	}

	bmcSecret, err := c.createBmcSecret(ctx, device, commonLabels)
	if err != nil {
		logger.Error(err, "unable to create bmc secret")
		return err
	}

	bmc, err := c.createBmc(ctx, device, oobIP, bmcSecret, commonLabels)
	if err != nil {
		logger.Error(err, "unable to create BMC")
		return err
	}

	metalServerName := fmt.Sprintf("%s-system-0", device.Name)
	metalServer := &metalv1alpha1.Server{
		ObjectMeta: ctrl.ObjectMeta{
			Name:   metalServerName,
			Labels: commonLabels,
		},
		Spec: metalv1alpha1.ServerSpec{
			BMCRef: &corev1.LocalObjectReference{
				Name: bmc.Name,
			},
		},
	}
	err = c.Client.Create(ctx, metalServer)
	if errors.IsAlreadyExists(err) {
		logger.Info("server already exists", "Server", bmcSecret.Name)
		return nil
	}
	if err != nil {
		logger.Error(err, "unable to create Server")
		return err
	}

	if err := controllerutil.SetControllerReference(server, bmcSecret, c.Scheme); err != nil {
		logger.Error(err, "unable to set owner reference for bmc secret")
		return err
	}
	err = c.Client.Patch(ctx, bmcSecret, client.MergeFrom(bmcSecret))
	if err != nil {
		logger.Error(err, "unable to patch bmcSecret")
		return err
	}

	if err := controllerutil.SetControllerReference(server, bmc, c.Scheme); err != nil {
		logger.Error(err, "unable to set owner reference for bmc")
		return err
	}
	err = c.Client.Patch(ctx, bmc, client.MergeFrom(bmc))
	if err != nil {
		logger.Error(err, "unable to patch BMC")
		return err
	}

	return nil
}

func (c *IronCoreServerController) createBmcSecret(
	ctx context.Context,
	device *models.Device,
	labels map[string]string,
) (*metalv1alpha1.BMCSecret, error) {
	user := c.BMCUser
	password := c.BMCPassword
	if user == "" || password == "" {
		return nil, fmt.Errorf("bmc user or password not set")
	}
	bmcSecret := &metalv1alpha1.BMCSecret{
		ObjectMeta: ctrl.ObjectMeta{
			Name:   device.Name,
			Labels: labels,
		},
		StringData: map[string]string{
			"username": user,
			"password": password,
		},
	}
	err := c.Client.Create(ctx, bmcSecret)
	if errors.IsAlreadyExists(err) {
		log.FromContext(ctx).Info("bmc secret already exists", "bmcSecret", bmcSecret.Name)
		return bmcSecret, nil
	}
	if err != nil {
		return nil, fmt.Errorf("unable to create bmc secret: %w", err)
	}
	return bmcSecret, nil
}

func (c *IronCoreServerController) createBmc(
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
	if err := c.Client.Create(ctx, bmc); err != nil {
		if errors.IsAlreadyExists(err) {
			log.FromContext(ctx).Info("BMC already exists", "BMC", bmc.Name)
			return bmc, nil
		}
		return nil, fmt.Errorf("unable to create BMC: %w", err)
	}
	return bmc, nil
}

func (c *IronCoreServerController) getOobIP(
	device *models.Device,
) (string, error) {
	oobIP := device.OOBIp.Address
	ip, _, err := net.ParseCIDR(oobIP)
	if err != nil {
		return "", fmt.Errorf("uncable to parse Device OOB IP: %w", err)
	}
	return ip.String(), nil
}
