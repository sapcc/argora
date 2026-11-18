# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
make build-all          # build binary to build/manager
make test               # run unit/integration tests (requires setup-envtest first)
make setup-envtest      # download envtest binaries (run once before make test)
make lint               # run golangci-lint
make static-check       # lint + shellcheck + license checks
make check              # full suite: static-check + tests + build
make manifests          # regenerate CRDs from kubebuilder markers
make generate           # regenerate DeepCopy methods
make tilt               # local dev on kind cluster via Tilt
```

Run a single Ginkgo test file or spec:
```bash
go run github.com/onsi/ginkgo/v2/ginkgo run -v --label-filter '<label>' ./internal/controller/...
# or with standard go test:
KUBEBUILDER_ASSETS=$(./bin/setup-envtest use --bin-dir ./bin -p path) \
  go test ./internal/controller/... -run TestControllers/IronCore
```

After changing API types in `api/v1alpha1/`, always run:
```bash
make manifests generate
```

All commits must be signed with DCO sign-off:
```bash
git commit -s -S -m "message"
```

## Architecture

Argora is a Kubernetes operator that uses **Netbox as the source of truth** to reconcile bare-metal infrastructure resources. It has two mutually exclusive provisioning modes (controlled by `--enable-ironcore`, default `true`) and three additional controllers that run in both modes.

### CRDs (`api/v1alpha1/`)

| CRD | Purpose |
|---|---|
| `ClusterImport` | Selects Netbox clusters to import into IronCore (or Metal3) |
| `Update` | Selects Netbox clusters for Netbox data normalization |
| `IPPoolImport` | Selects Netbox IP prefixes to import as CAPI IPAM pools |

`ClusterSelector` (shared across `ClusterImport` and `Update`) selects clusters by `name`, `region`, and `type`. The `BMCCredentialsRef` field is only used by the IronCore controller.

### Controllers (`internal/controller/`)

**IronCore mode** (`--enable-ironcore=true`, default): `IronCoreReconciler` watches `ClusterImport` → queries Netbox for clusters+devices → creates `BMCSecret` + `BMC` (metal-operator CRDs) for each active device. If `--readiness-checks=serverwiring` is set, it also creates/updates `ServerWiring` objects (metal-maintenance-operator CRD) from Netbox interface data.

**Metal3 mode** (`--enable-ironcore=false`): `Metal3Reconciler` watches CAPI `Cluster` → queries Netbox → creates `BareMetalHost` + a `networkdata-<device>` Secret for each active device. Requires CAPI CRDs to be present.

**Both modes**:
- `UpdateReconciler` watches `Update` → normalizes Netbox device data: renames vendor management interfaces to `remoteboard`, sets OOB IP and platform to GardenLinux, removes VMK interfaces/IPs, and updates BMC hostnames.
- `IPPoolImportReconciler` watches `IPPoolImport` → creates/updates `InClusterIPPool` objects (CAPI IPAM) from matching Netbox prefixes.
- `IPUpdateReconciler` watches CAPI `IPAddress` claims → assigns/de-assigns IP addresses in Netbox on the corresponding device interface, using a finalizer to handle deletion.

### Netbox client (`internal/netbox/`)

`Netbox` interface wraps `go-netbox-go` and exposes four sub-clients: `DCIM()`, `IPAM()`, `Virtualization()`, `Extras()`. The client is re-initialized on every reconcile via `Reload(token, logger)` — credentials and the Netbox token are re-read from disk each time (`internal/credentials/`).

### Credentials (`internal/credentials/`)

`Credentials.Reload()` reads `/etc/credentials/credentials.json` (fields: `bmcUser`, `bmcPassword`, `netboxToken`) on every reconcile. In production the file is mounted from a Kubernetes Secret; in Tilt/kind it is provided via `dist/chart/values.yaml`.

### Annotation

Adding `argora.cloud.sap/ignore: "true"` to a `BMCSecret` (IronCore) or `bmc-secret-<device>` Secret (Metal3) causes the controller to skip updating credentials for that device without skipping the rest of reconciliation.

### Testing

Tests use **Ginkgo/Gomega**. Most controller tests use `fake.Client` with a mock Netbox (`internal/controller/mock/mock.go`). `ipupdate_controller_integration_test.go` uses `envtest` for a real API server. The test suite bootstraps envtest CRDs from `config/crd/bases/` and `hack/crd/`.
