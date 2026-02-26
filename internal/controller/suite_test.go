// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metalv1alpha1 "github.com/ironcore-dev/metal-operator/api/v1alpha1"
	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	ipamv1alpha2 "sigs.k8s.io/cluster-api-ipam-provider-in-cluster/api/v1alpha2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ipamv1 "sigs.k8s.io/cluster-api/api/ipam/v1beta2"

	argorav1alpha1 "github.com/sapcc/argora/api/v1alpha1"

	"github.com/sapcc/argora/internal/controller/mock"
	"github.com/sapcc/argora/internal/credentials"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	ctx           context.Context
	cancel        context.CancelFunc
	testEnv       *envtest.Environment
	cfg           *rest.Config
	k8sClient     client.Client
	k8sManager    interface{}
	mgrCtx        context.Context
	mgrCancel     context.CancelFunc
	testNamespace *corev1.Namespace
)

const (
	// reconciliation timing used by controllers in unit tests
	reconcileInterval        = 50 * time.Millisecond
	reconcileIntervalDefault = 1 * time.Minute

	// gomega defaults for integration tests
	pollingInterval      = 50 * time.Millisecond
	eventuallyTimeout    = 5 * time.Second
	consistentlyDuration = 1 * time.Second
)

func TestControllers(t *testing.T) {
	// configure gomega's default polling/timeout intervals so individual
	// specs don't need to repeat WithTimeout/WithPolling arguments.
	SetDefaultConsistentlyPollingInterval(pollingInterval)
	SetDefaultEventuallyPollingInterval(pollingInterval)
	SetDefaultEventuallyTimeout(eventuallyTimeout)
	SetDefaultConsistentlyDuration(consistentlyDuration)

	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.TODO())

	var err error
	err = argorav1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).ToNot(HaveOccurred())
	err = metalv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).ToNot(HaveOccurred())
	err = clusterv1.AddToScheme(scheme.Scheme)
	Expect(err).ToNot(HaveOccurred())
	err = ipamv1alpha2.AddToScheme(scheme.Scheme)
	Expect(err).ToNot(HaveOccurred())
	err = bmov1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).ToNot(HaveOccurred())
	err = ipamv1.AddToScheme(scheme.Scheme)
	Expect(err).ToNot(HaveOccurred())
	// +kubebuilder:scaffold:scheme

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "config", "crd", "bases"),
			filepath.Join("..", "..", "hack", "crd"),
		},
		ErrorIfCRDPathMissing: true,
	}

	// Retrieve the first found binary directory to allow running tests from IDEs
	if getFirstFoundEnvTestBinaryDir() != "" {
		testEnv.BinaryAssetsDirectory = getFirstFoundEnvTestBinaryDir()
	}

	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).ToNot(HaveOccurred())
	Expect(k8sClient).ToNot(BeNil())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	cancel()
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})

// getFirstFoundEnvTestBinaryDir locates the first binary in the specified path.
// ENVTEST-based tests depend on specific binaries, usually located in paths set by
// controller-runtime. When running tests directly (e.g., via an IDE) without using
// Makefile targets, the 'BinaryAssetsDirectory' must be explicitly configured.
//
// This function streamlines the process by finding the required binaries, similar to
// setting the 'KUBEBUILDER_ASSETS' environment variable. To ensure the binaries are
// properly set up, run 'make setup-envtest' beforehand.
func getFirstFoundEnvTestBinaryDir() string {
	basePath := filepath.Join("..", "..", "bin", "k8s")
	entries, err := os.ReadDir(basePath)
	if err != nil {
		logf.Log.Error(err, "Failed to read directory", "path", basePath)
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() {
			return filepath.Join(basePath, entry.Name())
		}
	}
	return ""
}

func SetupTest(fileReaderMock *mock.FileReaderMock, netBoxMock *mock.NetBoxMock) *corev1.Namespace {
	ns := &corev1.Namespace{}

	BeforeEach(func(ctx SpecContext) {
		var mgrCtx context.Context
		mgrCtx, cancel := context.WithCancel(context.Background())
		DeferCleanup(cancel)

		*ns = corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-ipupdate-",
			},
		}
		Expect(k8sClient.Create(ctx, ns)).To(Succeed(), "failed to create test namespace")
		DeferCleanup(k8sClient.Delete, ns)

		mgr, err := ctrl.NewManager(cfg, ctrl.Options{
			Scheme:     scheme.Scheme,
			Metrics:    metricsserver.Options{BindAddress: "0"},
			Controller: config.Controller{SkipNameValidation: ptr.To(true)},
		})
		Expect(err).ToNot(HaveOccurred())

		// Create credentials and register reconciler
		creds := credentials.NewDefaultCredentials(fileReaderMock)
		r := NewIPUpdateReconciler(mgr, creds, netBoxMock)
		if err := r.SetupWithManager(mgr); err != nil {
			Expect(err).ToNot(HaveOccurred())
		}

		go func() {
			defer GinkgoRecover()
			Expect(mgr.Start(mgrCtx)).To(Succeed())
		}()
	})

	return ns
}

func EnsureCleanState() {
	if testNamespace == nil {
		return
	}

	By("Cleaning up test resources from namespace: " + testNamespace.Name)

	ipList := &ipamv1.IPAddressList{}
	if err := k8sClient.List(ctx, ipList, &client.ListOptions{Namespace: testNamespace.Name}); err == nil {
		for _, ip := range ipList.Items {
			ip.Finalizers = nil
			k8sClient.Update(ctx, &ip)
			k8sClient.Delete(ctx, &ip)
		}
	}

	claimList := &ipamv1.IPAddressClaimList{}
	if err := k8sClient.List(ctx, claimList, &client.ListOptions{Namespace: testNamespace.Name}); err == nil {
		for _, claim := range claimList.Items {
			claim.Finalizers = nil
			k8sClient.Update(ctx, &claim)
			k8sClient.Delete(ctx, &claim)
		}
	}

	scList := &metalv1alpha1.ServerClaimList{}
	if err := k8sClient.List(ctx, scList, &client.ListOptions{Namespace: testNamespace.Name}); err == nil {
		for _, sc := range scList.Items {
			sc.Finalizers = nil
			k8sClient.Update(ctx, &sc)
			k8sClient.Delete(ctx, &sc)
		}
	}

	serverList := &metalv1alpha1.ServerList{}
	if err := k8sClient.List(ctx, serverList, &client.ListOptions{Namespace: testNamespace.Name}); err == nil {
		for _, server := range serverList.Items {
			server.Finalizers = nil
			k8sClient.Update(ctx, &server)
			k8sClient.Delete(ctx, &server)
		}
	}

	Expect(k8sClient.Delete(ctx, testNamespace)).Should(Succeed())
}

func createFakeClient(objects ...client.Object) client.Client {
	scheme := runtime.NewScheme()

	Expect(argorav1alpha1.AddToScheme(scheme)).Should(Succeed())
	Expect(metalv1alpha1.AddToScheme(scheme)).Should(Succeed())
	Expect(ipamv1alpha2.AddToScheme(scheme)).Should(Succeed())
	Expect(clusterv1.AddToScheme(scheme)).Should(Succeed())
	Expect(bmov1alpha1.AddToScheme(scheme)).Should(Succeed())
	Expect(ipamv1.AddToScheme(scheme)).Should(Succeed())

	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).WithStatusSubresource(objects...).Build()
}
