# Release Writeback

## Purpose

This document defines the current repo-local operational contract for `release-service` writeback and observer callbacks.
It is the owning doc for token-gated writeback routes such as release step progress updates and artifact writebacks.

Reader routing:

- start with `docs/system/flow-overview.md` for the authoritative stage map
- use `docs/system/release-steps.md` for the meaning of each release step
- use this document only for the post-handoff callback/writeback surface, primarily the release-owned side of stage 7

This document does not redefine release resource ownership or runtime read-model ownership; it explains how progress returns to the release-owned record after Argo handoff.

## Current Control-Plane Topology

Current steady-state topology:

- pre-production creates and owns build-side `Manifest`
- pre-production `release-service` owns pre-production `Release` records
- production `release-service` owns production `Release` records
- production `release-service` may read missing manifests from the configured manifest source fallback
- production runtime writeback targets production `release-service` directly

Ownership rule:

- the control plane that owns the deploy-side target owns the release record and callback surface
- workload identity and observer routing use `devflow.control-plane/id`
- `devflow.control-plane/id` comes from explicit `observer.control_plane_id` config on both `release-service` and `runtime-service`
- do not infer control-plane ownership from namespace names inside handlers or observers
- a writeback `404 not_found` should be treated first as an ownership or stale-release problem

## Current owner

- owning service boundary: `docs/services/release-service.md`
- owning HTTP package: `internal/release/transport/http`
- owning config injection: `internal/release/config/config.go`

## API surfaces

Service-internal writeback routes:

```text
POST /api/v1/verify/argo/events
POST /api/v1/verify/release/steps
POST /api/v1/verify/release/artifact
```

Pre-production shared ingress external routes:

```text
POST /api/v1/release/verify/argo/events
POST /api/v1/release/verify/release/steps
POST /api/v1/release/verify/release/artifact
```

These routes are registered by:

```text
internal/release/transport/http/router.go
internal/release/transport/http/release_writeback.go
```

## Authentication contract

Writeback routes are protected by the observer-token middleware:

```text
internal/release/transport/http/writeback_support.go
```

Accepted headers:

- `X-Devflow-Observer-Token`
- `X-Devflow-Verify-Token`

Validation rules:

- if `observer.shared_token` is empty, the middleware allows the request through
- if `observer.shared_token` is set, one of the accepted headers must match it exactly
- token mismatch returns `401 unauthorized`

The shared token is injected from release-service config:

```text
observer.shared_token
```

The current config binding lives at:

```text
internal/release/domain/config.go
internal/release/config/config.go
```

## Route behavior

### `POST /api/v1/verify/argo/events`

Purpose:
- update release-level status from Argo-side phase callbacks

Current implementation note:

- this route still exists for Argo-side status callbacks
- the active `runtime-service` startup path in `internal/runtime/config/config.go` now starts the in-tree release rollout observer when in-cluster config is available
- that observer derives rollout association from runtime observer state and Kubernetes workload labels, then writes back to `release-service`
- treat this route as part of the release-owned callback surface after stage 6 Argo handoff; callers may include Argo-facing senders and runtime-side observers, but `release-service` remains the owner of normalized release status persistence
- docs should describe this as an active observer callback path, not as a PostgreSQL-backed runtime store path
- release/application/environment metadata compatibility spans both `Deployment` and `Rollout` handoff objects, and the active in-tree runtime rollout observer now derives live progress from the observed workload kind
- for rolling releases, Argo-side step updates from this route normalize only onto callback-owned rollout confirmation steps such as `observe_rollout`; they must not reopen or advance release-owned handoff steps such as `start_deployment`
- a `running` callback means the rollout observer or Argo sender has accepted and reported progress, not that the full release graph has converged; release status stays `Running` until the remaining canonical release-owned steps also converge

Expected behavior:
- request body must include a valid `release_id`
- invalid payload or malformed `release_id` returns `400 invalid_argument`
- unknown release returns `404 not_found`
- known Argo phases are normalized onto release status values
- successful processing returns `204 no content`

Current status mapping:
- `succeeded` -> `Succeeded`
- `failed` -> `Failed`
- `error` -> `SyncFailed`
- `running` -> `Running`
- these callbacks are expected to come from a rollout observer, not from normal end-user traffic
- these callbacks should also advance `observe_rollout` step state with normalized messages such as:
  - `rollout is running in argocd`
  - `rollout observed as succeeded by argocd`

### `POST /api/v1/verify/release/steps`

Purpose:
- update release step status and progress from external execution callbacks

Current primary use:

