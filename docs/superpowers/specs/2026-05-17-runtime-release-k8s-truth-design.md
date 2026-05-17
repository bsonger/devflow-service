# Runtime Release Kubernetes Truth Design

## Purpose

This design defines how `runtime-service` should remove its remaining
PostgreSQL dependency from the release-runtime observation path.

It addresses one concrete problem:

1. `runtime-service` currently has a queue-driven release-runtime path that can
   still depend on `release` PostgreSQL storage to determine whether a release
   is running

The design goal is to make `runtime-service` depend only on Kubernetes
workload truth for runtime-side release observation, while keeping
`release-service` as the owner of release business truth.

## Scope

This design covers:

- the runtime-service release-runtime truth source
- how `release status` is projected onto Kubernetes workloads
- how informer bootstrap and event handling should filter release workloads
- which runtime-side PostgreSQL-dependent code should be removed
- which release-service workload metadata contract should be added
- required tests and documentation updates

This design does not cover:

- a broader redesign of release business status transitions
- a new release-service HTTP read API for runtime-service
- changing runtime read/action APIs unrelated to release-runtime observation
- introducing a new persistent runtime index store

## Constraints agreed during brainstorming

The user-approved constraints are:

- `runtime-service` must not depend on PostgreSQL
- `runtime-service` must not recover release truth by calling
  `release-service` HTTP
- `runtime-service` should trust Kubernetes workload metadata and live state
  only
- informer bootstrap and follow-up event handling should filter by
  `control_plane_id`
- release activity filtering should be driven by a workload label projection of
  release status

## Current problem

The current queue-driven release-runtime path under
`internal/runtime/bootstrap/release_runtime.go` still constructs a
`releaserepo.NewPostgresStore()` and uses release table reads to:

- list candidate running releases
- confirm that a release is still in `running` status before reconcile

That violates the current runtime contract documented in:

- `docs/services/runtime-service.md`
- `docs/system/runtime-storage-model.md`
- `docs/system/current-service-extraction-reality.md`

Those documents already define the active runtime-domain path as
PostgreSQL-free.

## Proposed approaches

### Approach A — Kubernetes label projection as runtime truth

`release-service` projects release activity onto workload labels.
`runtime-service` consumes only workload labels plus live rollout status.

Concretely:

- `release-service` writes `devflow.io/release-status=running` when creating or
  updating active release workloads
- `runtime-service` watches workloads and only processes workloads whose
  `devflow.control-plane/id` matches local config and whose
  `devflow.io/release-status=running`

Pros:

- fully removes runtime-side PostgreSQL dependency
- matches the user-approved “trust Kubernetes only” rule
- keeps release truth ownership in release-service while exposing a Kubernetes
  projection for runtime use
- aligns with the existing informer/watch-driven runtime direction

Cons:

- requires a stronger workload metadata contract
- if release-service fails to project the label, runtime-service will miss that
  workload

Recommendation: **accept**.

### Approach B — Runtime-side heuristic from labels without release-status

`runtime-service` treats any workload with release identity labels as active,
without any explicit `release-status` projection.

Pros:

- smaller release-service change
- fastest implementation

Cons:

- too ambiguous once completed or stale workloads remain in cluster
- runtime-service would be forced to guess business activity from infrastructure
  state alone
- does not satisfy the requirement to filter by release activity

Recommendation: reject.

### Approach C — Runtime-side local active-release index

Introduce a runtime-owned active-release index maintained from workload events,
then reconcile from that index.

Pros:

- clean future extensibility
- can support richer event reduction later

Cons:

- larger design surface than needed for this change
- adds new runtime-owned structures before the truth source is simplified

Recommendation: reject for now.

## Selected design

Use **Approach A**.

## Design

### Truth-source contract

For the release-runtime observation path, `runtime-service` should derive its
candidate release set only from Kubernetes workloads that satisfy all of the
following:

- workload carries `devflow.io/release-id`
- workload carries `devflow.application/id`
- workload carries `devflow.environment/id`
- workload carries `devflow.control-plane/id`
- workload carries `devflow.io/release-status=running`

`runtime-service` should not call PostgreSQL or a downstream HTTP API to
confirm whether that release is still running.

The business meaning is:

- `release-service` owns release truth
- `release-service` projects the subset of truth needed by runtime into
  workload labels
- `runtime-service` owns observation of the projected running workload and its
  live rollout condition

### Release-service responsibilities

`release-service` should treat `devflow.io/release-status` as part of the
rendered workload metadata contract.

At minimum for this slice:

- active workloads created for an in-progress release must include
  `devflow.io/release-status=running`
