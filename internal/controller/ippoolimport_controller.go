// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/sapcc/go-netbox-go/models"
	"golang.org/x/time/rate"
	"k8s.io/client-go/util/workqueue"
	ipamv1alpha2 "sigs.k8s.io/cluster-api-ipam-provider-in-cluster/api/v1alpha2"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	argorav1alpha1 "github.com/sapcc/argora/api/v1alpha1"
	"github.com/sapcc/argora/internal/credentials"
	"github.com/sapcc/argora/internal/netbox"
	"github.com/sapcc/argora/internal/status"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ComputeTransitPrefixRoleName = "kubernetes-compute-transit"
)

// IPPoolImportReconciler reconciles a IPPoolImport object
type IPPoolImportReconciler struct {
	k8sClient         client.Client
	scheme            *runtime.Scheme
	credentials       *credentials.Credentials
	statusHandler     status.IPPoolImportStatus
	netBox            netbox.Netbox
	reconcileInterval time.Duration
}

func NewIPPoolImportReconciler(mgr ctrl.Manager, creds *credentials.Credentials, statusHandler status.IPPoolImportStatus, netBox netbox.Netbox, reconcileInterval time.Duration) *IPPoolImportReconciler {
	return &IPPoolImportReconciler{
		k8sClient:         mgr.GetClient(),
		scheme:            mgr.GetScheme(),
		credentials:       creds,
		statusHandler:     statusHandler,
		netBox:            netBox,
		reconcileInterval: reconcileInterval,
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *IPPoolImportReconciler) SetupWithManager(mgr ctrl.Manager, rateLimiter RateLimiter) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&argorav1alpha1.IPPoolImport{}).
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
		Named("ippoolimport").
		Complete(r)
}

// +kubebuilder:rbac:groups=argora.cloud.sap,resources=ippoolimports,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=argora.cloud.sap,resources=ippoolimports/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=argora.cloud.sap,resources=ippoolimports/finalizers,verbs=update
// +kubebuilder:rbac:groups=ipam.cluster.x-k8s.io,resources=globalinclusterippools,verbs=get;list;watch;create;update;patch

