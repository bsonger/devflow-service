# Runtime Release Terminal Writeback Compensation Design

## Purpose

This design defines how `runtime-service` should close the release writeback gap
when a Kubernetes workload has already reached a terminal outcome but the owning
`release-service` has not yet received the terminal rollout callbacks.

It addresses one concrete problem:

1. with `observer.release_runtime_enabled=true`, the queue/reconcile release
   runtime lane can leave a release in non-terminal callback state even though
   the workload has already converged to terminal Kubernetes truth

## Scope

This design covers:

- the queue/reconcile release-runtime lane used when
  `observer.release_runtime_enabled=true`
- terminal writeback compensation for `Deployment`
- terminal writeback compensation for Argo Rollout `canary`
- terminal writeback compensation for Argo Rollout `blueGreen`
- runtime-side idempotency for terminal callback emission
- runtime-side release-status label convergence on live workloads
- required tests and documentation updates

This design does not cover:

- a redesign of release business status ownership
- a new runtime persistent store
- a release-service read API for runtime-side confirmation
- changes to unrelated runtime read or operator APIs

## Constraints agreed during brainstorming

The user-approved constraints are:

- keep working on top of the current local queue/reconcile code path
- cover both `Deployment` and `Rollout` terminal cases
- `runtime-service` must not depend on PostgreSQL for release-runtime truth
- `runtime-service` must not recover release truth by calling
  `release-service` read APIs
- `release-service` remains the owner of release truth and callback routes
- the final change should be committed and pushed after implementation

## Current problem

When `observer.release_runtime_enabled=true`, `runtime-service` uses the
queue/reconcile lane under:

- `internal/runtime/bootstrap/release_runtime.go`
- `internal/runtime/reconcile/release_reconciler.go`

That lane already tries to discover candidate releases and write rollout step
updates back to `release-service`, but it can still miss the final callback
closure after the workload has already become terminal in Kubernetes.

The gap is most visible in this failure shape:

1. workload enters terminal success or failure
2. runtime observed state reflects that terminal outcome
3. `release-service` does not receive the expected terminal callback sequence
   for `observe_rollout` and `finalize_release`
4. the release record remains open or only partially converged even though the
   workload is already in terminal state

The current dual-source discovery improvement in
`internal/runtime/bootstrap/release_runtime.go` helps with candidate detection,
but by itself it does not guarantee that terminal callbacks will still be sent
after the workload is no longer simply "running".

## Proposed approaches

### Approach A — strengthen queue discovery only

Keep the current queue/reconcile design and only improve release candidate
discovery, for example by combining watch-driven events with a running-release
scan.

Pros:

- smallest change to current bootstrap code
- preserves the current queue lane structure

Cons:

- solves only missing-candidate cases
- does not guarantee terminal callback compensation once workload state is
  already terminal
- leaves the actual writeback closure problem in the reconciler

Recommendation: reject as insufficient on its own.

### Approach B — add terminal compensation to the reconciler

Keep the current queue/reconcile lane, but make the reconciler able to emit one
final terminal writeback sequence whenever runtime-observed Kubernetes truth is
already terminal and release callback convergence has not yet happened.

Pros:

- directly solves the reported problem
- fits the current `release_runtime_enabled=true` execution path
- works for both `Deployment` and `Rollout`
- keeps release truth ownership in `release-service`

Cons:

- requires cleaner state normalization inside runtime code
- requires stronger idempotency handling to avoid duplicate terminal callbacks

Recommendation: **accept**.

### Approach C — fall back to the legacy rollout observer

Shift terminal closure responsibility back toward the legacy polling observer
path instead of the queue/reconcile lane.

Pros:

- closer to older behavior

Cons:

- conflicts with the active `release_runtime_enabled=true` direction
- leaves two competing runtime release lanes active in practice
- increases long-term drift between old and new logic

Recommendation: reject.

## Selected design

Use **Approach B**.

