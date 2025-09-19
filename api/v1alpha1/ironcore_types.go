// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IronCoreSpec defines the desired state of IronCore.
type IronCoreSpec struct {
	Clusters []*ClusterSelector `json:"clusters,omitempty"`
}

// IronCoreStatus defines the observed state of IronCore.
type IronCoreStatus struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=Ready;Error
	State       State               `json:"state"`
	Conditions  *[]metav1.Condition `json:"conditions,omitempty"`
	Description string              `json:"description,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:JSONPath=".status.state",name="State",type="string"

// IronCore is the Schema for the IronCores API.
type IronCore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IronCoreSpec   `json:"spec,omitempty"`
	Status IronCoreStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// IronCoreList contains a list of IronCore.
type IronCoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IronCore `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IronCore{}, &IronCoreList{})
}
