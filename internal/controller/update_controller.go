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
	"time"

	argorav1alpha1 "github.com/sapcc/argora/api/v1alpha1"
	"github.com/sapcc/argora/internal/config"
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
	client.Client
	Scheme            *runtime.Scheme
	Config            *config.Config
	ReconcileInterval time.Duration
}

func NewUpdateReconciler(mgr ctrl.Manager, cfg *config.Config, reconcileInterval time.Duration) *UpdateReconciler {
	return &UpdateReconciler{
		Client:            mgr.GetClient(),
		Scheme:            mgr.GetScheme(),
		Config:            cfg,
		ReconcileInterval: reconcileInterval,
	}
}

// +kubebuilder:rbac:groups=argora.cloud.sap,resources=updates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=argora.cloud.sap,resources=updates/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=argora.cloud.sap,resources=updates/finalizers,verbs=update

// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.0/pkg/reconcile
func (r *UpdateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// TODO(user): your logic here

	return ctrl.Result{}, nil
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
