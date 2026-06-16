// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// UpdateSpec defines the desired state of Update.
type UpdateSpec struct {
	Clusters []*ClusterSelector `json:"clusters,omitempty"`
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
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, &Update{}, &UpdateList{})
		return nil
	})
}
