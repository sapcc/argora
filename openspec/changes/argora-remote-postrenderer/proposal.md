# Proposal

## Why

The `bmcUser`/`bmcPassword` → `bmcCredentialsRef` + Secret transformation that allows per-cluster BMC credential overrides in ClusterImport values currently lives only in the `helm-charts` wrapper chart. It belongs in the upstream argora chart (`dist/chart/templates/extras/`) so any chart consumer gets it without duplicating the template logic.

This is a prerequisite for a downstream deployment pipeline consolidation: the wrapper's `cluster-import.yaml` and `bmc-credentials-secret.yaml` can only be removed once the upstream chart provides the equivalent logic.

## What Changes

- **Add `dist/chart/templates/extras/bmc-credentials-secret.yaml`**: generates a `bmc-<cluster.name>` Secret for each cluster entry that specifies `bmcUser` and `bmcPassword`.
- **Update `dist/chart/templates/extras/clusterImport.yaml`**: strip `bmcUser`/`bmcPassword` from the cluster spec and inject `bmcCredentialsRef` referencing the generated Secret.
- **Delete the redundant `update.yaml` from the downstream wrapper chart**: it is identical to `dist/chart/templates/extras/update.yaml`. No behavior change — the upstream subchart template renders the same output.
- **Publish a new upstream chart version** and update the dependency pin in the downstream wrapper chart.

## Capabilities

### New Capabilities

- `inline-bmc-credentials`: ClusterImport entries support per-cluster BMC credential overrides specified inline in chart values (`bmcUser`, `bmcPassword`), with the chart generating the corresponding `bmc-<name>` Secret and `bmcCredentialsRef` automatically.

### Modified Capabilities

(none)

## Impact

**`argora` repo (`dist/chart/`):**
- `templates/extras/clusterImport.yaml`: gains `bmcUser`/`bmcPassword` → `bmcCredentialsRef` logic
- `templates/extras/bmc-credentials-secret.yaml`: new template for per-cluster BMC credential Secrets

**Downstream wrapper chart:**
- `templates/update.yaml`: deleted (identical to upstream, rendered via subchart)
- `Chart.yaml` and `Chart.lock`: dependency pin updated to new upstream version
