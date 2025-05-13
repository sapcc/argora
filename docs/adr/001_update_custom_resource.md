<!--
SPDX-FileCopyrightText: 2025 SAP SE

SPDX-License-Identifier: Apache-2.0
-->

# Update Custom Resource

## Status
Accepted

## Context

We provide a Kubernetes operator and custom resource (CR) that will handle Netbox updates based on cluster selection per region and type. Additionally, it will enable it to work for a single cluster. Multiple clusters may be selected per custom resource. The status of the reconcile operation should result in CR status, which could be used to fire alerts on errors.

## Decision

We specify a new CR named `Update` in the `argora.cloud.sap` API group with the following `spec`:

```go
type UpdateSpec struct {
  Clusters []*Clusters `json:"clusters,omitempty"`
}

type Clusters struct {
  Name string `json:"name,omitempty"`
  // +kubebuilder:validation:Required
  Region string `json:"region"`
  // +kubebuilder:validation:Required
  Type string `json:"type"`
}
```

This will allow us to specify which clusters to update. Also, single cluster selection is possible if the optional `Name` is specified. An example for the Update CR:

An example Update CR could look as follows:

```yaml
apiVersion: argora.cloud.sap/v1alpha1
kind: Update
metadata:
  name: qa-de-1
  namespace: argora-system
spec:
  clusters:
  - name:
  region: qa-de-1
  type: cc-kvm-compute
```

We could extend it in the future to support different kinds of updates.

When CR reconciliation finishes, the CR's status gets updated. We will have for now just two states: `Ready` and `Error`. On `Error`, we set a description with the failure.

We introduce conditions for the status which could serve as a base for future developments when we want to quickly understand where a problem has occurred and when. For the sake of simplicity, we implement the condition of type `Ready` that supports multiple reasons. This way, we can reduce the number of condition types. We use the [SetStatusCondition function](https://pkg.go.dev/k8s.io/apimachinery/pkg/api/meta#SetStatusCondition) of [k8s.io/apimachinery](https://github.com/kubernetes/apimachinery). The **lastTransitionTime** is only updated when the status of the condition changes.

Conditions:

| CR state   | Type   | Status | Reason          | Message          |
|------------|--------|--------|-----------------|------------------|
| Ready      | Ready  | True   | UpdateSucceeded | Update succeeded |
| Error      | Ready  | False  | UpdateFailed    | Update failed    |

## Consequences
This decision allows our team to monitor the state of the regularly needed Netbox updates more easily.
