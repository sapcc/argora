/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"time"

	argorav1alpha1 "github.com/sapcc/argora/api/v1alpha1"
	"github.com/sapcc/argora/internal/config"
	"github.com/sapcc/argora/internal/netbox"
	"github.com/sapcc/argora/internal/netbox/dcim"
	"github.com/sapcc/argora/internal/netbox/ipam"
	"github.com/sapcc/argora/internal/netbox/virtualization"
	"github.com/sapcc/argora/internal/status"
	"github.com/sapcc/go-netbox-go/models"
	"golang.org/x/time/rate"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// UpdateReconciler reconciles a Update object
type UpdateReconciler struct {
	k8sClient         client.Client
	scheme            *runtime.Scheme
	cfg               *config.Config
	reconcileInterval time.Duration
}

func NewUpdateReconciler(mgr ctrl.Manager, cfg *config.Config, reconcileInterval time.Duration) *UpdateReconciler {
	return &UpdateReconciler{
		k8sClient:         mgr.GetClient(),
		scheme:            mgr.GetScheme(),
		cfg:               cfg,
		reconcileInterval: reconcileInterval,
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *UpdateReconciler) SetupWithManager(mgr ctrl.Manager, rateLimiter RateLimiter) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&argorav1alpha1.Update{}).
		WithEventFilter(predicate.Or[client.Object](predicate.GenerationChangedPredicate{}, predicate.AnnotationChangedPredicate{})).
		WithOptions(controller.Options{
			RateLimiter: workqueue.NewTypedMaxOfRateLimiter[ctrl.Request](
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

	err := r.cfg.Reload()
	if err != nil {
		logger.Error(err, "unable to reload configuration")
		return ctrl.Result{}, err
	}

	logger.Info("configuration reloaded", "config", r.cfg)

	netBox, err := netbox.NewNetbox(r.cfg.NetboxUrl, r.cfg.NetboxToken)
	if err != nil {
		logger.Error(err, "unable to create netbox client")
		return ctrl.Result{}, err
	}

	updateCR := &argorav1alpha1.Update{}
	err = r.k8sClient.Get(ctx, req.NamespacedName, updateCR)
	if err != nil {
		logger.Error(err, "unable to get Update CR")
		return ctrl.Result{}, err
	}

	statusHandler := status.NewStatusHandler(r.k8sClient)

	logger.Info("cluster selections", "count", len(updateCR.Spec.ClusterSelection))
	for _, clusterSelection := range updateCR.Spec.ClusterSelection {
		cluster, err := virtualization.NewVirtualization(netBox.Virtualization).GetClusterByNameRegionType(clusterSelection.Name, clusterSelection.Region, clusterSelection.Type)
		if err != nil {
			logger.Error(err, "unable to find clusters for selection", "name", clusterSelection.Name, "region", clusterSelection.Region, "type", clusterSelection.Type)

			statusHandler.UpdateToError(ctx, updateCR, err)
			statusHandler.SetCondition(updateCR, argorav1alpha1.NewReasonWithMessage(argorav1alpha1.ConditionReasonUpdateFailed))

			return ctrl.Result{}, err
		}

		devices, err := dcim.NewDCIM(netBox.DCIM).GetDevicesByClusterID(cluster.ID)
		if err != nil {
			logger.Error(err, "unable to find devices for cluster", "name", cluster.Name, "ID", cluster.ID)

			statusHandler.UpdateToError(ctx, updateCR, err)
			statusHandler.SetCondition(updateCR, argorav1alpha1.NewReasonWithMessage(argorav1alpha1.ConditionReasonUpdateFailed))

			return ctrl.Result{}, err
		}

		for _, device := range devices {
			err = r.reconcileDevice(ctx, netBox, &device)
			if err != nil {
				logger.Error(err, "unable to reconcile device", "device", device.Name, "deviceID", device.ID)

				statusHandler.UpdateToError(ctx, updateCR, err)
				statusHandler.SetCondition(updateCR, argorav1alpha1.NewReasonWithMessage(argorav1alpha1.ConditionReasonUpdateFailed))

				return ctrl.Result{}, err
			}
		}
	}

	statusHandler.UpdateToReady(ctx, updateCR)
	statusHandler.SetCondition(updateCR, argorav1alpha1.NewReasonWithMessage(argorav1alpha1.ConditionReasonUpdateSucceeded))

	return ctrl.Result{RequeueAfter: r.reconcileInterval}, nil
}

func (r *UpdateReconciler) reconcileDevice(ctx context.Context, netBox *netbox.Netbox, device *models.Device) error {
	logger := log.FromContext(ctx)
	logger.Info("reconciling device", "device", device.Name, "deviceID", device.ID)

	if device.Status.Value != "active" {
		logger.Info("device is not active", "status", device.Status.Value)
		return nil
	}

	if err := r.updateDeviceData(ctx, netBox, device); err != nil {
		return fmt.Errorf("unable to update device %s data: %w", device.Name, err)
	}

	// if err := r.removeVMKInterfacesAndIPs(ctx, netBox, device); err != nil {
	// 	return err
	// }

	return nil
}

func (r *UpdateReconciler) updateDeviceData(ctx context.Context, netBox *netbox.Netbox, device *models.Device) error {
	logger := log.FromContext(ctx)
	logger.Info("updating device data", "name", device.Name, "ID", device.ID)

	dcimClient := dcim.NewDCIM(netBox.DCIM)
	ipamClient := ipam.NewIPAM(netBox.IPAM)

	iface, err := dcimClient.GetInterfaceForDevice(device, "remoteboard")
	if err != nil {
		return err
	}
	logger.Info("found remoteboard interface", "ID", iface.ID)

	ipAddress, err := ipamClient.GetIPAddressForInterface(iface.ID)
	if err != nil {
		return err
	}
	logger.Info("found IP address for remoteboard interface", "IP", ipAddress.Address)

	platform, err := dcimClient.GetPlatformByName("Linux KVM")
	if err != nil {
		return err
	}
	logger.Info("found platform", "name", platform.Name, "ID", platform.ID)

	if device.Platform.ID != platform.ID || device.OOBIp.ID != ipAddress.ID {
		wDevice := device.Writeable()
		wDevice.Platform = platform.ID
		wDevice.OOBIp = ipAddress.ID

		_, err = dcimClient.UpdateDevice(wDevice)
		if err != nil {
			return err
		}

		logger.Info("updated device data", "name", device.Name, "ID", device.ID)
	} else {
		logger.Info("device already has correct data", "name", device.Name, "ID", device.ID)
	}

	return nil
}

// func (r *UpdateReconciler) removeVMKInterfacesAndIPs(ctx context.Context, netBox *netbox.Netbox, device *models.Device) error {
// 	logger := log.FromContext(ctx)
// 	logger.Info("checking for vmk interfaces")

// 	ifaces, err := r.netBox.GetInterfacesForDevice(device)
// 	if err != nil {
// 		return err
// 	}

// 	for _, iface := range ifaces {
// 		if strings.HasPrefix(iface.Name, "vmk") {
// 			logger.Info("found %s interface to delete", iface.Name)

// 			ipAddresses, err := r.netBox.GetIPAddressesForInterface(iface.ID)
// 			if err != nil {
// 				return err
// 			}

// 			for _, ip := range ipAddresses {
// 				logger.Info("Deleting IP %s for interface %s", ip.Address, iface.Name)
// 				err := r.netBox.Ipam.DeleteIPAddress(ip.ID)
// 				if err != nil {
// 					logger.Info("unable to delete %s IP", ip.Address)
// 					return err
// 				}
// 			}

// 			logger.Info("Deleting interface %s", iface.Name)
// 			err = r.netBox.Dcim.DeleteInterface(iface.ID)
// 			if err != nil {
// 				logger.Info("unable to delete %s interface", iface.Name)
// 				return err
// 			}
// 		}
// 	}

// 	logger.Info("all vmk interfaces were deleted")
// 	return nil
// }
