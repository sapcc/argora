// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/sapcc/go-netbox-go/models"
	"golang.org/x/time/rate"

	argorav1alpha1 "github.com/sapcc/argora/api/v1alpha1"
	"github.com/sapcc/argora/internal/credentials"
	"github.com/sapcc/argora/internal/netbox"
	"github.com/sapcc/argora/internal/status"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

var interfacesToRename = []string{
	"iLO",      // HPE
	"iDRAC",    // Dell
	"imm",      // Lenovo
	"XClarity", // Lenovo
	"cimc",     // Cisco
}

// UpdateReconciler reconciles a Update object
type UpdateReconciler struct {
	k8sClient         client.Client
	scheme            *runtime.Scheme
	credentials       *credentials.Credentials
	statusHandler     status.UpdateStatus
	netBox            netbox.Netbox
	reconcileInterval time.Duration
}

func NewUpdateReconciler(mgr ctrl.Manager, creds *credentials.Credentials, statusHandler status.UpdateStatus, netBox netbox.Netbox, reconcileInterval time.Duration) *UpdateReconciler {
	return &UpdateReconciler{
		k8sClient:         mgr.GetClient(),
		scheme:            mgr.GetScheme(),
		credentials:       creds,
		statusHandler:     statusHandler,
		netBox:            netBox,
		reconcileInterval: reconcileInterval,
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *UpdateReconciler) SetupWithManager(mgr ctrl.Manager, rateLimiter RateLimiter) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&argorav1alpha1.Update{}).
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
		Named("update").
		Complete(r)
}

// +kubebuilder:rbac:groups=argora.cloud.sap,resources=updates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=argora.cloud.sap,resources=updates/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=argora.cloud.sap,resources=updates/finalizers,verbs=update

