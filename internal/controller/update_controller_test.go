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
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	argorav1alpha1 "github.com/sapcc/argora/api/v1alpha1"
	"github.com/sapcc/argora/internal/config"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type FileReaderMock struct {
	fileContent map[string]string
	returnError bool
}

func (f *FileReaderMock) ReadFile(fileName string) ([]byte, error) {
	if f.returnError {
		return nil, errors.New("error")
	}
	return []byte(f.fileContent[fileName]), nil
}

var _ = Describe("Update Controller", func() {
	var fileReaderMock *FileReaderMock

	Context("When reconciling a Update custom resource", func() {
		const resourceName = "test-resource"
		const resourceNamespace = "default"

		fileReaderMock = &FileReaderMock{
			fileContent: make(map[string]string),
			returnError: false,
		}
		configJson := `{
			"ironCoreRoles": "role1",
			"ironCoreRegion": "region1",
			"serverController": "ironcore"
		}`
		credentialsJson := `{
			"netboxUrl": "aHR0cDovL25ldGJveA==",
			"netboxToken": "dG9rZW4=",
			"bmcUser": "dXNlcg==",
			"bmcPassword": "cGFzc3dvcmQ="
		}`

		fileReaderMock.fileContent["/etc/config/config.json"] = configJson
		fileReaderMock.fileContent["/etc/credentials/credentials.json"] = credentialsJson

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: resourceNamespace,
		}
		update := &argorav1alpha1.Update{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind Update")
			err := k8sClient.Get(ctx, typeNamespacedName, update)
			if err != nil && apierrors.IsNotFound(err) {
				resource := &argorav1alpha1.Update{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: resourceNamespace,
					},
					Spec: argorav1alpha1.UpdateSpec{
						// Clusters: []*argorav1alpha1.Clusters{
						// 	{
						// 		Name:   "cluster1",
						// 		Region: "region1",
						// 		Type:   "type1",
						// 	},
						// },
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			err := k8sClient.Get(ctx, typeNamespacedName, update)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Update")
			Expect(k8sClient.Delete(ctx, update)).To(Succeed())
		})

		It("should successfully reconcile the CR", func() {
			// given
			By("Reconciling the created resource")
			controllerReconciler := &UpdateReconciler{
				k8sClient:         k8sClient,
				scheme:            k8sClient.Scheme(),
				cfg:               config.NewDefaultConfiguration(k8sClient, fileReaderMock),
				reconcileInterval: reconcileInterval,
			}

			// when
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})

			// then
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Get(ctx, typeNamespacedName, update)
			Expect(err).NotTo(HaveOccurred())
			Expect(update.Status.State).To(Equal(argorav1alpha1.Ready))
			Expect(update.Status.Description).To(BeEmpty())
			Expect(update.Status.Conditions).ToNot(BeNil())
			Expect((*update.Status.Conditions)).To(HaveLen(1))
			Expect((*update.Status.Conditions)[0].Type).To(Equal(string(argorav1alpha1.ConditionTypeReady)))
			Expect((*update.Status.Conditions)[0].Status).To(Equal(metav1.ConditionTrue))
			Expect((*update.Status.Conditions)[0].Reason).To(Equal(string(argorav1alpha1.ConditionReasonUpdateSucceeded)))
		})
	})
})
