package controller

import (
	"context"
	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type ClusterController struct {
}

// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metal3.io,resources=baremetalhosts,verbs=get;list;watch;create;update;patch;delete
// Reconcile looks up a cluster in netbox and creates baremetal hosts for it
func (c *ClusterController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

func (c *ClusterController) AddToManager(mgr manager.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).For(&clusterv1.Cluster{}).Owns(&bmov1alpha1.BareMetalHost{}).Complete(c)
}
