# HTTP Handler Policy

## Reader and outcome

This policy is for engineers and agents changing HTTP transport code in `devflow-service`.

After reading it, a fresh reader should be able to:
- implement a new Gin handler that matches repo conventions
- choose the right shared response helper
- parse ids, filters, and pagination consistently
- keep handler responsibilities separate from service and repository responsibilities

## Scope

This policy governs code under:

- `internal/*/transport/http`
- `internal/platform/httpx`

It complements:

- `docs/policies/error-handling.md`
- `docs/policies/api-compatibility.md`
- `docs/policies/go-monorepo-layout.md`

## Core rules

1. HTTP handlers own HTTP concerns only.
2. Handlers must use the shared response helpers in `internal/platform/httpx`.
3. Handlers must not perform business orchestration that belongs in `service`.
4. Handlers must not perform direct persistence.
5. Handler error mapping must stay consistent with the error-handling policy.

## Handler responsibilities

Handlers are responsible for:

- binding path, query, and JSON input
- validating transport-level parse failures such as invalid UUIDs or invalid pagination values
- constructing service input objects
- calling the service layer
- mapping returned errors into HTTP status codes and API error codes
- shaping response envelopes

Handlers should stay thin.

## Shared response helpers

Use the shared helpers from `internal/platform/httpx`:

- `WriteData`
- `WriteList`
- `WriteNoContent`
- `WriteError`
- `WriteInvalidArgument`
- `WriteNotFound`
- `WriteConflict`
- `WriteFailedPrecondition`
- `WriteUnauthorized`
- `WriteInternalError`
- `BindJSON`
- `ParseUUIDParam`
- `ParseUUIDQuery`
- `ParseUUIDString`
- `ParsePaginationOrWrite`
- `WritePaginatedList`

Do not hand-roll success or error envelopes in individual handlers when the shared helper already covers the case.

Prefer the specialized helper over `WriteError(...)` when the error code and status are already standardized.

Examples:

- use `WriteInvalidArgument` instead of `WriteError(..., "invalid_argument", ...)`
- use `WriteFailedPrecondition` instead of `WriteError(..., "failed_precondition", ...)`
- use `WriteUnauthorized` instead of repeating the same unauthorized envelope

### Preferred response patterns

#### Create

- on success: `201 Created` with `WriteData`

#### Get

- on success: `200 OK` with `WriteData`

#### Update or patch without a response body

- on success: `204 No Content` with `WriteNoContent`

#### Delete

- on success: `204 No Content` with `WriteNoContent`

#### List

- on success: `200 OK` with `WriteList`

## Path and query parsing rules

### UUID path parameters

Parse ids at the handler boundary.

Prefer the shared UUID helpers in `internal/platform/httpx` instead of hand-rolling `uuid.Parse(...)` plus repeated `WriteError(...)` blocks.

If parsing fails:

- return `400`
- use error code `invalid_argument`
- use a small explicit message such as `invalid id`, `invalid application_id`, or `invalid manifest_id`

Do not push malformed path ids into the service layer.

### Query filters

Handlers should parse query filters into an explicit service filter struct.

Prefer:

- explicit filter field names
- low-surprise defaults
- transport-safe parsing at the edge
- shared helpers such as `ParseUUIDQuery` for optional UUID filters

Examples:

- `application_id`
- `environment_id`
- `status`
- `type`
- `name`
- `include_deleted`

## Pagination rules

Use the shared pagination helpers in `internal/platform/httpx`:

- `ParsePagination`
- `PaginateSlice`
- `IncludeDeleted`

When pagination parsing fails:

- return `400`
- use error code `invalid_argument`

List handlers should:

1. parse filters
2. call the service
3. prefer `WritePaginatedList` for the shared pagination + response path

## Error handling rules

Follow `docs/policies/error-handling.md`.

At the handler layer:

- parse and bind failures map to `invalid_argument`
- `sql.ErrNoRows` usually maps to `not_found`
- precondition-style workflow blockers map to `failed_precondition`
- uniqueness or state collisions map to `conflict`
- unexpected failures map to `internal`

Handlers should not leak:

- stack traces
- SQL text
- driver internals
- tokens or secrets

Prefer `WriteInternalError` for unexpected 5xx paths so the client message stays stable while the original error remains visible in logs.

## Logging rules

Handlers may rely on shared middleware for request summary logging.

Add handler-specific logs only when they provide durable diagnostic value beyond middleware-level request logs.

Prefer:

- decision-point logs
- explicit outcome logs for exceptional branches

Avoid adding noisy per-handler logs that duplicate the existing request middleware summary.

## Layer boundary rules

### Handler must not do

- direct DB access
- direct repository construction as a shortcut around the service layer
- business workflow sequencing that belongs in `service`
- storage-specific translation logic

### Service must not do

- write HTTP responses
- depend on Gin request or response types
- choose HTTP status codes

## Recommended handler checklist

When adding or changing a handler, check:

1. did I parse ids and query values at the HTTP edge
2. did I use `httpx` response helpers
3. did I keep business logic in `service`
4. did I map errors with the standard API error vocabulary
5. did I keep messages short and safe
6. did I apply shared pagination behavior for list endpoints

## Verification expectations

When changing handler behavior:

- keep the response envelope aligned with `internal/platform/httpx`
- keep error mapping aligned with `docs/policies/error-handling.md`
- update affected `docs/resources/*.md` files when public behavior changes
- keep docs, handlers, and verification aligned in the same change cycle
