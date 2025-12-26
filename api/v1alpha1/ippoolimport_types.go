// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IPPoolImportSpec defines the desired state of IPPoolImport
type IPPoolImportSpec struct {
	IPPools []*IPPoolSelector `json:"ippools,omitempty"`
}

// IPPoolImportStatus defines the observed state of IPPoolImport.
type IPPoolImportStatus struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=Ready;Error
	State       State               `json:"state"`
	Conditions  *[]metav1.Condition `json:"conditions,omitempty"`
	Description string              `json:"description,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:JSONPath=".status.state",name="State",type="string"

// IPPoolImport is the Schema for the ippoolimports API
type IPPoolImport struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IPPoolImportSpec   `json:"spec,omitempty"`
	Status IPPoolImportStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// IPPoolImportList contains a list of IPPoolImport
type IPPoolImportList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []IPPoolImport `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IPPoolImport{}, &IPPoolImportList{})
}
