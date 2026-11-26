# Spec Delta

## Purpose

Allows per-cluster BMC credential overrides to be specified inline in ClusterImport chart values, with the chart generating the required Secret and bmcCredentialsRef automatically, without operators needing to manage Secrets separately.

## ADDED Requirements

### Requirement: Inline BMC credentials generate a Secret
When a cluster entry in `clusterImport` values includes both `bmcUser` and `bmcPassword`, the chart SHALL generate an `Opaque` Secret named `bmc-<cluster.name>` in the same namespace containing those credentials.

#### Scenario: Secret created for cluster with inline credentials
- **WHEN** a cluster entry has `bmcUser: "admin"` and `bmcPassword: "secret"`
- **AND** the cluster name is `my-cluster`
- **THEN** a Secret named `bmc-my-cluster` is rendered with `bmcUser` and `bmcPassword` in `stringData`

#### Scenario: No Secret for cluster without inline credentials
- **WHEN** a cluster entry has no `bmcUser` or `bmcPassword` fields
- **THEN** no `bmc-<name>` Secret is rendered for that cluster

### Requirement: bmcCredentialsRef injected into ClusterImport spec
When inline credentials are present, the chart SHALL add a `bmcCredentialsRef` field to the cluster entry in the rendered ClusterImport spec referencing the generated Secret.

#### Scenario: bmcCredentialsRef added to cluster spec
- **WHEN** a cluster entry has `bmcUser` and `bmcPassword`
- **THEN** the rendered ClusterImport spec contains `bmcCredentialsRef.name: bmc-<cluster.name>` for that cluster entry

### Requirement: Inline credentials stripped from ClusterImport spec
The `bmcUser` and `bmcPassword` fields SHALL be omitted from the rendered ClusterImport spec; they are consumed by the chart and must not appear in the CR.

#### Scenario: Credentials absent from rendered CR
- **WHEN** a cluster entry has `bmcUser` and `bmcPassword`
- **THEN** the rendered ClusterImport spec does not contain `bmcUser` or `bmcPassword` fields for that cluster entry
