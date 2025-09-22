// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"os"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.

	"github.com/sapcc/go-api-declarations/bininfo"
	"go.uber.org/zap/zapcore"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	argorav1alpha1 "github.com/sapcc/argora/api/v1alpha1"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	ironcorev1alpha1 "github.com/ironcore-dev/metal-operator/api/v1alpha1"
	metal3v1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/sapcc/argora/internal/controller"
	"github.com/sapcc/argora/internal/credentials"
	"github.com/sapcc/argora/internal/netbox"
	"github.com/sapcc/argora/internal/status"
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

type FlagVariables struct {
	metricsAddr             string
	probeAddr               string
	leaderElectionNamespace string
	netboxURL               string

	enableLeaderElection bool
	secureMetrics        bool
	enableHTTP2          bool
	enableIronCore       bool

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
	flagVar := getFlagVariables()
	opts := zap.Options{
		Development: true,
		Level:       zapcore.InfoLevel,
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
		Scheme:                  scheme,
		Metrics:                 metricsServerOptions,
		HealthProbeBindAddress:  flagVar.probeAddr,
		LeaderElection:          flagVar.enableLeaderElection,
		LeaderElectionID:        "849e53e1.cloud.sap",
		LeaderElectionNamespace: flagVar.leaderElectionNamespace,
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

	creds := credentials.NewDefaultCredentials(&credentials.Reader{})
	setupLog.Info("argora", "version", bininfo.Version())

	if flagVar.enableIronCore {
		if err = controller.NewIronCoreReconciler(mgr, creds, status.NewClusterImportStatusHandler(mgr.GetClient()), netbox.NewNetbox(flagVar.netboxURL), flagVar.reconcileInterval).SetupWithManager(mgr, rateLimiter); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "ironcore")
			os.Exit(1)
		}
	} else {
		capiCRDExists, err := capiCRDExists(mgr.GetClient())
		if err != nil {
			setupLog.Error(err, "unable to check if CAPI CRD exists")
			os.Exit(1)
		}

		if !capiCRDExists {
			setupLog.Error(errors.New("CAPI CRD not found"), "unable to start Metal3 controller as CAPI CRD is missing")
			os.Exit(1)
		}

		if err = controller.NewMetal3Reconciler(mgr.GetClient(), mgr.GetScheme(), creds, netbox.NewNetbox(flagVar.netboxURL), flagVar.reconcileInterval).SetupWithManager(mgr, rateLimiter); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "metal3")
			os.Exit(1)
		}
	}

	if err = controller.NewUpdateReconciler(mgr, creds, status.NewUpdateStatusHandler(mgr.GetClient()), netbox.NewNetbox(flagVar.netboxURL), flagVar.reconcileInterval).SetupWithManager(mgr, rateLimiter); err != nil {
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

func getFlagVariables() *FlagVariables {
	flagVariables := new(FlagVariables)

	flag.StringVar(&flagVariables.metricsAddr, "metrics-bind-address", "0", "The address the metrics endpoint binds to. Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
	flag.StringVar(&flagVariables.probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.StringVar(&flagVariables.leaderElectionNamespace, "leader-elect-ns", "kube-system", "The namespace in which the leader election resource will be created. This is only used if --leader-elect is set to true. Defaults to kube-system.")
	flag.StringVar(&flagVariables.netboxURL, "netbox-url", "https://netbox-test.global.cloud.sap", "The URL of the NetBox instance to connect to. If not set, the default value will be used.")

	flag.BoolVar(&flagVariables.enableLeaderElection, "leader-elect", false, "Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&flagVariables.secureMetrics, "metrics-secure", true, "If true (default), the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	flag.BoolVar(&flagVariables.enableHTTP2, "enable-http2", false, "If true (default is false), HTTP/2 will be enabled for the metrics and webhook servers")
	flag.BoolVar(&flagVariables.enableIronCore, "enable-ironcore", true, "If true (default), the IronCore controller will be enabled, otherwise Metal3 controller will be enabled.")

	flag.IntVar(&flagVariables.rateLimiterBurst, "rate-limiter-burst", rateLimiterBurstDefault, "Indicates the burst value for the bucket rate limiter.")
	flag.IntVar(&flagVariables.rateLimiterFrequency, "rate-limiter-frequency", rateLimiterFrequencyDefault, "Indicates the bucket rate limiter frequency, signifying no. of events per second.")
	flag.DurationVar(&flagVariables.failureBaseDelay, "failure-base-delay", failureBaseDelayDefault, "Indicates the failure base delay for rate limiter.")
	flag.DurationVar(&flagVariables.failureMaxDelay, "failure-max-delay", failureMaxDelayDefault, "Indicates the failure max delay.")
	flag.DurationVar(&flagVariables.reconcileInterval, "reconcile-interval", reconcileIntervalDefault, "Indicates the time based reconcile interval.")

	return flagVariables
}

// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get

func capiCRDExists(k8sClient client.Client) (bool, error) {
	ctx, cancelFunc := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelFunc()
	var u unstructured.Unstructured
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apiextensions.k8s.io",
		Version: "v1",
		Kind:    "CustomResourceDefinition",
	})
	err := k8sClient.Get(ctx, types.NamespacedName{Name: "clusters.cluster.x-k8s.io"}, &u)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return false, nil
		} else {
			return false, err
		}
	}
	return true, nil
}
