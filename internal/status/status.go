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

type ClusterImportStatus interface {
	UpdateToReady(ctx context.Context, clusterImportCR *argorav1alpha1.ClusterImport) error
	UpdateToError(ctx context.Context, clusterImportCR *argorav1alpha1.ClusterImport, err error) error

	SetCondition(clusterImportCR *argorav1alpha1.ClusterImport, reason argorav1alpha1.ReasonWithMessage)
}

type IPPoolImportStatus interface {
	UpdateToReady(ctx context.Context, ipPoolImportCR *argorav1alpha1.IPPoolImport) error
	UpdateToError(ctx context.Context, ipPoolImportCR *argorav1alpha1.IPPoolImport, err error) error

	SetCondition(ipPoolImportCR *argorav1alpha1.IPPoolImport, reason argorav1alpha1.ReasonWithMessage)
}

func NewUpdateStatusHandler(k8sClient client.Client) UpdateStatus {
	return UpdateStatusHandler{
		k8sClient: k8sClient,
	}
}

func NewClusterImportStatusHandler(k8sClient client.Client) ClusterImportStatus {
	return ClusterImportStatusHandler{
		k8sClient: k8sClient,
	}
}

func NewIPPoolImportStatusHandler(k8sClient client.Client) IPPoolImportStatus {
	return IPPoolImportStatusHandler{
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

type ClusterImportStatusHandler struct {
	k8sClient client.Client
}

func (d ClusterImportStatusHandler) update(ctx context.Context, clusterImportCR *argorav1alpha1.ClusterImport) error {
	newStatus := clusterImportCR.Status
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if getErr := d.k8sClient.Get(ctx, client.ObjectKeyFromObject(clusterImportCR), clusterImportCR); getErr != nil {
			return getErr
		}
		clusterImportCR.Status = newStatus
		if updateErr := d.k8sClient.Status().Update(ctx, clusterImportCR); updateErr != nil {
			return updateErr
		}
		return nil
	})
}

func (d ClusterImportStatusHandler) UpdateToReady(ctx context.Context, clusterImportCR *argorav1alpha1.ClusterImport) error {
	clusterImportCR.Status.State = argorav1alpha1.Ready
	clusterImportCR.Status.Description = ""
	return d.update(ctx, clusterImportCR)
}

func (d ClusterImportStatusHandler) UpdateToError(ctx context.Context, clusterImportCR *argorav1alpha1.ClusterImport, err error) error {
	clusterImportCR.Status.State = argorav1alpha1.Error
	clusterImportCR.Status.Description = err.Error()
	return d.update(ctx, clusterImportCR)
}

func (d ClusterImportStatusHandler) SetCondition(clusterImportCR *argorav1alpha1.ClusterImport, reason argorav1alpha1.ReasonWithMessage) {
	if clusterImportCR.Status.Conditions == nil {
		clusterImportCR.Status.Conditions = &[]metav1.Condition{}
	}
	setCondition(clusterImportCR.Status.Conditions, reason)
}

type IPPoolImportStatusHandler struct {
	k8sClient client.Client
}

func (d IPPoolImportStatusHandler) update(ctx context.Context, ipPoolImportCR *argorav1alpha1.IPPoolImport) error {
	newStatus := ipPoolImportCR.Status
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if getErr := d.k8sClient.Get(ctx, client.ObjectKeyFromObject(ipPoolImportCR), ipPoolImportCR); getErr != nil {
			return getErr
		}
		ipPoolImportCR.Status = newStatus
		if updateErr := d.k8sClient.Status().Update(ctx, ipPoolImportCR); updateErr != nil {
			return updateErr
		}
		return nil
	})
}

func (d IPPoolImportStatusHandler) UpdateToReady(ctx context.Context, ipPoolImportCR *argorav1alpha1.IPPoolImport) error {
	ipPoolImportCR.Status.State = argorav1alpha1.Ready
	ipPoolImportCR.Status.Description = ""
	return d.update(ctx, ipPoolImportCR)
}

func (d IPPoolImportStatusHandler) UpdateToError(ctx context.Context, ipPoolImportCR *argorav1alpha1.IPPoolImport, err error) error {
	ipPoolImportCR.Status.State = argorav1alpha1.Error
	ipPoolImportCR.Status.Description = err.Error()
	return d.update(ctx, ipPoolImportCR)
}

func (d IPPoolImportStatusHandler) SetCondition(ipPoolImportCR *argorav1alpha1.IPPoolImport, reason argorav1alpha1.ReasonWithMessage) {
	if ipPoolImportCR.Status.Conditions == nil {
		ipPoolImportCR.Status.Conditions = &[]metav1.Condition{}
	}
	setCondition(ipPoolImportCR.Status.Conditions, reason)
}

func setCondition(conditions *[]metav1.Condition, reason argorav1alpha1.ReasonWithMessage) {
	condition := argorav1alpha1.ConditionFromReason(reason)
	if condition != nil {
		meta.SetStatusCondition(conditions, *condition)
	} else {
		ctrl.Log.Error(errors.New("condition not found"), "unable to find condition from reason", "reason", reason)
	}
}
