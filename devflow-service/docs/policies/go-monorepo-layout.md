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
2. initialize logger, database, tracing, and other platform dependencies
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
- database access
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

- today only `meta-service` is the active runnable service in this repo
- the layout and naming rules above still apply so future domains do not drift
- if existing files lag behind this policy, update them toward this layout instead of preserving the older wording
