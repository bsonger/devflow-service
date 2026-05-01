# Metadata surfaces audit

This document is the tracked handoff surface for metadata ownership across the current build → release → Argo CD → runtime-observer path.

It inventories the metadata surfaces currently emitted by code, the exact keys written on each surface, and the downstream code paths that consume those keys.

## Why this exists

Later slices need one repo-tracked place to answer:

- which metadata keys are actually written today
- which surfaces carry release/application/environment identity
- which consumers require labels instead of annotations
- where trace/span metadata is present only for diagnostics
- where Argo CD drift investigations should start

For release lifecycle context, read this alongside:

- `docs/resources/release.md`
- `docs/resources/metadata-drift-proof.md`
- `docs/system/flow-overview.md`
- `docs/system/release-writeback.md`
- `docs/services/release-service.md`
- `docs/services/runtime-service.md`
- `bash scripts/verify-metadata-audit.sh`

## Metadata surfaces

The current code emits metadata on four distinct surfaces:

1. release-rendered workload object metadata (`Deployment` or `Rollout` object metadata)
2. release-rendered pod-template metadata (`spec.template.metadata`)
3. Argo CD `Application` handoff metadata
4. Tekton `PipelineRun` build metadata

The key contract split is:

- **business identity** for release/runtime correlation lives on **labels**
- **trace/span correlation** lives on **annotations** when present
- runtime observers must be able to recover release/application/environment identity from labels alone

## Surface matrix

| Surface | Produced by | Identity keys written | Diagnostic keys written | Primary downstream consumers |
|---|---|---|---|---|
| Release-rendered workload object metadata | `internal/release/service/release_bundle.go` (`buildReleaseWorkloadResource`) | `app.kubernetes.io/name`, `devflow.io/release-id`, `devflow.application/id`, `devflow.environment/id` | workload snapshot annotations copied through from `manifest.WorkloadConfigSnapshot.Annotations` | `internal/runtime/observer/kubernetes_runtime.go`, `internal/runtime/observer/release_rollout.go` |
| Release-rendered pod-template metadata | `internal/release/service/release_bundle.go` (`buildReleaseWorkloadResource`) | `app.kubernetes.io/name`, `devflow.io/release-id`, `devflow.application/id`, `devflow.environment/id` | workload snapshot annotations copied through from `manifest.WorkloadConfigSnapshot.Annotations` | `internal/runtime/observer/kubernetes_runtime.go` (stores template annotations, parses restart annotation), rollout observer indirectly via persisted runtime state |
| Argo CD `Application` | `internal/release/service/release.go` (`applyReleaseApplicationMetadata`) | `status`, `app.kubernetes.io/name`, `devflow.io/release-id`, `devflow.application/id`, `devflow.environment/id` | `otel.devflow.io/trace-id`, `otel.devflow.io/parent-span-id` | Argo object handoff, operator diagnostics, future drift/debug inspection |
| Tekton `PipelineRun` | `internal/manifest/service/manifest.go` (`buildManifestPipelineRun`, `submitManifestBuild`) | `devflow.manifest/id` (label and annotation) | `otel.devflow.io/trace-id`, `otel.devflow.io/parent-span-id` | `internal/runtime/observer/tekton_manifest.go`, manifest writeback routes |

## Surface details

### 1. Release-rendered workload object metadata

Source:

- `internal/release/service/release_bundle.go` (`buildReleaseWorkloadResource`)
- label constants from `internal/release/domain/types.go`

Current object-level labels written on the rendered `Deployment` or `Rollout`:

