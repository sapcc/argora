// Copyright 2025 SAP SE
// SPDX-License-Identifier: Apache-2.0

// Package periodic provides a periodic runner that sends events at a given interval.
package periodic

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

type Runner struct {
	client       client.Client
	interval     time.Duration
	eventChannel chan event.GenericEvent
}

type Option func(c *Runner) error

func NewRunner(opts ...Option) (*Runner, error) {
	r := &Runner{}
	for _, opt := range opts {
		if err := opt(r); err != nil {
			return nil, err
		}
	}

	return r, nil
}

func WithClient(c client.Client) Option {
	opt := func(r *Runner) error {
		r.client = c
		return nil
	}

	return opt
}

// WithInterval configures the [Runner] with the given interval.
func WithInterval(interval time.Duration) Option {
	opt := func(r *Runner) error {
		r.interval = interval
		return nil
	}

	return opt
}

func WithEventChannel(channel chan event.GenericEvent) Option {
	opt := func(r *Runner) error {
		r.eventChannel = channel
		return nil
	}

	return opt
}

func (r *Runner) Start(ctx context.Context) error {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	defer close(r.eventChannel)

	// send initial event to trigger the first reconciliation
	r.eventChannel <- event.GenericEvent{
		Object: &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "periodic",
				Namespace: "argora-system",
			},
		},
	}

	for {
		select {
		case <-ticker.C:
			r.eventChannel <- event.GenericEvent{
				Object: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "periodic",
						Namespace: "argora-system",
					},
				},
			}
		case <-ctx.Done():
			return nil
		}
	}
}
