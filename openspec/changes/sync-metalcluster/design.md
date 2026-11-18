# Design

## Context

The `IronCoreReconciler` currently iterates `ClusterImport` → Netbox clusters → devices and creates one `BMC` + one `BMCSecret` per active device. The new MOO model inverts this: Argora provides static inventory and MOO creates BMC CRs lazily. `MetalCluster` is cluster-scoped and holds all devices for one cluster; `OOBSubnet` is also cluster-scoped and drives the metaldhcp DHCP service.

All Netbox data needed for both CRs is already reachable with existing Netbox client methods:

| Field | Source |
|---|---|
| `ServerEntry.Name` | `device.Name` |
| `ServerEntry.Serial` | `device.Serial` |
| `ServerEntry.RemoteboardMAC` | `DCIM().GetInterfaceForDevice(device, "remoteboard").MacAddress` |
| `ServerEntry.Interfaces` | `DCIM().GetInterfacesForDevice(device)` (exists, used by `reconcileServerWiring`) |
| `OOBSubnet.CIDR` | mask of `device.OOBIp.Address` (no Netbox call) |
| `OOBSubnet.Gateway` | first host in CIDR (no Netbox call) |
| `StaticLease.IP` | host part of `device.OOBIp.Address` |
| `StaticLease.MAC` | remoteboard `MacAddress` (same call as above) |

## Goals / Non-Goals

**Goals:**
- Add `--sync-metalcluster` flag that switches the IronCore reconciler path
- Produce correct `MetalCluster` and `OOBSubnet` CRs kept in sync with Netbox
- Zero impact on existing deployments (flag defaults to `false`)

**Non-Goals:**
- Deleting pre-existing `BMC`/`BMCSecret` CRs when switching modes — that is an operator migration concern
- Per-`ClusterSelector` flag granularity — global flag only
- Populating `OOBSubnet.RangeStart/RangeEnd`, `BootURL`, `DNSServers`, or `Mode` — those stay at defaults; the static leases are the only non-CIDR data Netbox can provide

## Decisions

### D1 — Extend `IronCoreReconciler` rather than add a new reconciler

**Decision**: add a `syncMetalCluster bool` field to `IronCoreReconciler` and branch inside `reconcileCluster`. Both paths share credential reload, Netbox client reload, cluster lookup, and device iteration.

**Alternative**: separate `MetalClusterReconciler` watching `ClusterImport`. Rejected — it duplicates the outer loop and complicates the flag check; the existing reconciler structure is a better fit.

### D2 — Collect all devices before writing CRs (cluster-level aggregation)

**Decision**: iterate all devices for a cluster first, accumulate `ServerEntry` slice and group by CIDR, then do one `CreateOrUpdate` per `MetalCluster` and per `OOBSubnet`. This avoids partial-write states and matches the controller-runtime idiomatic `CreateOrUpdate` pattern already used in `reconcileServerWiring`.

**Alternative**: write each device immediately. Rejected — `MetalCluster` is a single CR with a list; partial writes would require read-modify-write on every device.

### D3 — Derive OOB CIDR from `device.OOBIp.Address` without an extra Netbox call

**Decision**: `device.OOBIp.Address` is already in CIDR notation (e.g., `10.1.2.3/24`). Mask it with `netip.ParsePrefix` → `.Masked()` to get the network address. No `GetPrefixesContaining` call needed.

**Alternative**: call `IPAM().GetPrefixesContaining(device.OOBIp.Address)` for the authoritative prefix. Rejected — adds N Netbox API calls per device per reconcile; the address already carries the prefix length, so it is sufficient.

### D4 — OOBSubnet named by slugified CIDR

**Decision**: name format `oobsubnet-<a>-<b>-<c>-<d>-<prefix>` (e.g., `oobsubnet-10-1-2-0-24`). Globally unique, human-readable, derivable without state.

**Alternative**: name after the cluster. Rejected — a cluster can span multiple OOB CIDRs; cluster-named CRs would collide.

### D5 — Interface DPU detection via Netbox tags

**Decision**: check `iface.Tags[].Name` for `"SmartNIC-mgmt"` or `"SmartNIC-NIC1"`. Set `InterfaceEntry.Type = "dpu"`.

**Rationale**: tag-based detection matches the existing `getRoleFromTags` pattern in the codebase and is consistent with how the UpdateReconciler annotates SmartNIC interfaces.

### D6 — Two new go.mod dependencies; no replace directives

MOO and metaldhcp are in the same GitHub instance (`github.com/SAP-cloud-infrastructure`). They are referenced by commit SHA pseudoversions. No workspace-level `replace` directives are used; the Argora module fetches them from the SAP internal proxy.

### D7 — RBAC via kubebuilder annotations; chart regenerated via `make manifests`

New `// +kubebuilder:rbac:` annotations on the `IronCoreReconciler` reconcile method. `make manifests` regenerates `dist/chart/templates/rbac/manager-role.yaml`. The chart version is bumped in the same PR.

## Risks / Trade-offs

| Risk | Mitigation |
|---|---|
| Existing BMC/BMCSecret CRs remain if an operator switches mode on a running cluster | Document in release notes; out of scope for this change |
| `GetInterfaceForDevice(device, "remoteboard")` returns error for devices with no remoteboard interface in Netbox | Log warning, set `RemoteboardMAC = ""` and omit the `oob` interface entry; do not fail the whole reconcile |
| MOO or metaldhcp CRDs not installed on the target cluster | `CreateOrUpdate` returns a no-match-for-kind error; IronCore reconciler returns error and requeues (standard controller error handling) |
| Two new transitive dependencies pulling in conflicting k8s library versions | Resolve at implementation time with `go mod tidy`; both repos target the same k8s minor |

## Migration Plan

1. Deploy new Argora binary (chart version bump).
2. Install MOO and metaldhcp CRDs on the cluster if not already present.
3. Add `--sync-metalcluster` to `manager.args` in the values override for the target instance.
4. Verify `MetalCluster` and `OOBSubnet` CRs appear and are populated correctly.
5. Existing `BMC`/`BMCSecret` CRs are unaffected; clean them up manually after verifying MOO creates new ones from `MetalCluster` inventory.

**Rollback**: remove `--sync-metalcluster` from `manager.args` and redeploy. The IronCore reconciler resumes creating `BMC`/`BMCSecret` CRs as before.