func (r *IPPoolImportReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("reconciling IPPoolImport")

	importCR := &argorav1alpha1.IPPoolImport{}
	err := r.k8sClient.Get(ctx, req.NamespacedName, importCR)
	if err != nil {
		logger.Error(err, "failed to get IPPoolImport")
		return ctrl.Result{}, err
	}

	err = r.credentials.Reload()
	if err != nil {
		logger.Error(err, "unable to reload credentials")

		r.statusHandler.SetCondition(importCR, argorav1alpha1.NewReasonWithMessage(argorav1alpha1.ConditionReasonIPPoolImportFailed))
		if errUpdateStatus := r.statusHandler.UpdateToError(ctx, importCR, err); errUpdateStatus != nil {
			return ctrl.Result{}, errUpdateStatus
		}

		return ctrl.Result{}, err
	}

	logger.Info("credentials reloaded", "credentials", r.credentials)

	err = r.netBox.Reload(r.credentials.NetboxToken, logger)
	if err != nil {
		logger.Error(err, "unable to reload netbox")

		r.statusHandler.SetCondition(importCR, argorav1alpha1.NewReasonWithMessage(argorav1alpha1.ConditionReasonIPPoolImportFailed))
		if errUpdateStatus := r.statusHandler.UpdateToError(ctx, importCR, err); errUpdateStatus != nil {
			return ctrl.Result{}, errUpdateStatus
		}

		return ctrl.Result{}, err
	}

	for _, ipPoolSelector := range importCR.Spec.IPPools {
		err = r.reconcileIPPoolSelection(ctx, importCR, ipPoolSelector)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	r.statusHandler.SetCondition(importCR, argorav1alpha1.NewReasonWithMessage(argorav1alpha1.ConditionReasonIPPoolImportSucceeded))
	if errUpdateStatus := r.statusHandler.UpdateToReady(ctx, importCR); errUpdateStatus != nil {
		return ctrl.Result{}, errUpdateStatus
	}

	return ctrl.Result{RequeueAfter: r.reconcileInterval}, nil
}

func (r *IPPoolImportReconciler) reconcileIPPoolSelection(ctx context.Context, importCR *argorav1alpha1.IPPoolImport, ipPoolSelector *argorav1alpha1.IPPoolSelector) error {
	logger := log.FromContext(ctx)
	logger.Info("fetching prefixes", "region", ipPoolSelector.Region, "role", ipPoolSelector.Role)

	prefixes, err := r.netBox.IPAM().GetPrefixesByRegionRole(ipPoolSelector.Region, ipPoolSelector.Role)
	if err != nil {
		logger.Error(err, "unable to find prefixes", "region", ipPoolSelector.Region, "role", ipPoolSelector.Role)

		r.statusHandler.SetCondition(importCR, argorav1alpha1.NewReasonWithMessage(argorav1alpha1.ConditionReasonIPPoolImportFailed))
		if errUpdateStatus := r.statusHandler.UpdateToError(ctx, importCR, fmt.Errorf("unable to import prefix: %w", err)); errUpdateStatus != nil {
			return errUpdateStatus
		}

		return err
	}

	for _, prefix := range prefixes {
		logger.Info("reconciling prefix", "prefix", prefix.Prefix, "ID", prefix.ID)

		err = r.reconcileIPPool(ctx, ipPoolSelector, &prefix)
		if err != nil {
			logger.Error(err, "unable to reconcile ippool", "ippool", ipPoolSelector.NamePrefix, "prefix", prefix.Prefix, "ID", prefix.ID)

			r.statusHandler.SetCondition(importCR, argorav1alpha1.NewReasonWithMessage(argorav1alpha1.ConditionReasonIPPoolImportFailed))
			if errUpdateStatus := r.statusHandler.UpdateToError(ctx, importCR, fmt.Errorf("unable to reconcile prefix %s on ippool %s: %w", prefix.Prefix, ipPoolSelector.NamePrefix, err)); errUpdateStatus != nil {
				return errUpdateStatus
			}

			return err
		}
	}

	return nil
}

func (r *IPPoolImportReconciler) reconcileIPPool(ctx context.Context, ipPoolSelector *argorav1alpha1.IPPoolSelector, prefix *models.Prefix) error {
	logger := log.FromContext(ctx)
	logger.Info("reconciling IPPool", "prefix", prefix.Prefix, "ID", prefix.ID)

	ippool := &ipamv1alpha2.GlobalInClusterIPPool{}
	ippoolName, err := generateIPPoolName(ipPoolSelector, prefix)
	if err != nil {
		return fmt.Errorf("unable to generate ippool name for prefix %s: %w", prefix.Prefix, err)
	}

	net, gateway, mask, err := generateNetGatewayIP(prefix)
	if err != nil {
		return fmt.Errorf("unable to generate gateway IP for prefix %s: %w", prefix.Prefix, err)
	}

	err = r.k8sClient.Get(ctx, client.ObjectKey{Name: ippoolName}, ippool)
	if err != nil {
		logger.Info("IPPool not found, creating", "name", ippoolName)

		newIPPool := &ipamv1alpha2.GlobalInClusterIPPool{
			ObjectMeta: ctrl.ObjectMeta{
				Name: ippoolName,
			},
			Spec: ipamv1alpha2.InClusterIPPoolSpec{
				Addresses: []string{prefix.Prefix},
				Gateway:   gateway,
				Prefix:    mask,
			},
		}
		if ipPoolSelector.ExcludeMask != nil {
			if mask >= *ipPoolSelector.ExcludeMask {
				return fmt.Errorf("excludeMask (%d) must be longer than prefix mask (%d) for prefix %s", *ipPoolSelector.ExcludeMask, mask, prefix.Prefix)
			}
			newIPPool.Spec.ExcludedAddresses = []string{fmt.Sprintf("%s/%d", net, *ipPoolSelector.ExcludeMask)}
		}

		if ipPoolSelector.ExcludedAddresses != nil {
			newIPPool.Spec.ExcludedAddresses = append(newIPPool.Spec.ExcludedAddresses, ipPoolSelector.ExcludedAddresses...)
		}

		err = r.k8sClient.Create(ctx, newIPPool)
		if err != nil {
			logger.Error(err, "unable to create IPPool", "name", ippoolName)
			return err
		}

		logger.Info("IPPool created", "name", ippoolName)
		return nil
	}

	logger.Info("IPPool already exists, skipping", "name", ippoolName)
	return nil
}

// generateIPPoolName generates the name of the IPPool based on the given name prefix and prefix information.
func generateIPPoolName(ipPoolSelector *argorav1alpha1.IPPoolSelector, prefix *models.Prefix) (string, error) {
	if ipPoolSelector.NameOverride != "" {
		return ipPoolSelector.NameOverride, nil
	}
	if strings.Contains(prefix.Role.Slug, ComputeTransitPrefixRoleName) {
		azLetter := strings.TrimPrefix(prefix.Site.Slug, prefix.Site.Region.Slug)

		// if contains "compute1" it should be "%s-%s0-%s", if "compute2" it should be "%s-%s1-%s", and so on
		re := regexp.MustCompile(`(?i)compute(\d+)`)
		if matches := re.FindStringSubmatch(prefix.Vlan.Name); len(matches) > 1 {
			computeNum, err := strconv.Atoi(matches[1])
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("%s-%s%d-%s", ipPoolSelector.NamePrefix, azLetter, computeNum-1, prefix.Site.Region.Slug), nil
		}
	}
	return fmt.Sprintf("%s-%s", ipPoolSelector.NamePrefix, prefix.Site.Slug), nil
}

// generateNetGatewayIP generates the network address and gateway IP from the given prefix.
func generateNetGatewayIP(prefix *models.Prefix) (net, gw string, mask int, err error) {
	prefixParsed, err := netip.ParsePrefix(prefix.Prefix)
	if err != nil {
		return "", "", 0, err
	}
	return prefixParsed.Addr().String(), prefixParsed.Addr().Next().String(), prefixParsed.Bits(), nil
}
