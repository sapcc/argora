/*
 * Copyright (c) 2024. Lorem ipsum dolor sit amet, consectetur adipiscing elit.
 * Morbi non lorem porttitor neque feugiat blandit. Ut vitae ipsum eget quam lacinia accumsan.
 * Etiam sed turpis ac ipsum condimentum fringilla. Maecenas magna.
 * Proin dapibus sapien vel ante. Aliquam erat volutpat. Pellentesque sagittis ligula eget metus.
 * Vestibulum commodo. Ut rhoncus gravida arcu.
 */

package cmd

import (
	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	"github.com/sapcc/argora/internal/controller"
	"github.com/sapcc/argora/internal/netbox"
	"github.com/sapcc/go-api-declarations/bininfo"
	"github.com/spf13/cobra"
	"os"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var RootCmd = &cobra.Command{
	Use:     "argora",
	Short:   "Argora is a operator to discover cluster api clusters and look them up in netbox",
	RunE:    RunRootCmd,
	Version: bininfo.Version(),
}

var (
	setupLog    = ctrl.Log.WithName("setup")
	netboxUrl   string
	netboxToken string
)

func init() {
	RootCmd.PersistentFlags().StringVar(&netboxUrl, "netbox-url", "https://netbox.global.cloud.sap", "URL of the netbox instance")
	RootCmd.PersistentFlags().StringVar(&netboxToken, "netbox-token", os.Getenv("NETBOX_TOKEN"), "API token for netbox")
}

func RunRootCmd(cmd *cobra.Command, args []string) error {
	ctrl.SetLogger(zap.New())
	mgr, err := ctrl.NewManager(config.GetConfigOrDie(), ctrl.Options{})
	if err != nil {
		setupLog.Error(err, "unable to create manager")
		return err
	}
	err = clusterv1.AddToScheme(mgr.GetScheme())
	if err != nil {
		setupLog.Error(err, "unable to register cluster api scheme")
		return err
	}
	err = bmov1alpha1.AddToScheme(mgr.GetScheme())
	if err != nil {
		setupLog.Error(err, "unable to register baremetal operator scheme")
		return err
	}
	nbc, err := netbox.NewNetboxClient(netboxUrl, netboxToken)
	if err != nil {
		setupLog.Error(err, "unable to create netbox client")
		return err
	}
	clusterController := &controller.ClusterController{
		Client: mgr.GetClient(),
		Nb:     nbc,
	}
	err = clusterController.AddToManager(mgr)
	if err != nil {
		setupLog.Error(err, "unable to add cluster controller to manager")
		return err
	}

	setupLog.Info("starting manager")
	return mgr.Start(ctrl.SetupSignalHandler())
}
