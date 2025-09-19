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

type UpdateStatus interface {
	UpdateToReady(ctx context.Context, updateCR *argorav1alpha1.Update) error
	UpdateToError(ctx context.Context, updateCR *argorav1alpha1.Update, err error) error

	SetCondition(updateCR *argorav1alpha1.Update, reason argorav1alpha1.ReasonWithMessage)
}

type IronCoreStatus interface {
	UpdateToReady(ctx context.Context, ironCoreCR *argorav1alpha1.IronCore) error
	UpdateToError(ctx context.Context, ironCoreCR *argorav1alpha1.IronCore, err error) error

	SetCondition(ironCoreCR *argorav1alpha1.IronCore, reason argorav1alpha1.ReasonWithMessage)
}

func NewUpdateStatusHandler(k8sClient client.Client) UpdateStatus {
	return UpdateStatusHandler{
		k8sClient: k8sClient,
	}
}

func NewIronCoreStatusHandler(k8sClient client.Client) IronCoreStatus {
	return IronCoreStatusHandler{
		k8sClient: k8sClient,
	}
}

type UpdateStatusHandler struct {
	k8sClient client.Client
}

func (d UpdateStatusHandler) update(ctx context.Context, updateCR *argorav1alpha1.Update) error {
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

func (d UpdateStatusHandler) UpdateToReady(ctx context.Context, updateCR *argorav1alpha1.Update) error {
	updateCR.Status.State = argorav1alpha1.Ready
	updateCR.Status.Description = ""
	return d.update(ctx, updateCR)
}

func (d UpdateStatusHandler) UpdateToError(ctx context.Context, updateCR *argorav1alpha1.Update, err error) error {
	updateCR.Status.State = argorav1alpha1.Error
	updateCR.Status.Description = err.Error()
	return d.update(ctx, updateCR)
}

func (d UpdateStatusHandler) SetCondition(updateCR *argorav1alpha1.Update, reason argorav1alpha1.ReasonWithMessage) {
	if updateCR.Status.Conditions == nil {
		updateCR.Status.Conditions = &[]metav1.Condition{}
	}
	setCondition(updateCR.Status.Conditions, reason)
}

type IronCoreStatusHandler struct {
	k8sClient client.Client
}

func (d IronCoreStatusHandler) update(ctx context.Context, ironCoreCR *argorav1alpha1.IronCore) error {
	newStatus := ironCoreCR.Status
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if getErr := d.k8sClient.Get(ctx, client.ObjectKeyFromObject(ironCoreCR), ironCoreCR); getErr != nil {
			return getErr
		}
		ironCoreCR.Status = newStatus
		if updateErr := d.k8sClient.Status().Update(ctx, ironCoreCR); updateErr != nil {
			return updateErr
		}
		return nil
	})
}

func (d IronCoreStatusHandler) UpdateToReady(ctx context.Context, ironCoreCR *argorav1alpha1.IronCore) error {
	ironCoreCR.Status.State = argorav1alpha1.Ready
	ironCoreCR.Status.Description = ""
	return d.update(ctx, ironCoreCR)
}

func (d IronCoreStatusHandler) UpdateToError(ctx context.Context, ironCoreCR *argorav1alpha1.IronCore, err error) error {
	ironCoreCR.Status.State = argorav1alpha1.Error
	ironCoreCR.Status.Description = err.Error()
	return d.update(ctx, ironCoreCR)
}

func (d IronCoreStatusHandler) SetCondition(ironCoreCR *argorav1alpha1.IronCore, reason argorav1alpha1.ReasonWithMessage) {
	if ironCoreCR.Status.Conditions == nil {
		ironCoreCR.Status.Conditions = &[]metav1.Condition{}
	}
	setCondition(ironCoreCR.Status.Conditions, reason)
}

func setCondition(conditions *[]metav1.Condition, reason argorav1alpha1.ReasonWithMessage) {
	condition := argorav1alpha1.ConditionFromReason(reason)
	if condition != nil {
		meta.SetStatusCondition(conditions, *condition)
	} else {
		ctrl.Log.Error(errors.New("condition not found"), "unable to find condition from reason", "reason", reason)
	}
}
