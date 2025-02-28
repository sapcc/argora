package status

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"k8s.io/api/apps/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	types2 "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	argorav1alpha1 "github.com/sapcc/argora/api/v1alpha1"
)

func TestStatus(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Status Suite")
}

var _ = Describe("Status", func() {
	Describe("UpdateToReady", func() {
		It("should update Update CR status to ready", func() {
			// given
			cr := argorav1alpha1.Update{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
			}
			k8sClient := createFakeClient(&cr)
			handler := NewStatusHandler(k8sClient)

			// when
			err := handler.UpdateToReady(context.TODO(), &cr)

			// then
			Expect(err).ToNot(HaveOccurred())

			err = k8sClient.Get(context.TODO(), types2.NamespacedName{Name: "test", Namespace: "default"}, &cr)

			Expect(err).ToNot(HaveOccurred())
			Expect(cr.Status.State).To(Equal(argorav1alpha1.Ready))
			Expect(cr.Status.Conditions).To(BeNil())
		})

		It("should reset existing status description to empty on update to Ready", func() {
			// given
			cr := argorav1alpha1.Update{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
				Status: argorav1alpha1.UpdateStatus{
					State:       argorav1alpha1.Error,
					Description: "error description",
				},
			}

			k8sClient := createFakeClient(&cr)
			handler := NewStatusHandler(k8sClient)

			// when
			err := handler.UpdateToReady(context.TODO(), &cr)

			// then
			Expect(err).ToNot(HaveOccurred())

			Expect(k8sClient.Get(context.TODO(), types2.NamespacedName{Name: "test", Namespace: "default"}, &cr)).Should(Succeed())
			Expect(cr.Status.State).To(Equal(argorav1alpha1.Ready))
			Expect(cr.Status.Description).To(BeEmpty())
			Expect(cr.Status.Conditions).To(BeNil())
		})
	})

	Describe("UpdateToError", func() {
		It("should update Update CR status to error with description without condition", func() {
			// given
			cr := argorav1alpha1.Update{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
			}
			k8sClient := createFakeClient(&cr)
			handler := NewStatusHandler(k8sClient)

			// when
			err := handler.UpdateToError(context.TODO(), &cr, errors.New("some error"))

			// then
			Expect(err).ToNot(HaveOccurred())

			err = k8sClient.Get(context.TODO(), types2.NamespacedName{Name: "test", Namespace: "default"}, &cr)

			Expect(err).ToNot(HaveOccurred())
			Expect(cr.Status.State).To(Equal(argorav1alpha1.Error))
			Expect(cr.Status.Description).To(Equal("some error"))
			Expect(cr.Status.Conditions).To(BeNil())
		})
	})

	Describe("SetCondition", func() {
		It("should set Update CR status conditions", func() {
			// given
			cr := argorav1alpha1.Update{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
			}
			k8sClient := createFakeClient(&cr)
			handler := NewStatusHandler(k8sClient)

			// when
			handler.SetCondition(&cr, argorav1alpha1.NewReasonWithMessage(argorav1alpha1.ConditionReasonUpdateSucceeded))

			// then
			Expect(cr.Status.Conditions).ToNot(BeNil())
			Expect((*cr.Status.Conditions)).To(HaveLen(1))
			Expect((*cr.Status.Conditions)[0].Type).To(Equal(string(argorav1alpha1.ConditionTypeReady)))
			Expect((*cr.Status.Conditions)[0].Reason).To(Equal(string(argorav1alpha1.ConditionReasonUpdateSucceeded)))
			Expect((*cr.Status.Conditions)[0].Status).To(Equal(metav1.ConditionTrue))
		})
	})
})

func createFakeClient(objects ...client.Object) client.Client {
	return fake.NewClientBuilder().WithScheme(getTestScheme()).WithObjects(objects...).WithStatusSubresource(objects...).Build()
}

func getTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	Expect(argorav1alpha1.AddToScheme(scheme)).Should(Succeed())
	Expect(v1beta1.AddToScheme(scheme)).Should(Succeed())

	return scheme
}
