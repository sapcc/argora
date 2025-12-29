// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

type ClusterSelector struct {
	// +kubebuilder:validation:Optional
	Name string `json:"name,omitempty"`
	// +kubebuilder:validation:Optional
	Region string `json:"region,omitempty"`
	// +kubebuilder:validation:Optional
	Type string `json:"type,omitempty"`
}

type IPPoolSelector struct {
	// +kubebuilder:validation:Optional
	NamePrefix string `json:"namePrefix,omitempty"`
	// +kubebuilder:validation:Optional
	Region string `json:"region,omitempty"`
	// +kubebuilder:validation:Optional
	Role string `json:"role,omitempty"`
	// +kubebuilder:validation:Optional
	ExcludeMask *int `json:"excludeMask,omitempty"`
}