- external executors or observers may use this route for ongoing release step progression
- the in-tree runtime rollout observer is one active caller when `runtime-service` starts with in-cluster config and release-service writeback configuration
- in production, that runtime observer writes directly to production `release-service`
- this remains a token-gated release-owned callback surface rather than a public user-facing route or a runtime-owned API
- the stable `step_code` set is the compatibility boundary, and each code has one advancing owner even when multiple components can report facts into release-service
- for workload-kind questions, separate metadata compatibility from live observation scope: release-owned labels, Argo `Application` metadata, and inspection helpers intentionally support both `Deployment` and `Rollout` primary workloads, and the active in-tree runtime rollout observer now advances the canonical step set according to the observed workload strategy
- the active runtime bootstrap still polls only running releases, but when a release disappears from that running set it replays the same release key once more; that replay is the intended last-chance compensation path for terminal callback-owned writes
- this compensation replay may re-send `observe_rollout` and `finalize_release` from observed workload truth, but it must not reopen `start_deployment` or any other release-owned dispatch step
- once `finalize_release` closes a release, late compensated callbacks must not rewrite top-level terminal truth or overwrite already-finalized callback-owned step details
- runtime writeback failures now preserve both HTTP `status_code` and a trimmed response-body excerpt in the runtime-side `WritebackError`, so callback `404`, `409`, or token/ownership drift can be diagnosed from one failure surface instead of requiring a second request replay

Expected behavior:
- request body must include a valid `release_id`
- request body should include `step_code`
- `step_name` may be accepted as legacy compatibility input during migration
- invalid payload or malformed `release_id` returns `400 invalid_argument`
- unknown release returns `404 not_found`
- step status strings are normalized case-insensitively to the repo-local step enum
- successful processing returns `204 no content`

Preferred step targeting rule:

- update release steps by stable `step_code`
- do not rely on human-facing display names as the long-term writeback key
- `step_name` remains migration-only compatibility input and should not be used for new callback integrations
- for rolling releases, callback senders should target `observe_rollout` and `finalize_release` only; `start_deployment` stays release-service-owned
- if callback payload omits `message`, release-service should synthesize a default operator-facing message from `step_code`, `status`, and `progress`
- when convergence stalls, inspect the callback layer in this order: missing `release_id` or token rejection at the writeback route, unexpected `step_code` normalization, then release-service step/status convergence state
- when runtime writeback returns a non-2xx response, inspect the preserved `status_code` plus response-body excerpt before retrying; treat `404 not_found` as ownership/stale-release first, and treat conflict-style `409` responses as release-state sequencing drift first

### `POST /api/v1/verify/release/artifact`

Purpose:

- allow release execution paths or callback workers to write back release-owned deployment bundle metadata
- update:
  - `artifact_repository`
  - `artifact_tag`
  - `artifact_digest`
  - `artifact_ref`
- optionally advance `publish_bundle` step state in the same request

Recommended payload shape:

```json
{
  "release_id": "11111111-1111-1111-1111-111111111111",
  "artifact_repository": "registry.example.com/devflow/releases/demo-api",
  "artifact_tag": "release-20260428",
  "artifact_digest": "sha256:abc",
  "artifact_ref": "oci://registry.example.com/devflow/releases/demo-api:release-20260428",
  "status": "Succeeded",
  "progress": 100,
  "message": "bundle published"
}
```

Notes:

- this is release-owned artifact writeback, not manifest artifact writeback
- `status` is interpreted as the state of the release-owned `publish_bundle` step
- artifact fields may be written before Argo application creation begins
- artifact callbacks may refresh the same `publish_bundle` step with normalized status and metadata, but they do not transfer ownership of that step away from release-service dispatch
- if callback payload omits `message`, release-service should synthesize a default artifact writeback message and prefer including `artifact_ref` when present

Normalized step statuses:
- `pending` -> `Pending`
- `running` -> `Running`
- `succeeded` -> `Succeeded`
- `failed` -> `Failed`

## Operational notes

- these routes are writeback surfaces, not public user-facing CRUD resources
- they mutate existing `Release` execution state only; they do not create releases
- the canonical resource-level shape for release data still lives in `docs/resources/release.md`
- the canonical step-by-step meaning of each release step now lives in `docs/system/release-steps.md`
- when writeback contracts change, update this file, `docs/resources/release.md`, and the handler tests in the same change
- if writeback returns `404 not_found`, inspect ownership routing before retry behavior

## Source pointers

- router: `internal/release/transport/http/router.go`
- auth middleware: `internal/release/transport/http/writeback_support.go`
- handlers: `internal/release/transport/http/release_writeback.go`
- tests: `internal/release/transport/http/release_writeback_test.go`
- config wiring: `internal/release/config/config.go`
