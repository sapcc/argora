# Spec Delta

## Purpose

Enables Argora to sync Netbox device inventory into `MetalCluster` CRs (metal-onboarding-operator) and `OOBSubnet` CRs (metaldhcp), providing static inventory and DHCP subnet data for the lazy-provisioning model where MOO creates BMC CRs only when servers check in, rather than eagerly from Netbox.

## ADDED Requirements

### Requirement: MetalCluster sync mode activation
When the `--sync-metalcluster` flag is set, Argora SHALL switch the IronCore reconciler to MetalCluster sync mode for all clusters. In this mode Argora SHALL NOT create `BMC` or `BMCSecret` CRs.

#### Scenario: Default mode unchanged
- **WHEN** Argora starts without `--sync-metalcluster`
- **THEN** it creates `BMC` and `BMCSecret` CRs as before, and no `MetalCluster` or `OOBSubnet` CRs are created

#### Scenario: Flag enables new mode
- **WHEN** Argora starts with `--sync-metalcluster`
- **THEN** it creates `MetalCluster` and `OOBSubnet` CRs instead of `BMC` and `BMCSecret` CRs

### Requirement: MetalCluster per cluster
For each Netbox cluster matched by a `ClusterImport` selector, Argora SHALL create or update one `MetalCluster` CR named after the Netbox cluster.

#### Scenario: MetalCluster created on first reconcile
- **WHEN** a Netbox cluster matches a `ClusterImport` selector and no `MetalCluster` CR with that name exists
- **THEN** Argora creates a `MetalCluster` CR whose name equals the Netbox cluster name

#### Scenario: MetalCluster updated when devices change
- **WHEN** a device is added to or removed from a Netbox cluster
- **AND** Argora reconciles
- **THEN** the `MetalCluster` CR's `spec.servers` list is updated to reflect the current active device set

### Requirement: MetalCluster server entries
Each active device in a Netbox cluster SHALL be represented as a `ServerEntry` in the `MetalCluster` spec with `name`, `serial`, `remoteboardMAC`, `interfaces`, and `labels` populated from Netbox.

#### Scenario: Server entry fields populated
- **WHEN** Argora builds a `ServerEntry` for an active device
- **THEN** `name` equals the device name in Netbox
- **AND** `serial` equals the device's chassis serial number from Netbox
- **AND** `remoteboardMAC` equals the MAC address of the device's `remoteboard` interface in Netbox
- **AND** `labels` includes region, zone, cluster name, device type, role, and platform

#### Scenario: Inactive device excluded
- **WHEN** a device in a Netbox cluster has status other than `active`
- **THEN** no `ServerEntry` is created for that device in the `MetalCluster`

### Requirement: MetalCluster interface type mapping
Each non-management, non-LAG interface with a non-empty MAC address SHALL be included in a server's `interfaces` list with a type of `oob`, `dpu`, or `nic` based on the interface's Netbox properties.

#### Scenario: Remoteboard interface mapped to oob
- **WHEN** an interface is named `remoteboard`
- **THEN** its entry in `interfaces` has type `oob`

#### Scenario: SmartNIC interface mapped to dpu
- **WHEN** an interface has a Netbox tag of `SmartNIC-mgmt` or `SmartNIC-NIC1`
- **THEN** its entry in `interfaces` has type `dpu`

#### Scenario: Standard NIC mapped to nic
- **WHEN** an interface is not the remoteboard and carries no SmartNIC tag, and is not mgmt-only and not a LAG, and has a non-empty MAC address
- **THEN** its entry in `interfaces` has type `nic`

### Requirement: OOBSubnet per unique OOB CIDR
For each unique OOB subnet CIDR present in a cluster's devices, Argora SHALL create or update one `OOBSubnet` CR. The CIDR is derived by masking the host address in `device.OOBIp.Address`.

#### Scenario: Single OOBSubnet for shared subnet
- **WHEN** all devices in a cluster share the same OOB CIDR
- **THEN** exactly one `OOBSubnet` CR is created for that cluster

#### Scenario: Multiple OOBSubnets for multiple CIDRs
- **WHEN** devices in a cluster belong to two distinct OOB CIDRs
- **THEN** two `OOBSubnet` CRs are created, one per CIDR

#### Scenario: OOBSubnet name is CIDR-derived
- **WHEN** Argora creates an `OOBSubnet` for CIDR `10.1.2.0/24`
- **THEN** the CR is named `oobsubnet-10-1-2-0-24`

### Requirement: OOBSubnet CIDR and gateway
The `OOBSubnet` `spec.cidr` SHALL be set to the masked network address derived from the device OOB IP. `spec.gateway` SHALL be set to the first usable host address in that CIDR.

#### Scenario: CIDR and gateway derived from OOB IP
- **WHEN** a device has `OOBIp.Address` of `10.1.2.3/24`
- **THEN** `spec.cidr` is `10.1.2.0/24` and `spec.gateway` is `10.1.2.1`

### Requirement: OOBSubnet static leases
For each device whose OOB IP falls in an `OOBSubnet`'s CIDR, Argora SHALL add a `StaticLease` entry to that subnet pinning the device's remoteboard MAC to its assigned OOB IP with the device name as hostname.

#### Scenario: Static lease populated for each device
- **WHEN** Argora reconciles an `OOBSubnet`
- **THEN** each active device in that subnet has a `StaticLease` with `mac` equal to the remoteboard MAC, `ip` equal to the host part of `device.OOBIp.Address`, and `hostname` equal to the device name

#### Scenario: Static leases updated on device change
- **WHEN** a device's remoteboard MAC changes in Netbox
- **THEN** the corresponding `StaticLease` in the `OOBSubnet` is updated on the next reconcile

### Requirement: Continuous sync from Netbox
In `--sync-metalcluster` mode, `MetalCluster` and `OOBSubnet` CRs SHALL be kept in sync with Netbox on every reconcile cycle driven by the existing `--reconcile-interval`.

#### Scenario: Netbox change reflected on next reconcile
- **WHEN** a device's serial number or interface MAC is changed in Netbox
- **AND** the reconcile interval elapses
- **THEN** the corresponding `MetalCluster` `ServerEntry` reflects the updated value
