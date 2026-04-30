# Flow Overview

## Purpose

This document is the authoritative stage-routing contract for the current release lifecycle.
Use it when you need one reader-first map that answers, stage by stage:

- who owns the stage
- which inputs are consumed
- which outputs are produced
- who consumes those outputs next
- which code and docs are the strongest contract anchors

This document intentionally stays at the lifecycle-contract level.
It does not redesign the metadata schema or step semantics.
When you need exact field definitions, route payloads, or step-by-step execution semantics, follow the linked contract anchors.

## Reader outcome

After reading this file, a fresh engineer or agent should be able to:

- localize any release-lifecycle failure to a specific stage
- identify the owning service for that stage
- know which frozen record or runtime surface is authoritative there
- follow the strongest code and doc anchors into `release-service`, release writeback routes, Argo handoff, and runtime observer code paths

## End-to-end stage map

| Stage | Owner | Primary inputs | Outputs | Downstream consumer | Contract anchors |
|---|---|---|---|---|---|
| 1. Metadata resolution | `meta-service`, `config-service`, `network-service` | application metadata, environment metadata, cluster/deploy-target metadata, workload config truth, service truth, route truth | upstream metadata projections used by later freeze stages | `release-service` manifest/release creation flows | Docs: `docs/system/architecture.md`, `docs/services/release-service.md` |
| 2. Manifest freeze and build dispatch | `release-service` | metadata-stage application/workload/service truth, requested `git_revision` | persisted `Manifest`, frozen `services_snapshot`, frozen `workload_config_snapshot`, resolved `commit_hash`, Tekton build dispatch, manifest build status/writeback join keys | runtime Tekton observer, later release creation | Docs: `docs/resources/manifest.md`, `docs/services/release-service.md`; Code: `internal/manifest/service/manifest.go` |
| 3. Release freeze | `release-service` | deployable `Manifest`, target `environment_id`, app config truth, optional/deferred route truth, deploy-target metadata | persisted `Release`, frozen `app_config_snapshot`, frozen `routes_snapshot`, initialized release `steps`, release execution intent/dispatch state | release execution phases | Docs: `docs/resources/release.md`, `docs/system/release-steps.md`, `docs/services/release-service.md`; Code: `internal/release/service/release.go` |
| 4. Release bundle render | `release-service` | manifest image/workload/service snapshots, release app config snapshot, release route snapshot, strategy, target namespace/environment | canonical rendered deployment bundle persisted as release-owned bundle fact | bundle publication, bundle preview, Argo application creation | Docs: `docs/resources/release.md`, `docs/services/release-service.md`; Code: `internal/release/service/release.go`, `internal/release/service/release_bundle.go` |
| 5. Bundle publish | `release-service` | rendered release bundle, registry config (`manifest_registry` legacy naming) | `artifact_repository`, `artifact_tag`, `artifact_digest`, `artifact_ref` on `Release` | Argo CD application source, release detail readers | Docs: `docs/resources/release.md`, `docs/services/release-service.md`; Code: `internal/release/service/release.go` |
| 6. Release execution handoff / Argo deployment | `release-service` initiates, Argo CD executes | published release OCI artifact, deploy target metadata, Argo application config | Argo CD `Application`, sync request, external deployment handoff, release step/status updates for dispatch-owned phases | Argo CD controllers, release writeback senders | Docs: `docs/resources/release.md`, `docs/system/release-steps.md`, `docs/system/release-writeback.md`; Code: `internal/release/service/release.go` |
| 7. Runtime observation and release writeback | `runtime-service` observes; `release-service` owns release truth and callback surface | Kubernetes workload state, runtime observer/index state, release/app/environment labels on workloads, release writeback config/token | runtime observed workload/pod state, release rollout step callbacks, release terminal progress/failure updates | runtime readers/operators, release detail readers | Docs: `docs/services/runtime-service.md`, `docs/system/release-writeback.md`, `docs/resources/release.md`; Code: `internal/runtime/observer/release_rollout.go`, `internal/runtime/config/config.go`, `internal/release/transport/http/release_writeback.go` |
| 8. Runtime operator actions | `runtime-service` | operator request, runtime observed workload identity, Kubernetes API | pod delete / rollout restart mutations and refreshed runtime-observed state | operators, runtime UI/readers | Docs: `docs/services/runtime-service.md`, `docs/resources/runtime-spec.md`; Code: `internal/runtime/transport/http`, `internal/runtime/service/service.go` |

