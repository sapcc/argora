# Proposal

## Why

Argora's IronCore mode eagerly creates `BMC` and `BMCSecret` CRs the moment a device appears in Netbox. The metal-onboarding-operator (MOO) introduces a complementary model: Argora provides static inventory in `MetalCluster` CRs and MOO lazily creates `BMC` CRs when servers actually check in via DHCP. Additionally, the metaldhcp operator needs `OOBSubnet` CRs to serve DHCP on BMC management networks, and Netbox is the canonical source for that subnet data. Without this change, both those CRs must be managed manually.

## What Changes

- New `--sync-metalcluster` global flag on the Argora manager (default `false`).
- When the flag is set, the `IronCoreReconciler` no longer creates `BMC` or `BMCSecret` CRs. Instead, for each Netbox cluster it creates/updates one `MetalCluster` CR (MOO) containing the full device inventory, and one or more `OOBSubnet` CRs (metaldhcp) per unique OOB CIDR in that cluster.
- Both `MetalCluster` and `OOBSubnet` are kept in sync on every reconcile (Netbox remains the source of truth).
- Two new Go module dependencies added: `github.com/SAP-cloud-infrastructure/metal-onboarding-operator` and `github.com/SAP-cloud-infrastructure/metaldhcp`.
- RBAC extended with `get;list;watch;create;update;patch` on `metalclusters` (`onboarding.metal.ironcore.dev`) and `oobsubnets` (`dhcp.metal.ironcore.dev`).
- Chart: no template changes required. The flag is passed via `manager.args` in values overrides. `make manifests` regenerates the RBAC ClusterRole, which is committed as a chart update.
- Default behavior (no flag) is completely unchanged.

## Capabilities

### New Capabilities

- `sync-metalcluster`: Syncing Netbox cluster and device inventory into `MetalCluster` CRs (MOO) and `OOBSubnet` CRs (metaldhcp) as an alternative to the default BMC/BMCSecret provisioning path, enabled by `--sync-metalcluster`.

### Modified Capabilities

<!-- No existing specs exist; all capabilities in this project are new. -->

## Impact

- **`internal/controller/ironcore_controller.go`**: primary change site — new reconcile branches behind the flag.
- **`cmd/manager/main.go`**: new flag, two new scheme registrations.
- **`go.mod` / `go.sum`**: two new dependencies.
- **`dist/chart/templates/rbac/manager-role.yaml`**: regenerated via `make manifests`.
- **No Argora CRD changes** — `ClusterImport` and `ClusterSelector` are unmodified.
- **Backwards compatibility**: existing deployments unaffected; flag defaults to `false`.
