// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterImportSpec defines the desired state of ClusterImport.
type ClusterImportSpec struct {
	Clusters []*ClusterSelector `json:"clusters,omitempty"`
}

// ClusterImportStatus defines the observed state of ClusterImport.
type ClusterImportStatus struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=Ready;Error
	State       State               `json:"state"`
	Conditions  *[]metav1.Condition `json:"conditions,omitempty"`
	Description string              `json:"description,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:JSONPath=".status.state",name="State",type="string"

// ClusterImport is the Schema for the ClusterImports API.
type ClusterImport struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterImportSpec   `json:"spec,omitempty"`
	Status ClusterImportStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterImportList contains a list of ClusterImport.
type ClusterImportList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterImport `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterImport{}, &ClusterImportList{})
}