- the label must be present on the workload object metadata and the pod template
  metadata where release identity labels are already projected

This slice does not require a full multi-status workload projection model.
It only requires `running` to be projected correctly so runtime observers can
filter active release workloads without consulting release storage.

### Runtime-service responsibilities

`runtime-service` should remove all release-runtime reads that depend on
`releaserepo.NewPostgresStore()` or release table lookups.

The release-runtime queue path should become:

1. informer bootstrap lists workloads already present in the cluster
2. bootstrap keeps only workloads whose `control_plane_id` matches local config
3. bootstrap keeps only workloads whose `devflow.io/release-status=running`
4. event handlers apply the same filter for add/update/delete events
5. reconciler reconstructs release identity only from workload labels and
   runtime observed state
6. reconciler writes runtime observation back to release-service without first
   reading release business state from storage

If a workload is missing any required label, runtime-service should skip it.
It must not guess missing release identity or infer running status from partial
data.

### Reconciler behavior

The release reconciler should be driven by runtime-owned observation data only.

That means:

- `GetRunningRelease` should no longer call the release repository
- the reconciler input should be reconstructed from workload labels and
  runtime-store-observed workload state
- top-level release writeback should describe runtime observation only:
  workload kind, workload name, rollout phase, progress, and step statuses

This keeps the boundary clean:

- runtime-service reports observed rollout/runtime facts
- release-service decides how those facts affect final release truth

### Informer bootstrap and event filtering

Informer bootstrap and steady-state event handling should use the same filter
logic.

Accepted workload event:

- `devflow.control-plane/id` equals local `observer.control_plane_id`
- `devflow.io/release-status=running`
- release/application/environment identity labels are all present

Rejected workload event:

- missing control-plane label
- control-plane mismatch
- missing release-status label
- release-status not equal to `running`
- missing release/application/environment identity

This prevents runtime-service from processing unrelated workloads and avoids
reintroducing release-store-based filtering.

### Error handling

This design changes the failure model:

- missing or stale workload labels are now a release metadata contract failure,
  not a reason for runtime-service to fall back to PostgreSQL
- runtime-service should fail closed by skipping invalid workloads
- tests and docs should make clear that an unlabeled or incorrectly labeled
  workload is invisible to release-runtime observation

### Testing

This change should add or update tests for:

- release-service workload rendering includes `devflow.io/release-status=running`
- release-runtime informer bootstrap includes only matching control-plane and
  running-status workloads
- release-runtime event handling skips workloads without the running-status label
- release-runtime reconcile no longer depends on a release PostgreSQL store
- runtime-service startup and queue-driven reconcile still succeed without any
  PostgreSQL initialization

The runtime verification surface should continue to protect the
PostgreSQL-free runtime-domain rule.

## Implementation slices

This design should be implemented in the following order:

1. add and test workload label projection in release-service
2. remove runtime-side release PostgreSQL reads and switch release-runtime
   filtering to workload labels
3. update docs and verification expectations to describe the new truth-source
   contract

## Documentation updates required

At minimum update:

- `docs/services/runtime-service.md`
- `docs/system/runtime-storage-model.md`
- `docs/system/current-service-extraction-reality.md`
- any release-resource or release-service doc that describes workload metadata
  projection or runtime writeback expectations

The updated docs should explicitly say that:

- runtime-service release-runtime observation trusts Kubernetes workload labels
  and live state only
- `devflow.io/release-status` is part of the workload metadata contract
- runtime-service does not query PostgreSQL or release-service HTTP to confirm
  running release state

## Risks

- if release-service fails to project `devflow.io/release-status`, runtime will
  miss release observation for that workload
- if workload labels drift from release truth, runtime observation can continue
  longer than intended
- if only workload metadata is updated but pod-template metadata is not, some
  downstream runtime identity reconstruction paths can become inconsistent

These risks are acceptable for this slice because they are explicit contract
failures and are easier to diagnose than a hidden runtime-side database
dependency.

## Non-goals for this slice

This design intentionally does not add:

- a full workload label projection matrix for every release terminal state
- a new runtime-owned durable release cache
- downstream runtime reads from release-service
- fallback logic from Kubernetes truth back to PostgreSQL

## Acceptance criteria

This design is complete when all of the following are true:

- `runtime-service` release-runtime path no longer imports or constructs a
  release PostgreSQL store
- runtime release candidate selection is based on Kubernetes workload labels and
  runtime observed state only
- `release-service` projects `devflow.io/release-status=running` into active
  workloads
- runtime-service release observation works without PostgreSQL initialization
- current runtime docs consistently describe this Kubernetes-only truth-source
  contract
