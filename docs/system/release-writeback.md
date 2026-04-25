# Release Writeback

## Purpose

This document defines the current repo-local operational contract for `release-service` writeback and observer callbacks.
It is the owning doc for token-gated writeback routes such as Argo event status updates and release step progress updates.

## Current owner

- owning service boundary: `docs/services/release-service.md`
- owning HTTP package: `internal/release/transport/http`
- owning config injection: `internal/release/config/config.go`

## Protected writeback routes

The current writeback routes live under the `release-service` API surface:

```text
POST /api/v1/verify/argo/events
POST /api/v1/verify/release/steps
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

### `POST /api/v1/verify/release/steps`

Purpose:
- update release step status and progress from external execution callbacks

Expected behavior:
- request body must include a valid `release_id`
- invalid payload or malformed `release_id` returns `400 invalid_argument`
- unknown release returns `404 not_found`
- step status strings are normalized case-insensitively to the repo-local step enum
- successful processing returns `204 no content`

Normalized step statuses:
- `pending` -> `Pending`
- `running` -> `Running`
- `succeeded` -> `Succeeded`
- `failed` -> `Failed`

## Operational notes

- these routes are writeback surfaces, not public user-facing CRUD resources
- they mutate existing `Release` execution state only; they do not create releases
- the canonical resource-level shape for release data still lives in `docs/resources/release.md`
- when writeback contracts change, update this file, `docs/resources/release.md`, and the handler tests in the same change

## Source pointers

- router: `internal/release/transport/http/router.go`
- auth middleware: `internal/release/transport/http/writeback_support.go`
- handlers: `internal/release/transport/http/release_writeback.go`
- tests: `internal/release/transport/http/release_writeback_test.go`
- config wiring: `internal/release/config/config.go`