| Key | Kind | Written by | Notes | Downstream consumption |
|---|---|---|---|---|
| `app.kubernetes.io/name` | label | `requiredLabels` in `buildReleaseWorkloadResource` | Uses the first service name when present, otherwise the application name. Also used by Service selectors. | `runtimeSpecFromDeployment` falls back to this label when reconstructing the primary app/workload name in `internal/runtime/observer/kubernetes_runtime.go`; `deriveReleaseRolloutContext` also falls back to it if workload name is absent in `internal/runtime/observer/release_rollout.go`. |
| `devflow.io/release-id` | label | `requiredLabels` in `buildReleaseWorkloadResource` | Canonical release identity on live workloads. | `labelsMatchRuntimeSpec` requires it to be non-empty in `internal/runtime/observer/kubernetes_runtime.go`; `deriveReleaseRolloutContext` parses it as mandatory release identity in `internal/runtime/observer/release_rollout.go`; `lookupDeployment` later selects deployments by this label. |
| `devflow.application/id` | label | `requiredLabels` in `buildReleaseWorkloadResource` | Canonical application identity. | Used by the runtime observer list selector and spec matching in `internal/runtime/observer/kubernetes_runtime.go`; parsed as required in `runtimeSpecFromDeployment` and `deriveReleaseRolloutContext`. |
| `devflow.environment/id` | label | `requiredLabels` in `buildReleaseWorkloadResource` | Canonical environment identity. | Used by the runtime observer selector and matching in `internal/runtime/observer/kubernetes_runtime.go`; parsed as required in `runtimeSpecFromDeployment`; rollout observer prefers the label and only falls back to persisted runtime environment if the label is absent in `internal/runtime/observer/release_rollout.go`. |
| `<workload snapshot labels>` | label | copied from `manifest.WorkloadConfigSnapshot.Labels` before required labels are overlaid | User/config-supplied labels survive unless they collide with required identity labels. | Not relied on for release/runtime identity in the current observers. |

Current object-level annotations written on the rendered `Deployment` or `Rollout`:

| Key | Kind | Written by | Notes | Downstream consumption |
|---|---|---|---|---|
| `<workload snapshot annotations>` | annotation | copied from `manifest.WorkloadConfigSnapshot.Annotations` in `buildReleaseWorkloadResource` | Release rendering preserves workload-config annotations exactly; it does **not** add release trace/span annotations here. | Not used for identity matching. They are runtime-observed and stored, but only label keys are required for correlation. |

### 2. Release-rendered pod-template metadata

Source:

- `internal/release/service/release_bundle.go` (`buildReleaseWorkloadResource`)

Current pod-template metadata:

- `spec.template.metadata.labels` reuses the same `labels` map as top-level workload metadata
- `spec.template.metadata.annotations` reuses the same `annotations` map as top-level workload metadata

That means the pod template currently carries:

| Key group | Kind | Written by | Downstream consumption |
|---|---|---|---|
| `app.kubernetes.io/name`, `devflow.io/release-id`, `devflow.application/id`, `devflow.environment/id` | labels | same `labels` map passed into `spec.template.metadata` | Pod label matching in `podMatchesRuntimeSpec` / `labelsMatchRuntimeSpec` inside `internal/runtime/observer/kubernetes_runtime.go`. |
| `<workload snapshot annotations>` | annotations | same `annotations` map passed into `spec.template.metadata` | `syncDeployment` persists `deployment.Spec.Template.Annotations` into runtime observed workload state; `parseRestartAt` specifically reads `kubectl.kubernetes.io/restartedAt` from those annotations in `internal/runtime/observer/kubernetes_runtime.go`. |

Identity requirement:

- runtime observers do **not** use pod-template annotations for release/application/environment identity
- they require the label trio `devflow.io/release-id`, `devflow.application/id`, and `devflow.environment/id`

That requirement is enforced in code by `labelsMatchRuntimeSpec` in `internal/runtime/observer/kubernetes_runtime.go`, which returns false when:

- `devflow.application/id` does not match
- `devflow.environment/id` does not match
- `devflow.io/release-id` is empty

### 3. Argo CD `Application`

Source:

- `internal/release/service/release.go` (`applyReleaseApplicationMetadata`)
- annotation names from `internal/platform/oci/image.go`

Current `Application` annotations written:

| Key | Kind | Written by | Notes | Downstream consumption |
|---|---|---|---|---|
| `otel.devflow.io/trace-id` | annotation | `applyReleaseApplicationMetadata` via `oci.TraceIDAnnotation` | Copied from the current OpenTelemetry span context. | Diagnostic correlation only; no runtime observer requires it for identity. |
| `otel.devflow.io/parent-span-id` | annotation | `applyReleaseApplicationMetadata` via `oci.SpanAnnotation` | Span identifier for the Argo handoff step. | Diagnostic correlation only. |

Current `Application` labels written:

| Key | Kind | Written by | Notes | Downstream consumption |
|---|---|---|---|---|
| `status` | label | `applyReleaseApplicationMetadata` | Set to `Running` during release dispatch handoff. | Operator/Argo diagnostics only; not rollout truth. |
| `app.kubernetes.io/name` | label | `applyReleaseApplicationMetadata` | Set to the Argo application name. | Operator diagnostics; keeps the handoff object aligned with workload identity naming. |
| `devflow.io/release-id` | label | `applyReleaseApplicationMetadata` | Mirrors the rendered workload identity label. | Intended handoff/debug identity, but current runtime observers do not read the Argo `Application` object directly. |
| `devflow.application/id` | label | `applyReleaseApplicationMetadata` | Mirrors the rendered workload identity label. | Intended handoff/debug identity. |
| `devflow.environment/id` | label | `applyReleaseApplicationMetadata` | Mirrors the rendered workload identity label. | Intended handoff/debug identity. |

Important implementation note:

- `applyReleaseApplicationMetadata` replaces the `Application` labels/annotations map with the release-owned handoff set rather than merging arbitrary pre-existing values
- the Argo client update path in `internal/release/transport/argo/client.go` copies those labels/annotations onto the current object before update

### 4. Tekton `PipelineRun`

Source:

- `internal/manifest/service/manifest.go` (`buildManifestPipelineRun`, `submitManifestBuild`)
- runtime-side consumer in `internal/runtime/observer/tekton_manifest.go`

Current `PipelineRun` metadata written at creation:

| Key | Surface | Kind | Written by | Notes | Downstream consumption |
|---|---|---|---|---|---|
| `devflow.manifest/id` | `metadata.labels` | label | `buildManifestPipelineRun` | Canonical build/manifest identity key for the Tekton run. | `TektonManifestObserver.sync` lists PipelineRuns with label selector `devflow.manifest/id`; `syncPipelineRun` reads the label; task/image/status writeback payloads use it as the manifest identifier. |
| `devflow.manifest/id` | `metadata.annotations` | annotation | `buildManifestPipelineRun` | Duplicate annotation copy of the same manifest identity. | Current runtime Tekton observer does **not** rely on the annotation; label is the active lookup surface. |
| `otel.devflow.io/trace-id` | `metadata.annotations` | annotation | `submitManifestBuild` via `oci.TraceIDAnnotation` | Set from the span context immediately before create. | Diagnostic correlation only. |
| `otel.devflow.io/parent-span-id` | `metadata.annotations` | annotation | `submitManifestBuild` via `oci.SpanAnnotation` | Set from the span context immediately before create. | Diagnostic correlation only. |

Tekton consumption notes:

- `internal/runtime/observer/tekton_manifest.go` filters PipelineRuns by the **label** `devflow.manifest/id`
- the observer ignores PipelineRuns whose label is missing even if the annotation exists
- task-run state, manifest status, and build result payloads all flow from that label-based manifest identity

## Producer/consumer takeaways

### Release/runtime identity is label-only

The current runtime observers are intentionally label-driven.

Code evidence:

- `releaseOwnedSelector` in `internal/runtime/observer/kubernetes_runtime.go` selects by `devflow.application/id` and `devflow.environment/id`
- `labelsMatchRuntimeSpec` in the same file requires:
  - `devflow.application/id`
  - `devflow.environment/id`
  - non-empty `devflow.io/release-id`
- `deriveReleaseRolloutContext` in `internal/runtime/observer/release_rollout.go` parses release/application/environment from workload **labels**, not annotations
- `lookupDeployment` then lists deployments by `devflow.io/release-id=<release>`

Contract implication:

