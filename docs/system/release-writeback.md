# Release Writeback

## Purpose

This document defines the current repo-local operational contract for `release-service` writeback and observer callbacks.
It is the owning doc for token-gated writeback routes such as Argo event status updates and release step progress updates.

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
- these callbacks are expected to come from `runtime-service` or another rollout observer, not from normal end-user traffic
- these callbacks should also advance `observe_rollout` step state with normalized messages such as:
  - `rollout is running in argocd`
  - `rollout observed as succeeded by argocd`

### `POST /api/v1/verify/release/steps`

Purpose:
- update release step status and progress from external execution callbacks

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
- if callback payload omits `message`, release-service should synthesize a default operator-facing message from `step_code`, `status`, and `progress`

### `POST /api/v1/verify/release/artifact`

Purpose:

- allow async executor or callback workers to write back release-owned deployment bundle metadata
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
- `status` is interpreted as the state of the `publish_bundle` step
- artifact fields may be written before Argo application creation begins
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

## Source pointers

- router: `internal/release/transport/http/router.go`
- auth middleware: `internal/release/transport/http/writeback_support.go`
- handlers: `internal/release/transport/http/release_writeback.go`
- tests: `internal/release/transport/http/release_writeback_test.go`
- config wiring: `internal/release/config/config.go`