## Stage-by-stage contract

### Stage 1. Metadata resolution

**Owner:** `meta-service`, `config-service`, `network-service`

**Inputs:**
- application metadata from `meta-service`
- environment / cluster / deploy-target metadata from `meta-service`
- workload config truth from `config-service`
- service topology truth from `network-service`
- route topology truth from `network-service`

**Outputs:**
- upstream metadata projections that later stages freeze into release-owned records

**Downstream consumer:**
- `release-service` manifest creation and release creation flows

**Contract anchors:**
- Doc: `docs/system/architecture.md`
- Doc: `docs/services/release-service.md`

**Boundary rule:**
- This stage does **not** create a release-owned durable freeze record.
- It provides source-of-truth inputs that later freeze points must snapshot.

### Stage 2. Manifest freeze and build dispatch

**Owner:** `release-service`

**Inputs:**
- application identity and repository metadata from `meta-service`
- workload config truth from `config-service`
- service topology truth from `network-service`
- requested `git_revision`

**Outputs:**
- persisted build-side `Manifest`
- frozen `services_snapshot`
- frozen `workload_config_snapshot`
- resolved immutable `commit_hash`
- Tekton build dispatch metadata such as `pipeline_id`, `trace_id`, `span_id`
- manifest status and step writeback join points for runtime observers

**Downstream consumer:**
- runtime Tekton observer path for ongoing build progress
- release creation flow once manifest reaches deployable state

**Contract anchors:**
- Doc: `docs/resources/manifest.md`
- Doc: `docs/services/release-service.md`
- Code: `internal/manifest/service/manifest.go`

**Boundary rule:**
- `Manifest` is the build-side freeze point.
- It owns build identity and image-delivery trace.
- It does **not** own environment-specific deploy inputs, release bundle publication, or rollout truth.

### Stage 3. Release freeze

**Owner:** `release-service`

**Inputs:**
- deployable persisted `Manifest`
- `environment_id`
- app config truth from `config-service`
- optional/deferred route truth from `network-service`
- deploy-target metadata from `meta-service`

**Outputs:**
- persisted deploy-side `Release`
- frozen `app_config_snapshot`
- frozen `routes_snapshot`
- chosen rollout `strategy`
- initialized release `steps`
- create/dispatch state for release execution

**Downstream consumer:**
- release execution phases that render, publish, and hand off deployment

**Contract anchors:**
- Doc: `docs/resources/release.md`
- Doc: `docs/system/release-steps.md`
- Doc: `docs/services/release-service.md`
- Code: `internal/release/service/release.go`

**Boundary rule:**
- `Release` is the deploy-side freeze point.
- It consumes `Manifest`; it does not replace it.

### Stage 4. Release bundle render

**Owner:** `release-service`

**Inputs:**
- `manifest.image_ref`
- `manifest.services_snapshot`
- `manifest.workload_config_snapshot`
- `release.app_config_snapshot`
- optional/deferred `release.routes_snapshot`
- rollout `strategy`
- target namespace and deploy target

**Outputs:**
- one canonical rendered deployment bundle fact for the release
- persisted release bundle preview source (`bundle.yaml`, rendered objects, summary fields)

**Downstream consumer:**
- bundle publication
- Argo application creation
- release bundle preview readers

**Contract anchors:**
- Doc: `docs/resources/release.md`
- Doc: `docs/services/release-service.md`
- Code: `internal/release/service/release.go`
- Code: `internal/release/service/release_bundle.go`

**Boundary rule:**
- The rendered deployment bundle belongs to `Release`, not `Manifest`.
- Manifest resource inspection views are not the deployable artifact.

### Stage 5. Bundle publish

**Owner:** `release-service`

**Inputs:**
- persisted rendered release bundle
- registry publication config from the legacy-named `manifest_registry` block

**Outputs:**
- `artifact_repository`
- `artifact_tag`
- `artifact_digest`
- `artifact_ref`
- `publish_bundle` step progress/message on the `Release`

**Downstream consumer:**
- Argo CD application source configuration
- release detail readers and diagnostics

**Contract anchors:**
- Doc: `docs/resources/release.md`
- Doc: `docs/services/release-service.md`
- Code: `internal/release/service/release.go`

**Boundary rule:**
- The legacy config name `manifest_registry` does **not** change ownership.
- The published OCI deployment bundle is release-owned output, not manifest-owned output.

### Stage 6. Release execution handoff / Argo deployment