## Design

### Runtime boundary

The queue/reconcile lane becomes the runtime-side terminal compensation path for
release rollout writeback.

Boundary rules:

- `release-service` remains the owner of release truth, release status, and
  callback route policy
- `runtime-service` remains a sender of runtime-observed rollout facts only
- `runtime-service` does not read release PostgreSQL state to confirm terminal
  truth
- `runtime-service` does not call a release-service read API before sending
  terminal writeback
- runtime-side decisions are made from Kubernetes metadata plus runtime
  observed workload state only

### Candidate discovery

Candidate discovery remains best-effort and can keep both sources:

- `ReleaseEventSource`
- `RunningReleaseSource`

But those sources only enqueue candidate `release_id` values.
They do not decide whether terminal writeback is still allowed.

This means:

- the bootstrap improvement in `internal/runtime/bootstrap/release_runtime.go`
  should be preserved
- correctness moves into the reconciler instead of being inferred from source
  choice alone

### Terminal compensation rule

The reconciler must allow a terminal compensation pass when all of the
following are true:

- the workload still carries complete release identity metadata:
  - `devflow.io/release-id`
  - `devflow.application/id`
  - `devflow.environment/id`
  - `devflow.control-plane/id`
- `devflow.control-plane/id` matches local `observer.control_plane_id`
- runtime observed state can derive a terminal success or failure outcome for
  that workload

The reconciler must not require `devflow.io/release-status=running` as a hard
gate before sending the terminal callback sequence.

`devflow.io/release-status=running` remains the active-release projection for
normal processing, but it must not block one last terminal compensation pass
when Kubernetes truth is already terminal.

### Unified observed-state normalization

The queue/reconcile lane should use one normalized release-observed-state
helper that returns:

- `phase`
- `progress`
- `message`
- `state_key`
- `step_writes`
- optional `finalize_write`

The normalized helper must cover:

- `Deployment`
- Argo Rollout `canary`
- Argo Rollout `blueGreen`

The helper may reuse mature state-derivation logic from
`internal/runtime/observer/release_rollout.go`, but runtime code should not
keep two divergent terminal rule sets for the same release-runtime behavior.

### Deployment terminal rules

For `Deployment` workloads:

- success when desired replicas are fully updated, ready, and available, with
  no unavailable replicas and no failure conditions such as
  `ProgressDeadlineExceeded`
- failure when deployment conditions clearly indicate rollout failure, such as
  `ProgressDeadlineExceeded` or replica failure

Output contract:

- running:
  - `observe_rollout=Running`
- success:
  - `observe_rollout=Succeeded`
  - `finalize_release=Succeeded`
- failure:
  - `observe_rollout=Failed`
  - `finalize_release=Failed`

### Rollout terminal rules

For Argo Rollout workloads:

- `canary`
  - success when rollout phase is `Healthy` or `Completed`
  - failure when rollout phase is `Degraded`, `Error`, or `Failed`
- `blueGreen`
  - success when rollout phase is `Healthy` or `Completed`
  - failure when rollout phase is `Degraded`, `Error`, or `Failed`

Output contract:

- `canary`
  - running writes the active canary step progression
  - success writes the completed canary step set and
    `finalize_release=Succeeded`
  - failure writes the failed active canary step and
    `finalize_release=Failed`
- `blueGreen`
  - running writes preview/traffic progression
  - success writes
    `deploy_preview`
    `observe_preview`
    `switch_traffic`
    `verify_active`
    and `finalize_release=Succeeded`
  - failure writes the active failed preview or rollout step and
    `finalize_release=Failed`

### Reconciler behavior

The reconciler flow should become:

1. accept a candidate `release_id`
2. resolve the observed workload from runtime-owned state
3. derive normalized observed release state from the workload kind and live
   status
4. if the normalized state is `Running`
   - send running step writeback
   - requeue after a short delay
