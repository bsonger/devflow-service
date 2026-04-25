# Error Handling Policy

## Reader and outcome

This policy is for engineers and agents changing HTTP handlers, service flows, or error mapping in `devflow-service`.

After reading it, a fresh reader should be able to:
- return API errors with one stable envelope shape
- choose a small consistent error-code vocabulary
- map domain failures to HTTP status codes without inventing one-off patterns
- avoid leaking low-level internal details to API consumers

## Scope

This policy governs:
- HTTP error response shape
- application error code naming
- handler-level HTTP status mapping
- service and repository error propagation expectations

## Core rules

1. Public HTTP APIs must use one consistent error envelope.
2. Error codes must stay small, explicit, and reusable.
3. Handlers should translate domain and storage errors into API-safe responses.
4. Services should return explicit errors instead of writing HTTP responses directly.
5. Internal implementation details such as stack traces, SQL text, raw tokens, or secrets must not be exposed in API responses.

## HTTP error envelope

Use the shared envelope owned by `internal/platform/httpx`.

Expected response shape:

```json
{
  "error": {
    "code": "invalid_argument",
    "message": "invalid page",
    "details": {
      "field": "page"
    }
  }
}
```

Handlers should prefer `httpx.WriteError(...)` instead of hand-rolling response bodies.

Prefer the higher-level `httpx` helpers when they cover the case, such as:

- `WriteInvalidArgument`
- `WriteNotFound`
- `WriteConflict`
- `WriteFailedPrecondition`
- `WriteInternalError`
- `BindJSON`

For service- and repository-layer generic validation or state-classification errors, prefer the shared helpers in `internal/shared/errs`, such as:

- `errs.Required`
- `errs.JoinInvalid`
- `errs.InvalidArgument`
- `errs.Conflict`
- `errs.FailedPrecondition`

## Standard error codes

Prefer this small stable set:

- `invalid_argument`
- `not_found`
- `conflict`
- `failed_precondition`
- `internal`

Reserve adding new top-level codes for cases where the existing set cannot describe the failure clearly.

### Code meanings

#### `invalid_argument`

Use when the client supplied malformed, missing, or semantically invalid input.

Examples:
- invalid UUID in a path or query field
- missing required request field
- invalid pagination value
- invalid user-supplied enum or filter

#### `not_found`

Use when the addressed resource does not exist or is not visible through the current API contract.

Examples:
- missing project
- missing application
- soft-deleted resource queried by id
- missing release

#### `conflict`

Use when the request is valid in shape but conflicts with current state.

Examples:
- duplicate unique name
- uniqueness violation on create or update
- state conflict where the resource already exists in a mutually-exclusive way

#### `failed_precondition`

Use when the request is valid, but a required dependency or upstream state is not ready.

Examples:
- release manifest is not ready
- cluster onboarding is not ready
- runtime spec binding does not match the requested release
- sync cannot continue because a required upstream asset has not been prepared

#### `internal`

Use for unexpected server-side failures that the client cannot correct directly.

Examples:
- unexpected database failure
- unexpected serialization failure
- unexpected downstream transport failure without a more specific contract

## Recommended HTTP status mapping

Preferred mapping:

- `invalid_argument` -> `400 Bad Request`
- `not_found` -> `404 Not Found`
- `conflict` -> `409 Conflict`
- `failed_precondition` -> usually `409 Conflict`
- `internal` -> `500 Internal Server Error`

### When `424 Failed Dependency` is acceptable

`424 Failed Dependency` is allowed when the API is explicitly surfacing a dependency-chain failure rather than a generic resource-state conflict.

Use it sparingly and only when the handler is intentionally describing an unmet upstream dependency.

If there is no strong reason to distinguish it, prefer `409` with `failed_precondition`.

## Layer responsibilities

### `repository`

Repositories should return storage and persistence errors upward.

Repositories must not decide HTTP status codes.

They may expose helper classification such as:
- not found
- unique violation

but that classification should stay storage-oriented.

### `service`

Services should return explicit domain or workflow errors.

Services must not write HTTP responses.

Services should prefer:
- explicit sentinel errors
- wrapped errors with stable meaning
- domain-oriented failure names

### `transport/http`

HTTP handlers own final mapping from:
- parse/bind failure
- repository/service error
- storage classification

to:
- HTTP status code
- API error code
- API-safe message

## Response message rules

Error messages returned to API clients should be:
- short
- actionable when possible
- free of secrets
- free of stack traces
- free of raw SQL or driver internals

Allowed:
- `invalid id`
- `not found`
- `image runtime_spec_revision_id is required`
- `invalid request body`
- `internal error`

Avoid exposing:
- SQL queries
- low-level database driver dumps
- raw downstream credentials
- full internal stack traces

### Internal error rule

Unexpected server-side failures should not surface `err.Error()` directly to API clients.

Prefer:

- log or attach the original error to Gin context
- return a stable external message such as `internal error`

This keeps 5xx responses safe and predictable while preserving the real failure in logs and traces.

### Generic validation rule

When code under `internal/*/service` or `internal/*/repository` needs a generic validation or state-classification error, prefer `internal/shared/errs` over ad-hoc `errors.New(...)` strings.

Examples:

- required-field validation
- joined invalid-field validation
- generic conflict classification
- generic failed-precondition classification

Keep business-specific sentinel errors when they carry domain meaning, but avoid scattering many one-off string-only validation errors across packages.

Context-preserving wraps are allowed when attaching detail to an existing domain error classification, for example:

- `fmt.Errorf("%w: cluster name is required", ErrClusterOnboardingMalformed)`
- `fmt.Errorf("%w: kubeconfig payload invalid", ErrClusterOnboardingMalformed)`

In that pattern, the stable meaning comes from the wrapped sentinel, not from inventing a new ad-hoc top-level error string.

## Error details rules

Use `details` only for small safe structured context.

Good examples:
- invalid field name
- validation field list
- pagination boundary

Do not place sensitive values, stack traces, tokens, kubeconfigs, or large payloads in `details`.

## Handler guidance

When adding or changing a handler, check:

1. did I use the shared error envelope
2. is the chosen error code from the standard set
3. is the HTTP status code aligned with the code meaning
4. is the message safe for external consumers
5. did I avoid leaking internal implementation details

## Verification expectations

When changing API error behavior:
- keep the envelope shape consistent with `internal/platform/httpx`
- reuse the standard error code vocabulary where possible
- update affected `docs/resources/*.md` files when public behavior changes
- keep verification, docs, and handlers aligned in the same change cycle
