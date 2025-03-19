// Copyright 2025 SAP SE
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
)

type ReasonWithMessage struct {
	Reason  ConditionReason
	Message string
}

// UpdateSpec defines the desired state of Update.
type UpdateSpec struct {
	Clusters []*Clusters `json:"clusters,omitempty"`
}

type Clusters struct {
	Name   string `json:"name,omitempty"`
	Region string `json:"region,omitempty"`
	Type   string `json:"type,omitempty"`
}

// UpdateStatus defines the observed state of Update.
type UpdateStatus struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=Ready;Error
	State       State               `json:"state"`
	Conditions  *[]metav1.Condition `json:"conditions,omitempty"`
	Description string              `json:"description,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:JSONPath=".status.state",name="State",type="string"

// Update is the Schema for the updates API.
type Update struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UpdateSpec   `json:"spec,omitempty"`
	Status UpdateStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// UpdateList contains a list of Update.
type UpdateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Update `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Update{}, &UpdateList{})
}
