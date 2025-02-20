package periodic

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
	})
})
