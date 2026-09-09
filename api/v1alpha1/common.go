// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import corev1 "k8s.io/api/core/v1"

const AnnotationIgnore = "argora.cloud.sap/ignore"

// ClusterSelector is intentionally shared between ClusterImport and Update CRDs.
// Controller-specific fields (e.g. BMCCredentialsRef) are simply ignored by controllers that don't need them.
type ClusterSelector struct {
	// +kubebuilder:validation:Optional
	Name string `json:"name,omitempty"`
	// +kubebuilder:validation:Optional
	Region string `json:"region,omitempty"`
	// +kubebuilder:validation:Optional
	Type string `json:"type,omitempty"`
	// BMCCredentialsRef optionally references a Secret (same namespace) containing
	// bmcUser and bmcPassword keys to override central BMC credentials.
	// Used by the ironcore controller; ignored by others.
	// +kubebuilder:validation:Optional
	BMCCredentialsRef *corev1.LocalObjectReference `json:"bmcCredentialsRef,omitempty"`
}

// IPPoolSelector defines the selection criteria for an IP pool to be imported.
// +kubebuilder:validation:XValidation:rule="has(self.namePrefix) != has(self.nameOverride)", message="either namePrefix or nameOverride must be set, but not both"
type IPPoolSelector struct {
	// +kubebuilder:validation:Optional
	NamePrefix string `json:"namePrefix,omitempty"`
	// +kubebuilder:validation:Optional
	NameOverride string `json:"nameOverride,omitempty"`
	// +kubebuilder:validation:Optional
	Region string `json:"region,omitempty"`
	// +kubebuilder:validation:Optional
	Role string `json:"role,omitempty"`
	// Prefix optionally restricts the selection to the single NetBox prefix with
	// this exact CIDR. Use it to disambiguate when region+role match more than one
	// prefix (e.g. several management-transit networks share a region and role).
	// An empty value means no prefix filter. MaxLength is capped at 17 to keep the
	// isCIDR CEL rule within the apiserver's per-schema cost budget; that fits every
	// NetBox prefix in scope (IPv4 network CIDRs such as 10.219.139.128/26).
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxLength=17
	// +kubebuilder:validation:XValidation:rule="self == '' || isCIDR(self)",message="prefix must be a valid CIDR"
	Prefix string `json:"prefix,omitempty"`
	// +kubebuilder:validation:Optional
	ExcludeMask *int `json:"excludeMask,omitempty"`
	// +kubebuilder:validation:Optional
	ExcludedAddresses []string `json:"excludedAddresses,omitempty"`
	// +kubebuilder:validation:Optional
	ExcludeLastNAddresses *int `json:"excludeLastNAddresses,omitempty"`
}
