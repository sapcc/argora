// Copyright 2024 SAP SE
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"crypto/tls"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/runtime"

	metalv1alpha1 "github.com/ironcore-dev/metal-operator/api/v1alpha1"

	"github.com/ironcore-dev/controller-utils/cmdutils/switches"
	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	"github.com/sapcc/go-api-declarations/bininfo"
	"github.com/spf13/cobra"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/sapcc/argora/internal/controller"
	"github.com/sapcc/argora/internal/netbox"
)

var RootCmd = &cobra.Command{
	Use:     "argora",
	Short:   "Argora is a operator to discover cluster api clusters and look them up in netbox",
	RunE:    RunRootCmd,
	Version: bininfo.Version(),
}

var (
	scheme         = runtime.NewScheme()
	setupLog       = ctrl.Log.WithName("setup")
	netboxURL      string
	netboxToken    string
	bmcUser        string
	bmcPassword    string
	controllers    switches.Switches
	metricsAddr    string
	secureMetrics  bool
	enableHTTP2    bool
	ironCoreRoles  string
	ironCoreRegion string
)

const (
	// controllers
	metal3ClusterController  = "metal3Controller"
	ironCoreServerController = "ironCoreServerController"
)

func init() {
	RootCmd.PersistentFlags().StringVar(&netboxURL, "netbox-url", os.Getenv("NETBOX_URL"), "URL of the netbox instance")
	RootCmd.PersistentFlags().StringVar(&netboxToken, "netbox-token", os.Getenv("NETBOX_TOKEN"), "API token for netbox")
	RootCmd.PersistentFlags().StringVar(&bmcUser, "bmc-user", os.Getenv("BMC_USER"), "BMC user")
	RootCmd.PersistentFlags().StringVar(&bmcPassword, "bmc-password", os.Getenv("BMC_PASS"), "BMC password")
	RootCmd.PersistentFlags().StringVar(&ironCoreRoles, "ironcore-types", os.Getenv("IRONCORE_TYPES"), "Ironcore Cluster types")
	RootCmd.PersistentFlags().StringVar(&ironCoreRegion, "ironcore-region", os.Getenv("IRONCORE_REGION"), "Ironcore Regions")
	RootCmd.PersistentFlags().StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	RootCmd.PersistentFlags().BoolVar(&secureMetrics, "metrics-secure", false, "If set the metrics endpoint is served securely")
	RootCmd.PersistentFlags().BoolVar(&enableHTTP2, "enable-http2", false, "If set, HTTP/2 will be enabled for the metrics and webhook servers")

	controllers = *switches.New(
		metal3ClusterController,
		ironCoreServerController,
	)

	RootCmd.PersistentFlags().Var(&controllers, "controllers",
		fmt.Sprintf("Controllers to enable. All controllers: %v. Disabled-by-default controllers: %v",
			controllers.All(),
			controllers.DisabledByDefault(),
		),
	)
}

func RunRootCmd(cmd *cobra.Command, args []string) error {
	ctrl.SetLogger(zap.New())

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancelation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	tlsOpts := []func(*tls.Config){}
	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	mgr, err := ctrl.NewManager(config.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress:   metricsAddr,
			SecureServing: secureMetrics,
			TLSOpts:       tlsOpts,
		},
	})
	if err != nil {
		setupLog.Error(err, "unable to create manager")
		return err
	}
	err = corev1.AddToScheme(mgr.GetScheme())
	if err != nil {
		setupLog.Error(err, "unable to register core API scheme")
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
	err = metalv1alpha1.AddToScheme(mgr.GetScheme())
	if err != nil {
		setupLog.Error(err, "unable to register metal operator scheme")
		return err
	}
	nbc, err := netbox.NewNetboxClient(netboxURL, netboxToken)
	if err != nil {
		setupLog.Error(err, "unable to create netbox client")
		return err
	}

	if controllers.Enabled(metal3ClusterController) {
		if err = (&controller.Metal3ClusterController{
			Client:      mgr.GetClient(),
			Scheme:      mgr.GetScheme(),
			Nb:          nbc,
			BMCUser:     bmcUser,
			BMCPassword: bmcPassword,
		}).AddToManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "Metal3Cluster")
			os.Exit(1)
		}
	}

	if controllers.Enabled(ironCoreServerController) {
		if err = (&controller.IronCoreServerController{
			Client:         mgr.GetClient(),
			Scheme:         mgr.GetScheme(),
			Nb:             nbc,
			BMCUser:        bmcUser,
			BMCPassword:    bmcPassword,
			IronCoreRoles:  ironCoreRoles,
			IronCoreRegion: ironCoreRegion,
		}).AddToManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "IroncoreServer")
			os.Exit(1)
		}
	}

	setupLog.Info("starting manager")
	return mgr.Start(ctrl.SetupSignalHandler())
}
