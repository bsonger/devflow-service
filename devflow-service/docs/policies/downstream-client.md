# Downstream Client Policy

## Reader and outcome

This policy is for engineers and agents changing downstream runtime-boundary clients in `devflow-service`.

After reading it, a fresh reader should be able to:
- decide what belongs in a downstream client versus `service`
- reuse the shared downstream HTTP helper instead of hand-rolling transport code
- propagate request and trace context across service boundaries
- return typed downstream errors without leaking secrets or low-level noise

## Scope

This policy governs code under:

- `internal/*/transport/downstream`
- `internal/shared/downstreamhttp`
- HTTP-based runtime lookup clients that play the same role even if they are not yet migrated to the shared helper

It complements:

- `docs/policies/service-layer.md`
- `docs/policies/error-handling.md`
- `docs/policies/observability-logging.md`
- `docs/policies/go-monorepo-layout.md`

## Core rules

1. A downstream client is a transport adapter for another runtime boundary.
2. A downstream client must not own business orchestration.
3. A downstream client must propagate caller context, including trace context and `request_id` when present.
4. A downstream client must use explicit timeouts.
5. A downstream client should return typed transport errors that callers can classify without string matching.
6. A downstream client must not log secrets, auth material, or raw sensitive payloads.

## What belongs in a downstream client

Downstream client code is responsible for:

- building outbound HTTP requests
- applying shared headers such as `Accept: application/json`
- propagating trace context and request correlation headers
- decoding the downstream response envelope
- mapping transport failures into typed client-facing errors
- keeping request construction and response parsing small and testable

## What does not belong in a downstream client

Downstream client code must not:

- decide release, deploy, or workflow business outcomes
- own retries with business semantics unless the caller contract explicitly requires it
- parse Gin requests or write HTTP responses
- import `transport/http`
- duplicate shared helper logic already owned by `internal/shared/downstreamhttp`
- classify downstream errors by brittle string matching when a typed helper can be used

## Shared helper rules

When the downstream contract is HTTP + JSON, prefer the shared helper in `internal/shared/downstreamhttp`.

Shared helper responsibilities include:

- base URL normalization
- timeout ownership
- trace propagation
- request-id propagation
- standard success-envelope decoding
- typed non-2xx status errors

Feature-specific clients under `internal/*/transport/downstream` should stay thin wrappers around the shared helper.

## Error rules

Downstream clients should expose transport failures in a way callers can classify safely.

Prefer:

- a typed status error such as `downstreamhttp.StatusError`
- helpers such as `downstreamhttp.IsStatus(err, 404)`
- domain-level wrapping in the caller when the use case needs a more specific error name

Avoid:

- checking `err.Error()` for `404`
- leaking raw response bodies into returned errors by default
- inventing one-off status parsing helpers in each domain

### Recommended mapping pattern

- downstream adapter returns typed transport error
- service or support layer wraps it into domain meaning
- HTTP handler maps the domain meaning into API-safe error code and status

## Observability rules

Downstream clients must support correlation across metrics, logs, and traces.

Required expectations:

- outbound requests carry trace context
- outbound requests carry `X-Request-Id` when the caller context has one
- downstream helper logs, if any, use `snake_case`
- client code must not create high-cardinality metric labels from `trace_id`, `request_id`, `user_id`, `release_id`, or similar identifiers

When adding extra logs around downstream failures, prefer low-cardinality fields such as:

- `dependency`
- `operation`
- `method`
- `path`
- `status_code`
- `result`

## Security rules

Do not log or return these values from downstream adapters:

- `authorization` headers
- cookies
- tokens
- secrets
- kubeconfigs
- private keys
- raw request or response payloads that may contain sensitive material

If a downstream API needs credentials, keep credential handling in configuration and headers, not in error strings.

## Testing rules

Downstream clients should have small focused tests that cover:

- expected path and query construction
- envelope decoding
- bare JSON fallback when the downstream contract allows it
- typed non-2xx status classification
- trace and request-id propagation for shared helpers

## Recommended checklist

When adding or changing a downstream client, check:

1. is this really a runtime-boundary adapter and not business orchestration
2. did I reuse the shared HTTP helper where possible
3. does the client propagate trace context and `request_id`
4. does it return typed status errors instead of string-parsed failures
5. did I avoid logging or returning secrets
6. are tests covering request shape and status classification

## Verification expectations

When changing downstream adapters:

- keep `internal/*/transport/downstream` thin and transport-only
- keep shared HTTP behavior centralized in `internal/shared/downstreamhttp`
- keep docs, verification, and client behavior aligned in the same change cycle
