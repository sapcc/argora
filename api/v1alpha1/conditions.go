// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type State string
type ConditionType string
type ConditionReason string

const (
	Ready State = "Ready"
	Error State = "Error"

	ConditionTypeReady ConditionType = "Ready"

	ConditionReasonUpdateSucceeded        ConditionReason = "UpdateSucceeded"
	ConditionReasonUpdateSucceededMessage                 = "Update succeeded"
	ConditionReasonUpdateFailed           ConditionReason = "UpdateFailed"
	ConditionReasonUpdateFailedMessage                    = "Update failed"

	ConditionReasonClusterImportSucceeded        ConditionReason = "ClusterImportSucceeded"
	ConditionReasonClusterImportSucceededMessage                 = "ClusterImport succeeded"
	ConditionReasonClusterImportFailed           ConditionReason = "ClusterImportFailed"
	ConditionReasonClusterImportFailedMessage                    = "ClusterImport failed"

	ConditionReasonIPPoolImportSucceeded        ConditionReason = "IPPoolImportSucceeded"
	ConditionReasonIPPoolImportSucceededMessage                 = "IPPoolImport succeeded"
	ConditionReasonIPPoolImportFailed           ConditionReason = "IPPoolImportFailed"
	ConditionReasonIPPoolImportFailedMessage                    = "IPPoolImport failed"
)

var conditionReasons = map[ConditionReason]conditionMeta{
	ConditionReasonUpdateSucceeded: {Type: ConditionTypeReady, Status: metav1.ConditionTrue, Message: ConditionReasonUpdateSucceededMessage},
	ConditionReasonUpdateFailed:    {Type: ConditionTypeReady, Status: metav1.ConditionFalse, Message: ConditionReasonUpdateFailedMessage},

	ConditionReasonClusterImportSucceeded: {Type: ConditionTypeReady, Status: metav1.ConditionTrue, Message: ConditionReasonClusterImportSucceededMessage},
	ConditionReasonClusterImportFailed:    {Type: ConditionTypeReady, Status: metav1.ConditionFalse, Message: ConditionReasonClusterImportFailedMessage},

	ConditionReasonIPPoolImportSucceeded: {Type: ConditionTypeReady, Status: metav1.ConditionTrue, Message: ConditionReasonIPPoolImportSucceededMessage},
	ConditionReasonIPPoolImportFailed:    {Type: ConditionTypeReady, Status: metav1.ConditionFalse, Message: ConditionReasonIPPoolImportFailedMessage},
}

type ReasonWithMessage struct {
	Reason  ConditionReason
	Message string
}

func ConditionFromReason(reason ReasonWithMessage) *metav1.Condition {
	condition, found := conditionReasons[reason.Reason]
	if found {
		message := condition.Message
		if reason.Message != "" {
			message = reason.Message
		}
		return &metav1.Condition{
			Type:    string(condition.Type),
			Status:  condition.Status,
			Reason:  string(reason.Reason),
			Message: message,
		}
	}
	return nil
}

type conditionMeta struct {
	Type    ConditionType
	Status  metav1.ConditionStatus
	Message string
}

func NewReasonWithMessage(reason ConditionReason, customMessage ...string) ReasonWithMessage {
	message := ""
	if len(customMessage) > 0 {
		message = customMessage[0]
	}
	return ReasonWithMessage{
		Reason:  reason,
		Message: message,
	}
}