**Owner:** `release-service` initiates the handoff; Argo CD owns ongoing deployment execution

**Inputs:**
- published release artifact reference
- release strategy and target metadata
- Argo application spec derived from the release

**Outputs:**
- Argo CD `Application`
- sync request / deployment start handoff
- release-owned dispatch-step updates such as `create_argocd_application` and strategy-specific deployment-start steps

**Downstream consumer:**
- Argo CD controllers in cluster
- callback senders that report rollout progress back into `release-service`

**Contract anchors:**
- Doc: `docs/resources/release.md`
- Doc: `docs/system/release-steps.md`
- Doc: `docs/system/release-writeback.md`
- Code: `internal/release/service/release.go`

**Boundary rule:**
- `release-service` owns deployment initiation and the durable release record.
- It does **not** own long-running rollout truth by polling Argo in normal detail reads.
- Post-handoff progress returns through release-owned writeback routes.

### Stage 7. Runtime observation and release writeback

**Owner:**
- `runtime-service` owns runtime observation and runtime read state
- `release-service` owns release truth and the callback/writeback surface

**Inputs:**
- Kubernetes workload state
- runtime observer/index state
- workload metadata labels including:
  - `devflow.io/release-id`
  - `devflow.application/id`
  - `devflow.environment/id`
- release writeback base URL and shared observer token when configured

**Outputs:**
- runtime observed workload and pod summaries
- token-gated release step callbacks
- release rollout progress and terminal status updates persisted on `Release`

**Downstream consumer:**
- runtime readers/operators
- release detail readers
- future debugging/inspection of rollout state

**Contract anchors:**
- Doc: `docs/services/runtime-service.md`
- Doc: `docs/system/release-writeback.md`
- Doc: `docs/resources/release.md`
- Code: `internal/runtime/observer/release_rollout.go`
- Code: `internal/runtime/config/config.go`
- Code: `internal/release/transport/http/release_writeback.go`

**Boundary rule:**
- `runtime-service` is an active clustered callback sender when in-cluster config and release writeback wiring are present.
- `runtime-service` does **not** own release truth.
- `release-service` remains the owner of release state, callback routes, and normalized rollout status persistence.

### Stage 8. Runtime operator actions

**Owner:** `runtime-service`

**Inputs:**
- operator request
- runtime observed workload identity
- Kubernetes API reachability

**Outputs:**
- pod delete and rollout/restart mutations against Kubernetes
- refreshed runtime-observed state after controllers reconcile

**Downstream consumer:**
- human operators
- runtime UI/readers

**Contract anchors:**
- Doc: `docs/services/runtime-service.md`
- Doc: `docs/resources/runtime-spec.md`
- Code: `internal/runtime/transport/http`
- Code: `internal/runtime/service/service.go`

**Boundary rule:**
- Runtime actions mutate live Kubernetes state.
- They do not redefine build-side or deploy-side release truth.

## Freeze points and ownership summary

| Freeze / truth boundary | Authoritative owner | Answers |
|---|---|---|
| Build-side freeze | `Manifest` in `release-service` | what was built, from which commit, with which workload/service snapshots, and what image/build status resulted |
| Deploy-side freeze | `Release` in `release-service` | what was deployed, to which environment, with which app config/routes/strategy, and which deployment artifact was handed to Argo |
| Runtime read model | `runtime-service` observer/index state | what is running now, which pods/workloads are observed, and what operators can mutate |
| Rollout callback contract | `release-service` writeback routes | how rollout progress/failure is written back after deployment handoff |

## Metadata seam carried forward for later slices

The current lifecycle has an explicit metadata seam that this document records but does **not** resolve.

### What runtime rollout observation consumes

Runtime rollout observation currently associates workload state back to a release by consuming workload labels:

- `devflow.io/release-id`
- `devflow.application/id`
- `devflow.environment/id`

Code anchors:
- `internal/runtime/observer/release_rollout.go`
- `internal/release/domain/release.go` (label constants referenced by the observer)

### Why this is a seam

Current production points differ across the lifecycle:

- workload rendering injects release/application/environment labels into rendered workload metadata during release bundle construction
- Argo CD `Application` creation is a separate handoff stage with its own metadata production surface
- runtime rollout observation depends on those workload labels being present and consistent after deployment lands in Kubernetes

This means the release-to-runtime association contract currently spans more than one production point rather than being unified behind one formally documented metadata source.

### Scope decision for this slice

