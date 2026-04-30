# S02 Research — Define runtime-consumable metadata schema

## Summary

This slice directly owns **R013** (defined Argo CD Application metadata contract) and **R014** (labels for stable identity, annotations for supplementary context), and supports later **R016/R018** by choosing one metadata contract that code, docs, and proof can align around. The core seam from S01 is real: runtime observers already consume stable workload labels, but Argo `Application` creation currently carries only `status` and `devflow.io/release-id` plus trace annotations, so the handoff contract is split across bundle-rendered workloads and Argo objects rather than documented as one runtime-consumable schema.

This is **targeted research**, not greenfield architecture. The repo already contains the two ends of the contract:
- release bundle rendering injects stable workload labels into rendered Kubernetes workloads in `internal/release/service/release_bundle.go`
- runtime observers consume those labels in `internal/runtime/observer/kubernetes_runtime.go` and `internal/runtime/observer/release_rollout.go`

What is missing is the explicit schema boundary for **Argo CD Application + downstream workload metadata together**, plus tests/docs that say which fields are required, where they must live, and which consumer depends on each one.

## Recommendation

Treat S02 as a **contract-definition and producer-alignment slice**, not a runtime behavior rewrite.

Recommended contract:
- **Stable labels** for runtime identity and queryability:
  - `devflow.io/release-id`
  - `devflow.application/id`
  - `devflow.environment/id`
  - keep `app.kubernetes.io/name` as the workload-selection/name anchor already used by runtime observers
- **Supplementary annotations** for non-indexed context:
  - tracing annotations already added during Argo Application creation
  - optional artifact/reference/context annotations if later slices need them, but not as runtime lookup requirements

Recommended production points:
1. **Rendered workload resources remain the primary runtime-consumable source** for runtime observers, because the observers read Kubernetes workload labels after deployment lands.
2. **Argo CD Application should mirror the same release/application/environment identity labels**, so the handoff object itself states the same contract and later readers/operators can reconstruct identity from the deploy controller surface too.
3. Keep annotations non-authoritative for identity. Do not let runtime lookup depend on them.

This lines up with the inlined decision D006 and the runtime-service contract language already established in S01.

## Implementation Landscape

### 1. Current runtime consumers already define the minimum stable identity set

The strongest code-level consumer contract already exists in:
- `internal/runtime/observer/release_rollout.go`
- `internal/runtime/observer/kubernetes_runtime.go`
- `internal/release/domain/types.go`

Observed requirements:
- `deriveReleaseRolloutContext()` requires:
  - `devflow.io/release-id`
  - `devflow.application/id`
  - `devflow.environment/id` (or falls back to `workload.Environment` only for environment)
- `lookupDeployment()` queries Deployments by `devflow.io/release-id`
- `runtimeSpecFromDeployment()` in the Kubernetes runtime observer requires:
  - `devflow.application/id`
  - `devflow.environment/id`
  - plus `app.kubernetes.io/name` or deployment name fallback

Implication for the planner:
- The runtime consumer contract is not hypothetical. These labels are already the effective runtime schema.
- S02 should codify these as the required stable labels, not invent a different set.

### 2. Release bundle rendering already produces most of the workload-side contract

The main producer is:
- `internal/release/service/release_bundle.go`

`buildReleaseWorkloadResource()` currently injects:
- `app.kubernetes.io/name`
- `devflow.application/id`
- `devflow.environment/id`

It **does not** inject `devflow.io/release-id` into workload labels in the rendered bundle code shown here, even though runtime rollout observation requires that label later.

Important nuance: `rg` shows the release ID label is referenced in runtime consumers and on Argo Application metadata, but the rendered workload builder is the natural place to guarantee it on deployed workloads. That is the highest-risk mismatch to verify/fix first.

Planner implication:
- First task should audit and, if needed, add/assert `devflow.io/release-id` on release-rendered workload metadata and pod template metadata.
- This is the riskiest implementation seam because S05 integration proof depends on it.

### 3. Argo Application metadata is currently under-specified and under-populated

Current creation path:
- `internal/release/service/release.go`
- `buildArgoApplication()` creates the Argo object shell
- `createArgoApplication()` later assigns metadata

Current assigned metadata on the Argo `Application` object:
- annotations:
  - trace ID
  - span ID
- labels:
  - `status`
  - `devflow.io/release-id`

Missing relative to the desired runtime-consumable contract:
- `devflow.application/id`
- `devflow.environment/id`
- any explicit documentation that `status` is operational/decorative rather than identity

Planner implication:
- There is a natural seam for a task that updates `createArgoApplication()` / `buildArgoApplication()` plus `release_argo_test.go` to define the Argo-side metadata schema.
- This is the cleanest place to enforce R013 without touching transport or persistence layers.

### 4. Manifest resource rendering is inspection-only and should not become the deploy contract

`internal/manifest/service/manifest_renderer.go` injects:
- `app.kubernetes.io/name`
- `devflow.application/id`

But `docs/resources/manifest.md` explicitly says manifest resources are inspection-only and not the deployable artifact.

Planner implication:
- Do **not** treat manifest-rendered resources as the authoritative place for runtime/release contract fixes.
- If docs mention manifest-side labels for context, they should clearly say those are preview/inspection only.
- S02 changes should stay centered on release bundle rendering + Argo Application metadata + contract docs.

### 5. The docs already name the seam, so S02 should resolve it in one authoritative place

Current docs already contain the unresolved seam:
- `docs/system/flow-overview.md` explicitly says stage 7 consumes workload labels `devflow.io/release-id`, `devflow.application/id`, `devflow.environment/id`
- `docs/services/runtime-service.md` says release bundles need runtime-relevant labels such as application/environment labels
- `docs/resources/release.md` and `docs/system/release-steps.md` describe Argo handoff but do not currently define an explicit labels-vs-annotations schema table

