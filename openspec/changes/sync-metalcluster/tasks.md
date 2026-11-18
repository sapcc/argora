# Tasks

## 1. Dependencies and module setup

- [ ] 1.1 Add `github.com/SAP-cloud-infrastructure/metal-onboarding-operator` and `github.com/SAP-cloud-infrastructure/metaldhcp` to `go.mod` via `go get`, run `go mod tidy`, and verify `go build ./...` succeeds with no missing-import errors.

## 2. Flag and scheme wiring

- [ ] 2.1 Add `--sync-metalcluster` bool flag (default `false`) in `cmd/manager/main.go` alongside the existing `--enable-ironcore` flag. Verify the binary starts with `./build/manager --help` and the flag appears in output.
- [ ] 2.2 Register `metalv1alpha1` scheme from `metal-onboarding-operator` and `metaldhcpv1alpha1` scheme from `metaldhcp` in `main.go` (unconditional — scheme registration is cheap and avoids startup-order issues). Verify `make build-all` succeeds.
- [ ] 2.3 Pass `syncMetalCluster` to `NewIronCoreReconciler` and store it on the struct. Verify compilation.

## 3. MetalCluster reconciliation

- [ ] 3.1 Add helper `buildServerEntries(devices []models.Device, netBox netbox.Netbox) ([]moov1alpha1.ServerEntry, error)` in `internal/controller/ironcore_controller.go`: for each active device fetch remoteboard iface + all interfaces, map types (remoteboard→oob, SmartNIC tags→dpu, others→nic), populate `Name/Serial/RemoteboardMAC/Interfaces/Labels`. Verify with a unit test using the existing fake Netbox mock that covers the three interface type cases and the inactive-device-excluded case.
- [ ] 3.2 Add `reconcileMetalCluster(ctx, clusterName string, servers []moov1alpha1.ServerEntry) error` using `controllerutil.CreateOrUpdate` to create or patch the `MetalCluster` CR. Verify with a unit test that both create and update paths set `spec.servers` correctly.
- [ ] 3.3 In the IronCore reconciler's cluster loop, branch on `r.syncMetalCluster`: when `true`, call `buildServerEntries` then `reconcileMetalCluster` instead of `reconcileBmcSecret`/`createBmc`. Verify existing tests still pass (`make test`).

## 4. OOBSubnet reconciliation

- [ ] 4.1 Add helper `groupByOOBCIDR(devices []models.Device) (map[string][]models.Device, error)`: parse and mask each `device.OOBIp.Address` with `netip.ParsePrefix(...).Masked()`, return map of CIDR string → device slice. Verify with a unit test covering single-CIDR and multi-CIDR clusters and the CIDR-to-name slug format (`oobsubnet-10-1-2-0-24`).
- [ ] 4.2 Add helper `buildStaticLeases(devices []models.Device, netBox netbox.Netbox) ([]metaldhcpv1alpha1.StaticLease, error)`: for each device, use the remoteboard MAC already fetched (or re-fetch if not cached) and host part of `device.OOBIp.Address` as IP. Verify unit test covers correct MAC, IP, and Hostname fields.
- [ ] 4.3 Add `reconcileOOBSubnet(ctx, cidr string, leases []metaldhcpv1alpha1.StaticLease) error` using `controllerutil.CreateOrUpdate` to set `spec.cidr`, `spec.gateway` (first host in CIDR via `netip.Addr.Next()`), and `spec.staticLeases`. Verify with a unit test for create and update paths, including gateway derivation for `/24` and `/25` CIDRs.
- [ ] 4.4 In the `syncMetalCluster` branch, after `reconcileMetalCluster`, call `groupByOOBCIDR` and `reconcileOOBSubnet` for each CIDR. Verify `make test` passes.

## 5. RBAC and chart update

- [ ] 5.1 Add kubebuilder RBAC annotations to the `IronCoreReconciler.Reconcile` method for `onboarding.metal.ironcore.dev/metalclusters` and `dhcp.metal.ironcore.dev/oobsubnets` (verbs: `get;list;watch;create;update;patch`). Run `make manifests` and verify the two new rule blocks appear in `dist/chart/templates/rbac/manager-role.yaml`.

## 6. Integration verification

- [ ] 6.1 Run the full test suite (`make check`) and confirm it passes with no new failures.
- [ ] 6.2 Run Argora locally in a kind cluster via `make tilt` with `--sync-metalcluster` set in the Tilt values, create a test `ClusterImport`, and verify a `MetalCluster` CR and at least one `OOBSubnet` CR are created with expected field values.

## Workflow follow-up

- Archive the change after the PR is merged and the review is complete.
