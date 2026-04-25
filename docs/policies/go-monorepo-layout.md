# Go Monorepo Layout Policy

This document captures the detailed Go monorepo layout rule set for `devflow-service`.
This is the primary directory, layering, naming, and dependency policy for the repo.

## Scope

- Keep the current repo root as the active monorepo root.
- Keep the current active runnable service name as `meta-service`.
- Use this policy to guide new work, doc updates, and refactors.
- When layout guidance conflicts with older repo-local wording, prefer this document.

## Current top-level layout

The repo root should use:

- `go.mod`, `go.sum`
- `Makefile`
- `cmd/`
- `internal/`
- `api/`
- `deployments/`
- `scripts/`
- `test/`
- `docs/`

The current repo also keeps a root `Dockerfile`.
That is the active packaging contract today.

## Entrypoint rules

Each service entrypoint should live at `cmd/<service>/main.go`.

`main.go` should do only four things:

1. load configuration
2. initialize logger, PostgreSQL, tracing, and other platform dependencies
3. assemble domain modules
4. start the server or runtime loop

`cmd/` must not accumulate business logic, SQL, or long orchestration flows.

## Domain layout rules

Business code should be grouped by explicit domain under `internal/`.

Each domain should follow this shape:

- `internal/<domain>/domain`
- `internal/<domain>/service`
- `internal/<domain>/repository`
- `internal/<domain>/transport`
- `internal/<domain>/module.go`

Current repo-local examples include:

- `internal/application/`
- `internal/project/`
- `internal/cluster/`
- `internal/environment/`

## Layer responsibilities

### `domain`

- domain entities
- business rules
- domain validation
- domain enums and errors

`domain` must not depend on HTTP frameworks, repository implementations, or transport packages.

### `service`

- use-case orchestration
- business workflow sequencing
- repository interface consumption
- domain coordination

`service` should not write SQL directly or depend on HTTP status codes or framework request types.

### `repository`

- persistence interfaces and implementations
- PostgreSQL access
- storage-specific translation

`repository` may depend on `domain` and `internal/platform`, but it should not take on business orchestration.

### `transport`

- HTTP, gRPC, queue, or other protocol adapters
- request parsing and response shaping
- mapping between wire DTOs and service calls

`transport` should not own business rules or persistence logic.

### `module.go`

- initialize repositories
- initialize services
- initialize transports
- wire one domain together

## Platform rules

`internal/platform/` is the infrastructure layer.

Allowed areas include:

- `config/`
- `db/`
- `logger/`
- `otel/`
- `httpx/`
- `runtime/`

Platform code must stay domain-agnostic.
It must not import business-domain packages as a shortcut.

## Shared code rules

`internal/shared/` is optional, not mandatory.
If introduced, it must stay small and stable.

Allowed examples:

- `internal/shared/errs`
- `internal/shared/response`
- `internal/shared/middleware`
- `internal/shared/idgen`

Disallowed examples:

- `internal/shared/common`
- `internal/shared/utils`
- `internal/shared/base`
- `internal/shared/model`

Rules:

- prefer duplication once over premature abstraction
- extract only after repetition is stable
- do not move business-domain semantics into `internal/shared`

## Refactor and extraction rules

When reducing duplication under `internal/`, use this order of preference:

1. keep a small amount of local duplication if the repeated code is still evolving
2. extract infra-only helpers into `internal/platform/` when the behavior is storage-, runtime-, or transport-agnostic
3. extract tiny stable helpers into `internal/shared/` only when they are truly cross-domain and have no business meaning

Good extraction candidates:

- SQL helper functions such as placeholder builders, null handling, row-count checks, and JSON encode/decode helpers
- observability helpers that are domain-agnostic
- narrow response or middleware helpers

Poor extraction candidates:

- `BaseModel` or other domain entity bases shared across unrelated business domains
- catch-all `model`, `common`, `base`, or `util` packages
- helpers that smuggle one domain's language into another domain

If the repeated logic mentions business resources, business statuses, or domain-specific validation rules, it usually should stay inside that domain.

### Base model rule

Do not introduce a repo-global shared `BaseModel` under `internal/shared/` or `internal/platform/`.

Even if multiple domains currently use the same `id / created_at / updated_at / deleted_at` shape, keep the base entity definition inside each domain unless there is an explicit repo-wide decision to couple those domains to one lifecycle model.

Reason:

- domain entities evolve at different speeds
- a shared base model creates hidden cross-domain coupling
- the diff saved by deduplication is usually smaller than the long-term architectural cost

## Directory merge rules

Do not flatten or merge directories just to reduce file count.

Keep these boundaries explicit:

- `internal/<domain>/domain`
- `internal/<domain>/service`
- `internal/<domain>/repository`
- `internal/<domain>/transport`

Do not merge multiple business domains into one generic resource package.

Examples of merges to avoid:

- combining `application`, `project`, `cluster`, and `environment` into one broad CRUD package
- moving release-owned support logic into repo-global shared code
- collapsing `transport/http` and `transport/downstream` into one mixed adapter layer

Examples of file-level consolidation that are acceptable:

- deleting alias-only forwarding files after direct imports are practical
- consolidating repeated infra helpers into `internal/platform/`
- inlining tiny one-use helper files back into their owning package when the file boundary no longer adds clarity

The goal is explicit ownership boundaries first, smaller file counts second.

## Repository implementation consistency rules

Repository packages should prefer a consistent construction shape:

- expose a narrow `Store` interface
- keep the concrete PostgreSQL implementation private when practical
- prefer constructors that return the interface type, such as `func NewPostgresStore() Store`

This keeps service wiring stable and prevents callers from depending on storage-specific concrete types by accident.

Minor naming differences in existing files are acceptable during migration, but new work should move toward the consistent interface-returning pattern instead of adding more variants.

## API and deployments rules

- `api/` holds contracts such as OpenAPI, protobuf, JSON Schema, and examples
- `deployments/` holds deployment artifacts such as manifests, Helm assets, or overlays

`api/` is contract surface.
`internal/<domain>/transport` is implementation surface.

## Dependency direction

Preferred dependency direction:

- `cmd -> module + platform`
- `transport -> service`
- `service -> domain + repository interface`
- `repository -> domain + platform`
- `platform -> no business-domain dependency`
- `shared -> no business-domain dependency`

Avoid direct imports of another domain's internal implementation as a substitute for an API boundary.

## Naming rules

- service directories use kebab-case such as `meta-service`
- Go package names stay short, lowercase, and simple
- prefer names like `config`, `db`, `otel`, `logger`, `service`, `domain`
- avoid names like `commonutil`, `basepkg`, `release_service`

Interfaces should be named by role or behavior, not with `I` prefixes.

## Build and module rules

- keep one root `go.mod`
- do not introduce `go.work` during the current repo stage
- do not split into per-service modules unless the repo later has a real release-management reason to do so

## Current repo-specific notes

- `meta-service` remains the active migration focus in this repo
- current runnable service entrypoints in this repo are `meta-service`, `config-service`, `network-service`, `release-service`, and `runtime-service`
- the layout and naming rules above still apply so future domains do not drift