5. if the normalized state is terminal
   - send the full terminal step writeback sequence
   - update the workload `devflow.io/release-status` label to the terminal
     release status projection
   - stop requeueing

The reconciler should not rely on a release PostgreSQL read to confirm whether
the release is still "running" before sending the terminal callback sequence.

### Label convergence

When runtime observation reaches a terminal outcome:

- runtime-service updates the live workload
  `devflow.io/release-status` label from `running` to a terminal value
- this applies to the workload object that runtime currently treats as the
  observed primary workload
- label convergence is part of the same runtime-side closure attempt as the
  terminal callback sequence

Failure handling:

- if terminal callback writeback fails, do not update the label first and claim
  closure
- if label convergence fails after the callback sequence is ready, the reconcile
  attempt should fail and retry so the lane keeps trying to close both surfaces

### Idempotency

Runtime terminal compensation must be idempotent.

Rules:

- each observed state must produce a stable `state_key`
- `Running`, `Succeeded`, and `Failed` outcomes must produce distinct
  `state_key` values
- the queue/reconcile lane must skip duplicate processing for the same
  `release_id + state_key`
- a new state transition such as `Running -> Succeeded` must still be emitted
- a duplicate terminal observation of the same state must not re-send the same
  terminal callback sequence

`release-service` terminal guards remain the final defensive layer for late
callbacks, but runtime should still suppress obvious duplicate sends locally.

### Error handling

Error rules:

- missing required release identity labels are a metadata-contract failure;
  runtime skips rather than guessing
- `404 not_found` from release writeback is treated as stale ownership or stale
  release and should stop retrying for that exact state
- temporary HTTP errors, Kubernetes read failures, or label-update failures
  should retry through the queue lane
- terminal state on the workload does not authorize runtime to mutate
  top-level release truth directly; it only authorizes callback emission

### File-level implementation targets

Primary implementation targets:

- `internal/runtime/reconcile/release_reconciler.go`
- `internal/runtime/bootstrap/release_runtime.go`

Expected supporting work:

- extract or reuse normalized rollout-state helpers from
  `internal/runtime/observer/release_rollout.go`
- keep bootstrap source combination in
  `internal/runtime/bootstrap/release_runtime.go`
- avoid maintaining separate terminal rule sets for queue/reconcile and legacy
  observer behavior

## Testing

Required tests:

- `Deployment` terminal success triggers `observe_rollout=Succeeded` and
  `finalize_release=Succeeded`
- `Deployment` terminal failure triggers `observe_rollout=Failed` and
  `finalize_release=Failed`
- `Deployment` running state still requeues and does not finalize early
- `canary` rollout success emits completed canary step writes and terminal
  finalize
- `canary` rollout failure emits failed active canary step and terminal finalize
- `blueGreen` success emits preview/traffic completion and terminal finalize
- `blueGreen` failure emits failed rollout step and terminal finalize
- terminal writeback also triggers release-status label convergence
- `404 not_found` writeback is treated as stale and does not spin forever
- duplicate terminal `state_key` values do not resend the same callback

## Documentation updates

If implementation changes runtime release-runtime behavior, update:

- `docs/services/runtime-service.md`
- `docs/services/release-service.md`
- `docs/system/release-writeback.md`
- `docs/resources/release.md`

Those updates should describe the current code truth:

- queue/reconcile lane can compensate terminal writeback when Kubernetes truth
  is already terminal
- compensation covers both `Deployment` and `Rollout`
- runtime remains a callback sender rather than the owner of release truth

## Implementation sequence

Recommended implementation order:

1. make the reconciler support terminal compensation for `Deployment`
2. extend the same normalized state handling to `Rollout canary` and
   `Rollout blueGreen`
3. remove or consolidate duplicated terminal-state logic so queue/reconcile and
   legacy observer behavior do not drift

This sequence keeps the first slice small enough to verify while still
converging to full workload-kind coverage in the same change cycle.