This task records that seam as current reality.
It does **not** redesign label production or step semantics here.
Treat the seam as explicit follow-up contract work for:

- S02
- S04
- S05

## Runtime-consumable metadata contract

The release-to-runtime seam is now one explicit contract rather than an implied convention.

### Labels vs annotations

- **Labels** are the authoritative runtime-consumable identity surface.
- **Annotations** are supplementary tracing context and must not be required to reconstruct release ownership.

| Field | Surface | Kind | Produced by | Stage | Downstream consumer | Why it exists |
|---|---|---|---|---|---|---|
| `app.kubernetes.io/name` | rendered workloads, pod templates, Argo CD `Application` | label | `release-service` | stages 4 and 6 | `runtime-service` workload lookup, operator diagnostics | Stable application/workload name for selectors and fallback deployment-name recovery. |
| `devflow.io/release-id` | rendered workloads, pod templates, Argo CD `Application` | label | `release-service` | stages 4 and 6 | `runtime-service` rollout observer, `release-service` writeback correlation | Canonical release identity for joining live rollout state back to one `Release`. |
| `devflow.application/id` | rendered workloads, pod templates, Argo CD `Application` | label | `release-service` | stages 4 and 6 | `runtime-service` observed workload/pod indexing | Canonical application identity for runtime ownership reconstruction. |
| `devflow.environment/id` | rendered workloads, pod templates, Argo CD `Application` | label | `release-service` | stages 4 and 6 | `runtime-service` observed workload/pod indexing and rollout observer fallback | Canonical environment identity for runtime ownership reconstruction in shared clusters. |
| `status` | Argo CD `Application` | label | `release-service` | stage 6 | operator diagnostics | Dispatch-state hint on the handoff object; not rollout truth. |
| `devflow.io/trace-id` | Argo CD `Application` | annotation | `release-service` | stage 6 | trace/debug tooling | Supplementary tracing context for the Argo handoff step. |
| `devflow.io/span-id` | Argo CD `Application` | annotation | `release-service` | stage 6 | trace/debug tooling | Supplementary tracing context for the Argo handoff step. |

Boundary rule:

- stage 4 and stage 6 must agree on the identity labels above
- stage 7 consumers may use either rendered workloads or the Argo CD `Application` as an inspection surface
- stage 7 consumers must not require annotations to recover release/application/environment identity

## Failure routing by stage

If the problem looks like this, start here:

- metadata looks wrong before any freeze point
  - `docs/system/architecture.md`
  - `docs/services/release-service.md`
- build did not start, build progress is wrong, or manifest status/image result is wrong
  - `docs/resources/manifest.md`
  - `docs/services/release-service.md`
  - `internal/manifest/service/manifest.go`
- release create froze the wrong environment/config/routes or refuses to deploy a manifest
  - `docs/resources/release.md`
  - `docs/system/release-steps.md`
  - `internal/release/service/release.go`
- bundle preview or rendered deployment YAML looks wrong
  - `docs/resources/release.md`
  - `internal/release/service/release.go`
  - `internal/release/service/release_bundle.go`
- published artifact metadata is wrong or missing
  - `docs/resources/release.md`
  - `docs/services/release-service.md`
  - `internal/release/service/release.go`
- Argo handoff or release callback routing looks wrong
  - `docs/system/release-writeback.md`
  - `docs/system/release-steps.md`
  - `internal/release/transport/http/release_writeback.go`
- rollout progress is stale, duplicated, or not reaching the release record
  - `docs/system/release-writeback.md`
  - `docs/services/runtime-service.md`
  - `internal/runtime/observer/release_rollout.go`
  - `internal/runtime/config/config.go`
- runtime page data or operator actions look wrong
  - `docs/services/runtime-service.md`
  - `docs/resources/runtime-spec.md`
  - `internal/runtime/transport/http`

## Contract anchor index

### Docs
- `docs/system/architecture.md`
- `docs/resources/manifest.md`
- `docs/resources/release.md`
- `docs/resources/runtime-spec.md`
- `docs/services/release-service.md`
- `docs/services/runtime-service.md`
- `docs/system/release-writeback.md`
- `docs/system/release-steps.md`

### Code
- `internal/manifest/service/manifest.go`
- `internal/release/service/release.go`
- `internal/release/service/release_bundle.go`
- `internal/release/transport/http/release_writeback.go`
- `internal/runtime/observer/release_rollout.go`
- `internal/runtime/config/config.go`
