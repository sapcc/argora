// Copyright 2025 SAP SE
// SPDX-License-Identifier: Apache-2.0

package periodic

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Periodic Suite")
}

var _ = Describe("Runner", func() {
	var (
		eventChannel chan event.GenericEvent
		ctx          context.Context
		cancel       context.CancelFunc
	)

	BeforeEach(func() {
		eventChannel = make(chan event.GenericEvent, 1)
		ctx, cancel = context.WithCancel(context.Background())
	})

	AfterEach(func() {
		cancel()
	})

	Describe("NewRunner", func() {
		It("should create a new Runner with the provided options", func() {
			client := fake.NewClientBuilder().Build()
			interval := 1 * time.Second

			r, err := NewRunner(
				WithClient(client),
				WithInterval(interval),
				WithEventChannel(eventChannel),
			)

			Expect(err).NotTo(HaveOccurred())
			Expect(r).NotTo(BeNil())
			Expect(r.client).To(Equal(client))
			Expect(r.interval).To(Equal(interval))
			Expect(r.eventChannel).To(Equal(eventChannel))
		})
	})

	Describe("Start", func() {
		It("should send events at the specified interval", func() {
			interval := 100 * time.Millisecond

			runner, err := NewRunner(
				WithInterval(interval),
				WithEventChannel(eventChannel),
			)
			Expect(err).NotTo(HaveOccurred())

			go func() {
				err := runner.Start(ctx)
				Expect(err).NotTo(HaveOccurred())
			}()

			Eventually(eventChannel).Should(Receive())
		})

		It("should stop sending events when context is done", func() {
			interval := 100 * time.Millisecond

			runner, err := NewRunner(
				WithInterval(interval),
				WithEventChannel(eventChannel),
			)
			Expect(err).NotTo(HaveOccurred())

			go func() {
				err := runner.Start(ctx)
				Expect(err).NotTo(HaveOccurred())
			}()

			Eventually(eventChannel).Should(Receive())

			cancel()

			Consistently(eventChannel).ShouldNot(Receive())
		})

		It("should send a pod event with correct metadata", func() {
			interval := 100 * time.Millisecond

			runner, err := NewRunner(
				WithInterval(interval),
				WithEventChannel(eventChannel),
			)
			Expect(err).NotTo(HaveOccurred())

			go func() {
				err := runner.Start(ctx)
				Expect(err).NotTo(HaveOccurred())
			}()

			var receivedEvent event.GenericEvent
			Eventually(eventChannel).Should(Receive(&receivedEvent))

			pod, ok := receivedEvent.Object.(*corev1.Pod)
			Expect(ok).To(BeTrue())
			Expect(pod.Name).To(Equal("periodic"))
			Expect(pod.Namespace).To(Equal("argora-system"))
		})
	})
})
