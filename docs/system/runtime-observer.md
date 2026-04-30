# Runtime Observer / Index Model

## Purpose

This document explains the active runtime read-model contract for workload and pod display.

Start with `docs/system/flow-overview.md` when you need the full release-lifecycle ownership map.
Use this document for the runtime-owned read model and for the runtime side of the release-to-runtime metadata seam.

Use it to answer:

- where runtime page data should come from
- which parts call Kubernetes directly
- what the active runtime read path is in pre-production
- which release-owned metadata labels runtime-service consumes from Kubernetes objects

## Core rule

Runtime display should follow one split:

- read model: observer/index-backed runtime records
- action model: direct Kubernetes mutations only when the user explicitly performs an action

That means:

- workload overview does not query Kubernetes on every page load
- pod list does not query Kubernetes on every page load
- delete pod and restart workload still call Kubernetes
- `runtime/workload` and `runtime/pods` must fail when the observer/index cannot truthfully identify exactly one runtime target for the requested `application + environment`

The active contract is truth over optimism:

- no read-path fallback to direct Kubernetes reads
- no fabricated runtime spec or empty pod list when observer state is missing
- no silent first-match selection when workload correlation is ambiguous

## Read-model resources

Runtime-service currently maintains and serves:

- `RuntimeObservedWorkload`
- `RuntimeObservedPod`
- `RuntimeOperation`

The workload overview is controller-level state.
The pod list is instance-level state.

Important implementation note:

- this read model is currently kept in-process inside `runtime-service`
- it is rebuilt by observers after restart rather than being loaded from PostgreSQL at boot
- the active recovery path depends on release-owned Kubernetes labels, with this stable contract:
  - `app.kubernetes.io/name`
  - `devflow.io/release-id`
  - `devflow.application/id`
  - `devflow.environment/id`
- annotations are supplementary only and must not be required to recover release, application, or environment identity from live cluster state

## Runtime-consumable metadata contract

The runtime observer path consumes release-owned workload metadata rather than runtime-local release truth.
That contract is shared with `docs/system/flow-overview.md`, `docs/resources/release.md`, and `docs/services/runtime-service.md`.

Stable required labels:

- `app.kubernetes.io/name` — stable workload/application name and deployment-name fallback
- `devflow.io/release-id` — canonical release identity for rollout callback correlation
- `devflow.application/id` — canonical application identity for runtime ownership reconstruction
- `devflow.environment/id` — canonical environment identity for shared-cluster ownership reconstruction

Rules:

- labels above are the authoritative runtime-consumable identity surface
- workload and pod correlation require matching `devflow.application/id` and `devflow.environment/id` plus a non-empty `devflow.io/release-id`
- `app.kubernetes.io/name` is a naming hint and deployment-name fallback, not a substitute for the release-owned identity labels
- annotations are supplementary only; they may carry trace or restart context but must not be required for identity recovery
- `runtime-service` may send rollout callbacks into `release-service`, but it does not own release truth
- `release-service` remains the owner of release state, release steps, and terminal rollout persistence

## Public runtime read surface

Shared ingress routes:

- `GET /api/v1/runtime/workload?application_id=...&environment_id=...`
- `GET /api/v1/runtime/pods?application_id=...&environment_id=...`

Frontend usage model:

- call `runtime/workload` for the summary card and conditions
- call `runtime/pods` for the pod table
- refresh both after any explicit runtime action succeeds

Read failure contract:

- `404 not_found` means runtime observer/index identity is missing for that `application + environment`, or no observed workload/pods currently exist for the resolved runtime target
- `412 failed_precondition` means the runtime target exists conceptually, but lookup could not be resolved truthfully, such as unresolved namespace or ambiguous workload correlation
- these failures are diagnostic surfaces, not cues for the caller to retry by querying Kubernetes directly

## Internal observer write surface

Observer callbacks:

- `POST /api/v1/internal/runtime-workloads/sync`
- `POST /api/v1/internal/runtime-workloads/delete`
- `POST /api/v1/internal/runtime-pods/sync`
- `POST /api/v1/internal/runtime-pods/delete`

These routes are runtime-owned index write APIs, not user-facing APIs.

Authentication note:

- these routes use `X-Devflow-Observer-Token` when `observer.shared_token` is configured
- if `observer.shared_token` is empty, the middleware allows the request through

## Current implementation status

- runtime-service supports workload overview reads
- runtime-service supports internal workload summary sync
- pre-production runtime observation is now owned by the in-process Kubernetes observer inside `runtime-service`
- the default runtime HTTP path is memory-backed and rebuilt by observer sync
- runtime-service active/runtime-domain storage is PostgreSQL-free
- release rollout observation is also started by the active runtime startup path and consumes the in-memory runtime observer state plus Kubernetes labels
- the observer startup path in `internal/runtime/config/config.go` starts rollout callbacks only when in-cluster config and release writeback wiring are available
- those rollout callbacks currently update release-owned steps such as `observe_rollout` and `finalize_release`
- shared platform startup outside `cmd/runtime-service` may still open PostgreSQL for other services

For the storage boundary, see `docs/system/runtime-storage-model.md`.

## Operator mental model

When a user opens the runtime page, think:

1. read latest workload summary from runtime index
2. read latest pod list from runtime index
3. when the user clicks restart or delete, call Kubernetes through runtime-service
4. refresh workload + pods from runtime index after the action

When runtime-service restarts, think:

1. observers rescan live Kubernetes resources
2. runtime-service reconstructs `application + environment` ownership from release-owned workload labels
3. rollout observation can re-derive release correlation from `devflow.io/release-id` plus the same observed workload metadata
4. workload and pod state is repopulated into the in-process runtime index

When lookup fails, think:

1. check whether observer-owned runtime identity exists for the requested `application + environment`
2. check whether namespace resolution failed before the read/action could target Kubernetes truthfully
3. check whether workload labels drifted or multiple release-owned Deployments still match
4. do not bypass the observer/index contract with ad-hoc direct Kubernetes reads in product code

## Source pointers

- runtime service behavior: `docs/services/runtime-service.md`
- runtime API contract: `docs/resources/runtime-spec.md`
- frontend runtime page rules: `docs/resources/frontend-ui.md`
