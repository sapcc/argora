# Design

## Context

See proposal.md for motivation.

The clusterImport BMC credential logic currently lives in a downstream wrapper chart (`cluster-import.yaml` and `bmc-credentials-secret.yaml`). The upstream chart's `clusterImport.yaml` uses `toYaml $clusters | nindent 2` without any credential handling, and `update.yaml` in the wrapper is identical to the upstream version.

The upstream chart already supports `extraObjects` and has the right template structure. Adding the credential logic directly — mirroring the wrapper's existing template — is the smallest, most auditable change.

## Goals / Non-Goals

**Goals:**
- BMC credential logic in the upstream chart (`dist/chart`), removing the need for wrapper duplicates
- Wrapper chart can remove `cluster-import.yaml`, `bmc-credentials-secret.yaml`, and `update.yaml` after this is published

**Non-Goals:**
- Adding `tpl` evaluation to cluster values (not needed for this use case; can be added independently)
- Any post-renderer or Gardener ManagedResource support (tracked in a downstream deployment pipeline change)

## Decisions

### Credential injection as explicit template logic, not `tpl` evaluation

**Decision:** Add the `bmcUser`/`bmcPassword` → `bmcCredentialsRef` + Secret generation as direct Go template logic in `clusterImport.yaml` and a new `bmc-credentials-secret.yaml`, mirroring the existing wrapper template.

**Rationale:** The credential injection logic is well-defined and belongs as explicit template logic, not user-side expression evaluation. `tpl` can be added independently if needed.

**Alternative considered:** Use `tpl (toYaml ...) $` in the upstream clusterImport template to allow referencing `{{ printf "bmc-%s" .name }}` as a value string. Not necessary for the current use case.

---

### Wrapper `update.yaml` deleted immediately

**Decision:** Delete the downstream wrapper chart's `update.yaml` as part of this change, since it is identical to the upstream template and will be rendered via the subchart.

**Rationale:** Keeping the duplicate requires ongoing maintenance to stay in sync. It is immediately redundant once the chart dependency is updated.

## Open Questions

None.