- if a workload or pod loses the release/application/environment labels, runtime correlation and rollout writeback can break even when annotations still exist
- trace/span annotations are not substitutes for those labels

### Argo handoff mirrors the release identity contract

`applyReleaseApplicationMetadata` writes the same release/application/environment identity labels onto the Argo CD `Application` that release rendering writes onto the workload and pod-template surfaces.

That alignment means later drift-fix slices can compare:

- desired labels on release-rendered resources
- handoff labels on the Argo `Application`
- labels seen back from live workloads by runtime observers

from a single shared contract.

### Tekton uses a separate build identity contract

Tekton `PipelineRun` metadata is not part of release/runtime identity.

Instead it carries:

- `devflow.manifest/id` for build-side correlation
- `otel.devflow.io/trace-id` and `otel.devflow.io/parent-span-id` for diagnostics

This is a separate producer/consumer path from the release/workload/Argo identity labels.

## Trace/span naming drift

There is an active code/doc naming drift for trace/span annotations.

### What code writes today

Code constants in `internal/platform/oci/image.go` define:

- `otel.devflow.io/trace-id`
- `otel.devflow.io/parent-span-id`

Those exact keys are written by:

- `internal/release/service/release.go` on the Argo CD `Application`
- `internal/manifest/service/manifest.go` on the Tekton `PipelineRun`

### What `docs/resources/release.md` documented before this audit

The release doc described Argo annotation keys as:

- `devflow.io/trace-id`
- `devflow.io/span-id`

That does not match the code.

### Current contract decision

Until code changes, the live contract is:

- `otel.devflow.io/trace-id`
- `otel.devflow.io/parent-span-id`

These are **supplementary diagnostics only**, not business identity.

Rules:

- do not use trace/span annotations as release/application/environment lookup keys
- do not treat missing trace/span annotations as release identity loss
- drift/debug tooling may inspect them for correlation, but runtime ownership must still come from labels

## Contrast with manifest inspection rendering

A nearby but different metadata surface exists in manifest inspection rendering:

- `internal/manifest/service/manifest_renderer.go`

That renderer writes:

- `app.kubernetes.io/name`
- `devflow.application/id`
- image-related annotations such as `devflow.io/image-tag` and `devflow.io/image-ref`
- workload snapshot annotations

It does **not** write:

- `devflow.io/release-id`
- `devflow.environment/id`

That contrast is useful because it shows where release-time identity begins:

- manifest-owned inspection resources are build-oriented and application-scoped
- release-owned rendered resources add release/environment identity needed by runtime observers and rollout writeback

## Audit checklist for drift investigations

When investigating Argo CD drift or runtime correlation issues, inspect in this order:

1. `internal/release/service/release_bundle.go`
   - confirm the rendered workload and pod-template labels still include:
     - `app.kubernetes.io/name`
     - `devflow.io/release-id`
     - `devflow.application/id`
     - `devflow.environment/id`
2. `internal/release/service/release.go`
   - confirm `applyReleaseApplicationMetadata` still mirrors the same identity labels onto the Argo `Application`
3. `internal/runtime/observer/kubernetes_runtime.go`
   - confirm selectors and matching logic still depend on the same labels
4. `internal/runtime/observer/release_rollout.go`
   - confirm rollout writeback still derives release/application/environment from workload labels
5. `internal/manifest/service/manifest.go` and `internal/runtime/observer/tekton_manifest.go`
   - confirm build-side `devflow.manifest/id` label flow is intact for Tekton writeback

## Summary

The current metadata contract is:

- **Release-rendered workloads and pod templates** carry the authoritative release/application/environment identity labels consumed by runtime observers.
- **Argo CD `Application`** mirrors those identity labels and adds OpenTelemetry trace/span annotations for diagnostics.
- **Tekton `PipelineRun`** uses `devflow.manifest/id` as its build-side identity key and also carries OpenTelemetry trace/span annotations for diagnostics.
- **Trace/span annotations are supplementary diagnostics, not business identity.**
- **Runtime observers are label-only for identity recovery.**
