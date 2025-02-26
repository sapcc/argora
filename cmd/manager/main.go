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

package main

import (
	"crypto/tls"
	"flag"
	"os"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.

	"github.com/sapcc/go-api-declarations/bininfo"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	argorav1alpha1 "github.com/sapcc/argora/api/v1alpha1"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	ironcorev1alpha1 "github.com/ironcore-dev/metal-operator/api/v1alpha1"
	metal3v1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/sapcc/argora/internal/config"
	"github.com/sapcc/argora/internal/controller"
	"github.com/sapcc/argora/internal/netbox"
	// +kubebuilder:scaffold:imports
)

const (
	rateLimiterBurstDefault     = 200
	rateLimiterFrequencyDefault = 30
	failureBaseDelayDefault     = 1 * time.Second
	failureMaxDelayDefault      = 1000 * time.Second
	reconcileIntervalDefault    = 5 * time.Minute
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

type FlagVar struct {
	metricsAddr          string
	probeAddr            string
	enableLeaderElection bool
	secureMetrics        bool
	enableHTTP2          bool

	failureBaseDelay     time.Duration
	failureMaxDelay      time.Duration
	rateLimiterFrequency int
	rateLimiterBurst     int
	reconcileInterval    time.Duration
}

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(argorav1alpha1.AddToScheme(scheme))

	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(clusterv1.AddToScheme(scheme))
	utilruntime.Must(metal3v1alpha1.AddToScheme(scheme))
	utilruntime.Must(ironcorev1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

// nolint:gocyclo
func main() {
	setupLog.Info("argora", "version", bininfo.Version())

	flagVar := createFlagVar()
	opts := zap.Options{
		Development: true,
	}

	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	rateLimiter := controller.RateLimiter{
		Burst:           flagVar.rateLimiterBurst,
		Frequency:       flagVar.rateLimiterFrequency,
		BaseDelay:       flagVar.failureBaseDelay,
		FailureMaxDelay: flagVar.failureMaxDelay,
	}

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	var tlsOpts []func(*tls.Config)

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	if !flagVar.enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	// Metrics endpoint is enabled in 'config/default/kustomization.yaml'. The Metrics options configure the server.
	// More info:
	// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.0/pkg/metrics/server
	// - https://book.kubebuilder.io/reference/metrics.html
	metricsServerOptions := metricsserver.Options{
		BindAddress:   flagVar.metricsAddr,
		SecureServing: flagVar.secureMetrics,
		TLSOpts:       tlsOpts,
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsServerOptions,
		HealthProbeBindAddress: flagVar.probeAddr,
		LeaderElection:         flagVar.enableLeaderElection,
		LeaderElectionID:       "849e53e1.cloud.sap",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	cfg := config.NewDefaultConfiguration(mgr.GetClient(), &config.ConfigReader{})

	if err = controller.NewMetal3Reconciler(mgr.GetClient(), mgr.GetScheme(), cfg, flagVar.reconcileInterval).SetupWithManager(mgr, rateLimiter); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "metal3")
		os.Exit(1)
	}

	if err = controller.NewIronCoreReconciler(mgr.GetClient(), mgr.GetScheme(), cfg, flagVar.reconcileInterval).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ironCore")
		os.Exit(1)
	}

	if err = controller.NewUpdateReconciler(mgr, cfg, netbox.NewNetbox(), flagVar.reconcileInterval).SetupWithManager(mgr, rateLimiter); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "update")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func createFlagVar() *FlagVar {
	flagVar := new(FlagVar)

	flag.StringVar(&flagVar.metricsAddr, "metrics-bind-address", "0", "The address the metrics endpoint binds to. Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
	flag.StringVar(&flagVar.probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&flagVar.enableLeaderElection, "leader-elect", false, "Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&flagVar.secureMetrics, "metrics-secure", true, "If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	flag.BoolVar(&flagVar.enableHTTP2, "enable-http2", false, "If set, HTTP/2 will be enabled for the metrics and webhook servers")

	flag.IntVar(&flagVar.rateLimiterBurst, "rate-limiter-burst", rateLimiterBurstDefault, "Indicates the burst value for the bucket rate limiter.")
	flag.IntVar(&flagVar.rateLimiterFrequency, "rate-limiter-frequency", rateLimiterFrequencyDefault, "Indicates the bucket rate limiter frequency, signifying no. of events per second.")
	flag.DurationVar(&flagVar.failureBaseDelay, "failure-base-delay", failureBaseDelayDefault, "Indicates the failure base delay for rate limiter.")
	flag.DurationVar(&flagVar.failureMaxDelay, "failure-max-delay", failureMaxDelayDefault, "Indicates the failure max delay.")
	flag.DurationVar(&flagVar.reconcileInterval, "reconcile-interval", reconcileIntervalDefault, "Indicates the time based reconcile interval.")

	return flagVar
}
