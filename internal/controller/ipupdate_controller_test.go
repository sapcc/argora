// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/sapcc/argora/internal/controller/mock"
	"github.com/sapcc/argora/internal/credentials"
)

var _ = Describe("IP Update Controller", func() {
	const resourceName = "test-resource"
	const resourceNamespace = "default"

	fileReaderMock := &mock.FileReaderMock{
		FileContent: make(map[string]string),
		ReturnError: false,
	}

	fileReaderMock.FileContent["/etc/credentials/credentials.json"] = `{
		"bmcUser": "user",
		"bmcPassword": "password",
		"netboxToken": "token"
	}`

	Context("Reconcile", func() {
		ctx := context.Background()

		typeNamespacedUpdateName := types.NamespacedName{
			Name:      resourceName,
			Namespace: resourceNamespace,
		}

		prepareNetboxMock := func() *mock.NetBoxMock {
			netBoxMock := &mock.NetBoxMock{
				ReturnError:        false,
				VirtualizationMock: &mock.VirtualizationMock{},
				DCIMMock:           &mock.DCIMMock{},
				IPAMMock:           &mock.IPAMMock{},
				ExtrasMock:         &mock.ExtrasMock{},
			}

			return netBoxMock
		}

		BeforeEach(func() {
		})

		AfterEach(func() {
		})

		It("should successfully reconcile the CR", func() {
			// given
			netBoxMock := prepareNetboxMock()
			controllerReconciler := createIPUpdateReconciler(netBoxMock, fileReaderMock)

			// when
			By("reconciling IPAM IPAddress CR")
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedUpdateName})

			// then
			Expect(err).ToNot(HaveOccurred())
		})
	})
})

func createIPUpdateReconciler(netBoxMock *mock.NetBoxMock, fileReaderMock credentials.FileReader) *IPUpdateReconciler {
	return &IPUpdateReconciler{
		k8sClient:   k8sClient,
		scheme:      k8sClient.Scheme(),
		netBox:      netBoxMock,
		credentials: credentials.NewDefaultCredentials(fileReaderMock),
	}
}