func (r *UpdateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("reconciling update")

	updateCR := &argorav1alpha1.Update{}
	err := r.k8sClient.Get(ctx, req.NamespacedName, updateCR)
	if err != nil {
		logger.Error(err, "unable to get Update CR")
		return ctrl.Result{}, err
	}

	err = r.credentials.Reload()
	if err != nil {
		logger.Error(err, "unable to reload credentials")

		r.statusHandler.SetCondition(updateCR, argorav1alpha1.NewReasonWithMessage(argorav1alpha1.ConditionReasonUpdateFailed))
		if errUpdateStatus := r.statusHandler.UpdateToError(ctx, updateCR, err); errUpdateStatus != nil {
			return ctrl.Result{}, errUpdateStatus
		}

		return ctrl.Result{}, err
	}

	logger.Info("credentials reloaded", "credentials", r.credentials)

	err = r.netBox.Reload(r.credentials.NetboxToken, logger)
	if err != nil {
		logger.Error(err, "unable to reload netbox")

		r.statusHandler.SetCondition(updateCR, argorav1alpha1.NewReasonWithMessage(argorav1alpha1.ConditionReasonUpdateFailed))
		if errUpdateStatus := r.statusHandler.UpdateToError(ctx, updateCR, err); errUpdateStatus != nil {
			return ctrl.Result{}, errUpdateStatus
		}

		return ctrl.Result{}, err
	}

	for _, clusterSelector := range updateCR.Spec.Clusters {
		err = r.reconcileClusterSelection(ctx, updateCR, clusterSelector)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	r.statusHandler.SetCondition(updateCR, argorav1alpha1.NewReasonWithMessage(argorav1alpha1.ConditionReasonUpdateSucceeded))
	if errUpdateStatus := r.statusHandler.UpdateToReady(ctx, updateCR); errUpdateStatus != nil {
		return ctrl.Result{}, errUpdateStatus
	}

	return ctrl.Result{RequeueAfter: r.reconcileInterval}, nil
}

func (r *UpdateReconciler) reconcileClusterSelection(ctx context.Context, updateCR *argorav1alpha1.Update, clusterSelector *argorav1alpha1.ClusterSelector) error {
	logger := log.FromContext(ctx)
	logger.Info("fetching clusters data", "name", clusterSelector.Name, "region", clusterSelector.Region, "type", clusterSelector.Type)

	clusters, err := r.netBox.Virtualization().GetClustersByNameRegionType(clusterSelector.Name, clusterSelector.Region, clusterSelector.Type)
	if err != nil {
		logger.Error(err, "unable to find clusters", "name", clusterSelector.Name, "region", clusterSelector.Region, "type", clusterSelector.Type)

		r.statusHandler.SetCondition(updateCR, argorav1alpha1.NewReasonWithMessage(argorav1alpha1.ConditionReasonUpdateFailed))
		if errUpdateStatus := r.statusHandler.UpdateToError(ctx, updateCR, fmt.Errorf("unable to reconcile cluster: %w", err)); errUpdateStatus != nil {
			return errUpdateStatus
		}

		return err
	}

	for _, cluster := range clusters {
		logger.Info("reconciling cluster", "name", cluster.Name, "ID", cluster.ID)

		devices, err := r.netBox.DCIM().GetDevicesByClusterID(cluster.ID)
		if err != nil {
			logger.Error(err, "unable to find devices for cluster", "name", cluster.Name, "ID", cluster.ID)

			r.statusHandler.SetCondition(updateCR, argorav1alpha1.NewReasonWithMessage(argorav1alpha1.ConditionReasonUpdateFailed))
			if errUpdateStatus := r.statusHandler.UpdateToError(ctx, updateCR, fmt.Errorf("unable to reconcile devices on cluster %s (%d): %w", cluster.Name, cluster.ID, err)); errUpdateStatus != nil {
				return errUpdateStatus
			}

			return err
		}

		for _, device := range devices {
			err = r.reconcileDevice(ctx, r.netBox, &device)
			if err != nil {
				logger.Error(err, "unable to reconcile device", "cluster", cluster.Name, "clusterID", cluster.ID, "device", device.Name, "deviceID", device.ID)

				r.statusHandler.SetCondition(updateCR, argorav1alpha1.NewReasonWithMessage(argorav1alpha1.ConditionReasonUpdateFailed))
				if errUpdateStatus := r.statusHandler.UpdateToError(ctx, updateCR, fmt.Errorf("unable to reconcile device %s (%d) on cluster %s (%d): %w", device.Name, device.ID, cluster.Name, cluster.ID, err)); errUpdateStatus != nil {
					return errUpdateStatus
				}

				return err
			}
		}
	}
	return nil
}

func (r *UpdateReconciler) reconcileDevice(ctx context.Context, netBox netbox.Netbox, device *models.Device) error {
	logger := log.FromContext(ctx)
	logger.Info("reconciling device", "device", device.Name, "ID", device.ID)

	if !slices.Contains([]string{"active", "staged"}, device.Status.Value) {
		logger.Info("device is neither active or staged, will skip", "status", device.Status.Value)
		return nil
	}

	if err := r.renameRemoteboardInterface(ctx, netBox, device); err != nil {
		return fmt.Errorf("unable to rename remoteboard interface for device %s: %w", device.Name, err)
	}

	if err := r.updateDeviceData(ctx, netBox, device); err != nil {
		return fmt.Errorf("unable to update device %s data: %w", device.Name, err)
	}

	if err := r.removeVMKInterfacesAndIPs(ctx, netBox, device); err != nil {
		return fmt.Errorf("unable to remove vmk interfaces and IPs for device %s: %w", device.Name, err)
	}

	return nil
}

func (r *UpdateReconciler) renameRemoteboardInterface(ctx context.Context, netBox netbox.Netbox, device *models.Device) error {
	logger := log.FromContext(ctx)

	ifaces, err := netBox.DCIM().GetInterfacesForDevice(device)
	if err != nil {
		return err
	}

	for _, iface := range ifaces {
		if slices.Contains(interfacesToRename, iface.Name) {
			wIface := models.WritableInterface{
				Name:   "remoteboard",
				Device: device.ID,
				Type:   iface.Type.Value,
			}

			_, err := netBox.DCIM().UpdateInterface(wIface, iface.ID)
			if err != nil {
				return fmt.Errorf("unable to rename %s interface: %w", iface.Name, err)
			}

			logger.Info("interface was renamed to remoteboard", "name", iface.Name, "ID", iface.ID)
		}
	}

	return nil
}

func (r *UpdateReconciler) updateDeviceData(ctx context.Context, netBox netbox.Netbox, device *models.Device) error {
	logger := log.FromContext(ctx)

	iface, err := netBox.DCIM().GetInterfaceForDevice(device, "remoteboard")
	if err != nil {
		return err
	}

	ipAddress, err := netBox.IPAM().GetIPAddressForInterface(iface.ID)
	if err != nil {
		return err
	}

	platform, err := netBox.DCIM().GetPlatformByName("GardenLinux")
	if err != nil {
		return err
	}

	if device.Platform.ID != platform.ID || device.OOBIp.ID != ipAddress.ID {
		wDevice := device.Writeable()
		wDevice.Platform = platform.ID
		wDevice.OOBIp = ipAddress.ID

		_, err = netBox.DCIM().UpdateDevice(wDevice)
		if err != nil {
			return err
		}

		logger.Info("updated device data", "name", device.Name, "ID", device.ID)
	} else {
		logger.Info("device already has correct data", "name", device.Name, "ID", device.ID)
	}

	return nil
}

func (r *UpdateReconciler) removeVMKInterfacesAndIPs(ctx context.Context, netBox netbox.Netbox, device *models.Device) error {
	logger := log.FromContext(ctx)

	ifaces, err := netBox.DCIM().GetInterfacesForDevice(device)
	if err != nil {
		return err
	}

	hasVmkInterfaces := false
	for _, iface := range ifaces {
		if strings.HasPrefix(iface.Name, "vmk") {
			logger.Info("found interface to delete", "name", iface.Name)

			if !hasVmkInterfaces {
				hasVmkInterfaces = true
			}

			ipAddresses, err := netBox.IPAM().GetIPAddressesForInterface(iface.ID)
			if err != nil {
				return err
			}

			for _, ip := range ipAddresses {
				err := netBox.IPAM().DeleteIPAddress(ip.ID)
				if err != nil {
					return fmt.Errorf("unable to delete IP address (%s): %w", ip.Address, err)
				}
				logger.Info("deleted IP for interface", "IP", ip.Address, "interface", iface.Name)
			}

			err = netBox.DCIM().DeleteInterface(iface.ID)
			if err != nil {
				return fmt.Errorf("unable to delete %s interface: %w", iface.Name, err)
			}
			logger.Info("deleted interface", "name", iface.Name)
		}
	}

	if hasVmkInterfaces {
		logger.Info("device vmk interfaces were deleted", "device", device.Name, "ID", device.ID)
	} else {
		logger.Info("device do not have vmk interfaces", "device", device.Name, "ID", device.ID)
	}

	return nil
}