Best doc landing zones for S02:
1. `docs/resources/release.md` — resource-level metadata contract for Argo Application + release-rendered runtime-visible resources
2. `docs/system/flow-overview.md` — update the S01 seam language from “carried forward” to explicit contract routing
3. `docs/services/runtime-service.md` — state exactly which labels runtime observers require and that annotations are non-authoritative for identity

This follows the S01 pattern: one authoritative system map plus supporting resource/service docs routing back to it.

## Natural Seams for Planning

### Seam A — Code: release-rendered workload metadata
Files:
- `internal/release/service/release_bundle.go`
- `internal/release/service/release_bundle_test.go`

Goal:
- Ensure the deployable workload metadata carries the full required stable label set.

Why first:
- Runtime observers depend on these labels directly.
- This is the highest-value producer/consumer seam for R013/R014.

Verification target:
- bundle tests that assert all required labels are present on workload metadata and pod template metadata.

### Seam B — Code: Argo Application metadata contract
Files:
- `internal/release/service/release.go`
- `internal/release/service/release_argo_test.go`
- possibly `internal/release/transport/argo/client.go` only for compatibility awareness, not necessarily code changes

Goal:
- Define and populate the Argo Application label/annotation schema.

Why second:
- It completes the handoff object contract after workload-side truth is clear.
- It directly satisfies the “Argo CD Application creation must carry a defined metadata contract” wording in R013.

Verification target:
- unit tests that assert Application labels/annotations exactly.

### Seam C — Docs: authoritative schema language
Files:
- `docs/resources/release.md`
- `docs/system/flow-overview.md`
- `docs/services/runtime-service.md`

Goal:
- Publish one explicit schema with:
  - required labels
  - optional labels
  - annotations
  - producer stage
  - downstream consumer

Why third:
- Once the code contract is settled, docs can truthfully match it.

Verification target:
- repo verifier plus targeted grep/read checks for the new schema table/section.

## Risks / Unknowns

### 1. Release ID label appears required by the rollout observer but may not be guaranteed on rendered workloads

This is the biggest implementation risk surfaced by the code read.

Evidence:
- consumer requires `devflow.io/release-id` in `internal/runtime/observer/release_rollout.go`
- bundle renderer snippet clearly sets application/environment labels, but not release ID
- Argo Application gets the release ID label, but runtime workload observers do not read Argo objects for rollout association

If this gap is real in full rendered output, S02 must close it before docs are updated, otherwise docs would describe phantom behavior.

### 2. `status` label on Argo Application should not be allowed to masquerade as identity metadata

Current code adds `status` and `devflow.io/release-id` to Argo Application labels. `status` is operational state, not identity. If S02 writes a metadata table, it should classify `status` separately as mutable/non-identity metadata.

### 3. Environment identity has mixed semantics (`environment_id` UUID-ish string vs runtime `Environment` field fallback)

`Release.EnvironmentID` is modeled as a string and docs say it currently expects a valid environment UUID string. Runtime observers fall back from label to `workload.Environment` only for environment. S02 should not broaden that fallback into a contract. The stable contract should still require `devflow.environment/id` as a label.

## What to Prove

For this slice to meaningfully unblock S03/S04/S05, proof should show:
1. Release-rendered workload objects carry the exact stable labels runtime observers require.
2. Argo CD Application creation carries the documented label/annotation contract.
3. Docs say labels are the stable lookup layer and annotations are supplementary only.
4. Tests fail if one of the required labels disappears.

Recommended verification commands:
- `go test ./internal/release/service ./internal/runtime/observer`
- `bash scripts/verify.sh`

Potential additional targeted proof:
- add/keep assertions in release bundle tests against rendered object metadata maps
- add/keep assertions in Argo application tests against `app.Labels` and `app.Annotations`

## Skill Discovery (suggest)

Installed skills directly relevant here:
- `api-design` — useful for expressing the metadata contract crisply even though this is not an external REST redesign

Promising external skills discovered but **not installed**:
- Argo CD: `npx skills add personamanagmentlayer/pcl@argocd-expert`
- Kubernetes: `npx skills add jeffallan/claude-skills@kubernetes-specialist`

Neither is required for S02 because the repo already uses established Argo/Kubernetes patterns and the slice is mainly contract alignment inside known code.

## References / Evidence

Primary code anchors:
- `internal/release/domain/types.go`
- `internal/release/service/release.go`
- `internal/release/service/release_bundle.go`
- `internal/release/service/release_bundle_test.go`
- `internal/release/service/release_argo_test.go`
- `internal/runtime/observer/kubernetes_runtime.go`
- `internal/runtime/observer/release_rollout.go`
- `internal/runtime/observer/release_rollout_test.go`

Primary doc anchors:
- `docs/system/flow-overview.md`
- `docs/resources/release.md`
- `docs/services/runtime-service.md`
- `docs/system/release-writeback.md`
- `docs/system/release-steps.md`

## Planner Notes

Build/prove in this order:
1. Confirm and enforce required workload labels in `release_bundle.go` + tests.
2. Define Argo Application metadata schema in `release.go` + `release_argo_test.go`.
3. Update `release.md` / `flow-overview.md` / `runtime-service.md` to publish one contract.
4. Verify with targeted Go tests first, then repo verifier.

Do not spend S02 on:
- runtime writeback semantics redesign (that belongs to S03)
- manifest inspection resource semantics
- persistence schema changes
- operator action flows
