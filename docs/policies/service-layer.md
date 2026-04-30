# Service Layer Policy

## Reader and outcome

This policy is for engineers and agents changing code under `internal/*/service`.

After reading it, a fresh reader should be able to:
- implement service-layer code that matches repo boundaries
- decide what belongs in `service` versus `transport` or `repository`
- handle cross-domain coordination without smuggling HTTP or storage concerns inward
- avoid turning the service layer into either a thin pass-through or a transport-aware god layer

## Scope

This policy governs code under:

- `internal/<domain>/service`

It complements:

- `docs/policies/go-monorepo-layout.md`
- `docs/policies/error-handling.md`
- `docs/policies/http-handler.md`

## Core rules

1. The service layer owns business orchestration.
2. The service layer must not write HTTP responses or depend on Gin request types.
3. The service layer must not access the database directly.
4. The service layer may coordinate multiple repositories or domains when the use case requires it.
5. The service layer should return explicit domain or workflow errors upward.

## What belongs in service

Service code is the place for:

- use-case orchestration
- state-transition sequencing
- cross-repository coordination
- domain validation that depends on current state
- workflow dispatch and retry entrypoints
- cross-domain coordination where a use case truly spans boundaries

Examples from this repo include:

- validating a referenced project before creating an application
- resolving release environment and dispatching release orchestration
- syncing configuration revisions from the config repository into application-owned state

## What does not belong in service

Service code must not:

- call `db.Postgres()` or `store.DB()` directly
- depend on `github.com/gin-gonic/gin`
- depend on `internal/platform/httpx`
- import `internal/*/transport/http`
- write HTTP status codes or response bodies
- own storage-specific SQL translation

These concerns belong in:

- `transport/http` for HTTP parsing and response shaping
- `repository` for persistence details

## Cross-domain coordination rules

Cross-domain coordination is allowed in `service`, but it should be explicit and narrow.

Prefer one of these patterns:

- depend on another domain's service interface when the use case needs business behavior
- depend on another domain's repository interface when only existence or lookup checks are needed
- depend on `transport/downstream` or support packages when crossing a true runtime boundary

Avoid:

- importing another domain's `transport/http`
- reaching into another domain's internal concrete implementation as a shortcut
- chaining many global default services together without a clear orchestration need

## Dependency guidance

Good service dependencies:

- owning domain package
- owning repository package
- selected cross-domain service or repository interfaces
- platform logging, tracing, or runtime helpers
- downstream clients when the use case must call another runtime boundary

Poor service dependencies:

- Gin handler packages
- HTTP response helpers
- raw SQL helpers used as a substitute for repository ownership
- broad shared business packages

## Error rules

Services should return explicit errors with stable meaning.

Prefer:

- sentinel errors
- wrapped errors that preserve stable intent
- domain-specific names such as `ErrReleaseManifestNotAvailable`
- shared generic helpers from `internal/shared/errs` for required-field, invalid-input, conflict, and failed-precondition cases that are not domain-specific

Services should not decide final HTTP status codes.
That mapping belongs in `transport/http`.

Avoid ad-hoc `errors.New("field is required")` patterns repeated across services when `internal/shared/errs` already covers the case.

When a service already has a domain-specific sentinel such as a malformed or failed-precondition error, wrapping it with contextual detail is acceptable.

Example:

- `fmt.Errorf("%w: cluster name is required", ErrClusterOnboardingMalformed)`

## Logging rules

Service logs should focus on:

- workflow decisions
- state transitions
- dependency failures
- orchestration outcomes

Avoid logging pure control flow noise.

When a service operation emits logs, prefer the shared observability fields:

- `operation`
- `resource`
- `resource_id`
- `result`

## Construction and interfaces

Prefer:

- a small interface for the service boundary when the service is used externally
- constructor injection for dependencies
- stable defaults only when they reduce boilerplate and do not hide important boundaries

Global default service singletons are acceptable in existing migration-era code, but new work should prefer explicit construction when it improves boundary clarity or testability.

## Recommended checklist

When adding or changing service-layer code, check:

1. does this belong in service rather than handler or repository
2. did I avoid direct DB access
3. did I avoid Gin and HTTP response helpers
4. is cross-domain coordination explicit and justified
5. am I returning domain/workflow errors instead of transport responses
6. are logs focused on decisions and outcomes

## Verification expectations

When changing service-layer behavior:

- keep direct persistence inside repository packages
- keep HTTP response shaping inside transport packages
- keep docs and verification aligned with the current service-layer boundary rules
