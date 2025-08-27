// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

// Package status provides helper functionality for updating status of a k8s CR
package status

import (
	"context"
	"errors"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argorav1alpha1 "github.com/sapcc/argora/api/v1alpha1"
)

type Status interface {
	UpdateToReady(ctx context.Context, updateCR *argorav1alpha1.Update) error
	UpdateToError(ctx context.Context, updateCR *argorav1alpha1.Update, err error) error
	SetCondition(updateCR *argorav1alpha1.Update, reason argorav1alpha1.ReasonWithMessage)
}

func NewStatusHandler(k8sClient client.Client) Status {
	return StatusHandler{
		k8sClient: k8sClient,
	}
}

type StatusHandler struct {
	k8sClient client.Client
}

func (d StatusHandler) update(ctx context.Context, updateCR *argorav1alpha1.Update) error {
	newStatus := updateCR.Status
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if getErr := d.k8sClient.Get(ctx, client.ObjectKeyFromObject(updateCR), updateCR); getErr != nil {
			return getErr
		}
		updateCR.Status = newStatus
		if updateErr := d.k8sClient.Status().Update(ctx, updateCR); updateErr != nil {
			return updateErr
		}
		return nil
	})
}

func (d StatusHandler) UpdateToReady(ctx context.Context, updateCR *argorav1alpha1.Update) error {
	updateCR.Status.State = argorav1alpha1.Ready
	updateCR.Status.Description = ""
	return d.update(ctx, updateCR)
}

func (d StatusHandler) UpdateToError(ctx context.Context, updateCR *argorav1alpha1.Update, err error) error {
	updateCR.Status.State = argorav1alpha1.Error
	updateCR.Status.Description = err.Error()
	return d.update(ctx, updateCR)
}

func (d StatusHandler) SetCondition(updateCR *argorav1alpha1.Update, reason argorav1alpha1.ReasonWithMessage) {
	if updateCR.Status.Conditions == nil {
		updateCR.Status.Conditions = &[]metav1.Condition{}
	}
	condition := argorav1alpha1.ConditionFromReason(reason)
	if condition != nil {
		meta.SetStatusCondition(updateCR.Status.Conditions, *condition)
	} else {
		ctrl.Log.Error(errors.New("condition not found"), "Unable to find condition from reason", "reason", reason)
	}
}
