# Tasks

## 1. Upstream chart: BMC credential logic

- [x] 1.1 Add `dist/chart/templates/extras/bmc-credentials-secret.yaml` — generates `bmc-<cluster.name>` Secret for each cluster entry with `bmcUser` and `bmcPassword`. Verify: `helm template` with inline credentials renders the Secret with correct name and `stringData`.
- [x] 1.2 Update `dist/chart/templates/extras/clusterImport.yaml` — strip `bmcUser`/`bmcPassword` from the cluster spec and inject `bmcCredentialsRef` when present. Verify: `helm template` output contains `bmcCredentialsRef`, does not contain `bmcUser`/`bmcPassword`, and matches the existing wrapper chart output for the same values.
- [x] 1.3 Delete the now-redundant `update.yaml` from the downstream wrapper chart — it is identical to `dist/chart/templates/extras/update.yaml`. Verify: `helm template` on the updated wrapper chart still renders Update CRs from `update:` values via the upstream subchart template.
- [x] 1.4 Publish a new upstream chart version and update the dependency pin in the downstream wrapper chart. Verify: `helm dependency update` succeeds and `helm template` renders the full chart without error.

## Workflow follow-up

- Once published, a downstream deployment pipeline change can proceed to delete the wrapper's `cluster-import.yaml` and `bmc-credentials-secret.yaml` and update the chart dependency.
- Archive this change.
