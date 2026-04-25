# Repository Layer Policy

## Reader and outcome

This policy is for engineers and agents changing persistence code under `internal/*/repository`.

After reading it, a fresh reader should be able to:
- decide what belongs in a repository versus `service`
- keep storage details isolated from HTTP and workflow orchestration
- implement repository constructors and interfaces consistently
- handle scan, filter, and persistence translation without leaking storage concerns outward

## Scope

This policy governs code under:

- `internal/<domain>/repository`

It complements:

- `docs/policies/go-monorepo-layout.md`
- `docs/policies/service-layer.md`
- `docs/policies/error-handling.md`

## Core rules

1. The repository layer owns persistence details.
2. Repositories must not own business orchestration.
3. Repositories must not depend on Gin, HTTP response helpers, or `transport/http` packages.
4. Repositories should expose a narrow interface for the calling layer.
5. Storage-specific translation belongs inside the repository package, not in `service`.

## What belongs in repository

Repository code is responsible for:

- SQL statements and query construction
- row scanning and JSON column decoding
- storage-only filters such as `include_deleted`, `status`, or foreign-key lookups
- converting nullable DB values into domain-safe fields
- persistence helpers such as placeholder, rows-affected, and JSON marshal helpers
- returning storage errors upward without inventing transport responses

Examples include:

- `scan*` helpers that decode rows into domain models
- `ListFilter` structs that shape repository-owned queries
- `NewPostgresStore()` constructors that hide the concrete implementation behind an interface

## What does not belong in repository

Repository code must not:

- import `github.com/gin-gonic/gin`
- import `internal/platform/httpx`
- import `internal/*/transport/http`
- import `internal/*/service`
- decide HTTP status codes or API error codes
- coordinate multi-step workflows across domains
- call downstream runtime-boundary clients

If logic needs current business state, sequencing, or cross-domain coordination, it belongs in `service`.

## Interface and constructor rules

Preferred construction shape:

- define a narrow role-based interface such as `Store` or `AppConfigStore`
- keep the concrete PostgreSQL implementation private when practical
- prefer constructors that return the interface type

Preferred examples:

- `func NewPostgresStore() Store`
- `func NewAppConfigPostgresStore() AppConfigStore`

Avoid exposing storage-specific concrete types as the public dependency shape for callers.

## Query and scan rules

Repositories own:

- SQL text
- query argument ordering
- `sql.Null*` handling
- JSONB marshal and unmarshal
- `RowsAffected` checks
- storage-driven list ordering such as `order by created_at desc`

Services should not duplicate scan or SQL helper logic that already belongs here.

When repeated infra-only persistence helpers appear across domains, prefer extracting them into `internal/platform/dbsql` instead of inventing a broad shared business package.

## Error rules

Repositories return storage-facing errors upward.

Prefer:

- `sql.ErrNoRows` for missing persisted rows
- wrapped storage errors when extra repository context matters
- shared DB helpers for rows-affected checks
- `internal/shared/errs` for repository-owned generic validation such as required-field or joined invalid-input checks

Repositories must not:

- convert errors into HTTP status codes
- write API envelopes
- leak secrets or credentials in error strings

Avoid repeating ad-hoc `errors.New("field is required")` validation strings in repository packages when `internal/shared/errs` already covers the case.

Service or handler layers may wrap repository errors into domain or API meaning later.

## Logging rules

Repository logs are optional and should stay low-noise.

If a repository emits logs, prefer:

- operation-level failure logs
- stable low-cardinality fields such as `operation`, `resource`, `resource_id`, and `result`

Avoid noisy success logging for every query unless it provides durable operational value.

## Cross-domain rules

A repository may depend on:

- its own domain package
- `internal/platform/db`
- `internal/platform/dbsql`
- standard-library SQL and encoding helpers

A repository should not depend on another domain's repository or service as a shortcut.

Cross-domain coordination belongs in `service`.

## Recommended checklist

When adding or changing repository code, check:

1. is this persistence logic rather than business orchestration
2. did I keep HTTP, Gin, and handler helpers out of the repository package
3. does the constructor return a narrow interface type
4. are scan and nullable-value translations owned here
5. did I return storage errors upward without mapping HTTP semantics
6. if helper logic repeated, should it move to `internal/platform/dbsql`

## Verification expectations

When changing repository code:

- keep persistence ownership inside `internal/*/repository`
- keep HTTP and workflow concerns out of repository packages
- keep docs, verification, and constructors aligned with the repository boundary rules
