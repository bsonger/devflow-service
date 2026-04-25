# Meta-Service Root Restructure Design

## Reader and action

Reader:
- internal engineer or agent continuing the `devflow-service` migration

Post-read action:
- update repo docs and verification, then move `meta-service` from the old nested layout into the root `cmd/` and `internal/` structure without changing service behavior

## Scope

This design covers:
- the repository-level layout contract for the current `devflow-service` root
- the restructuring of the repo-local docs into layered directories instead of one flat `docs/` surface
- the restructuring of the current `meta-service` into the root `cmd/` and `internal/` layout
- the Go and Docker baseline update required to support that migration
- the verification and CI contract that must prove the new structure honestly

This design does not cover:
- migration of future services beyond `meta-service`
- business logic changes to the current service behavior
- a new multi-module or `go.work` contract
- reopening `devflow-control` future-state debates during this migration

## Authority model

Current implementation authority lives in this repository.
That includes:
- `AGENTS.md`
- repo-local docs
- repo-local verification and Docker rules

`../devflow-control` remains the future-state architecture authority for broader migration sequencing and long-term backend ownership boundaries.
It is an input, not the execution truth for this refactor.

If the local repo contract and `devflow-control` differ during this migration, the local repo contract wins for implementation and verification.

## Goals

1. Make the repository root the active `repo/` layout.
2. Keep the current service name as `meta-service`.
3. Move service code out of `modules/meta-service` and into root-level `cmd/` and `internal/`.
4. Replace catch-all shared structure with explicit domain and platform boundaries.
5. Upgrade the repository Go baseline to `1.26.2`.
6. Move install behavior into controlled base images so service Dockerfiles become packaging-only.
7. Keep service behavior stable while restructuring.
8. Make verification and CI prove the new contract directly.

## Non-goals

- designing final layouts for other services
- renaming `meta-service`
- introducing a generic `common`, `shared`, or `util` area
- using the migration to rework domain behavior or API semantics
- adding per-service `go.mod` files or `go.work`

## Target repository layout

```text
repo/
├── cmd/
│   └── meta-service/
│       └── main.go
├── internal/
│   ├── platform/
│   │   ├── config/
│   │   ├── db/
│   │   ├── logger/
│   │   ├── otel/
│   │   ├── httpx/
│   │   └── runtime/
│   ├── project/
│   │   ├── application/
│   │   ├── domain/
│   │   ├── repository/
│   │   ├── transport/
│   │   └── module.go
│   ├── environment/
│   │   ├── application/
│   │   ├── domain/
│   │   ├── repository/
│   │   ├── transport/
│   │   └── module.go
│   └── cluster/
│       ├── application/
│       ├── domain/
│       ├── repository/
│       ├── transport/
│       └── module.go
├── api/
├── deployments/
├── scripts/
├── docs/
│   ├── index/
│   ├── system/
│   ├── services/
│   ├── policies/
│   ├── generated/
│   ├── archive/
│   └── superpowers/
├── Dockerfile
├── Makefile
├── go.mod
└── go.sum
```

`meta-service` remains the process name, not a domain package.
Business logic is organized by explicit domains under `internal/`.

## Target docs layout

The repo-local docs should follow a layered structure instead of keeping all active docs flat at the root.

```text
docs/
├── README.md
├── index/
│   ├── README.md
│   ├── getting-started.md
│   └── agent-path.md
├── system/
│   ├── architecture.md
│   ├── constraints.md
│   ├── observability.md
│   └── recovery.md
├── services/
│   └── meta-service.md
├── policies/
│   ├── repo-layout.md
│   ├── docker-baseline.md
│   └── verification.md
├── generated/
│   └── README.md
├── archive/
│   └── README.md
└── superpowers/
    ├── specs/
    └── plans/
```

The purpose of this structure is:
- `docs/index/` for navigation only
- `docs/system/` for current repo-local execution truth
- `docs/services/` for service-specific current behavior and ownership
- `docs/policies/` for durable repo rules such as layout, Docker, and verification contracts
- `docs/generated/` for generated artifacts only
- `docs/archive/` for historical material only
- `docs/superpowers/` for process artifacts such as specs and plans

`AGENTS.md` should stay small.
It should be the startup contract and routing surface, not the place where all durable rules are copied.

## Structural rules

### `cmd/<service>/main.go`

The service entrypoint does only four things:
- load config
- initialize logger, db, tracer, and runtime dependencies
- assemble modules
- start the server

No domain logic should live here.

### `internal/<domain>/domain`

Holds domain objects, invariants, and domain-specific errors.

### `internal/<domain>/application`

Holds use-case orchestration.
This is where workflows such as create, query, update, or coordination across repositories belong.

### `internal/<domain>/repository`

Holds data-access interfaces and implementations.
Storage-specific implementations may live in subdirectories when needed.

### `internal/<domain>/transport`

Holds protocol adapters.
For the current migration, the first-class target is HTTP transport.
Request and response DTOs live here instead of a generic `model` package.

### `internal/<domain>/module.go`

Owns domain assembly.
It wires repository, application, and transport together for use by the service entrypoint or root app composition.

### `internal/platform`

Holds infrastructure-only capabilities:
- config loading
- database bootstrap and transaction helpers
- logger setup
- tracing and metrics bootstrap
- HTTP server and client utilities
- runtime lifecycle and graceful shutdown helpers

It must not become a hiding place for business semantics.

## Migration mapping from current code

The current service code maps as follows:

- old `modules/meta-service/cmd/main.go` -> `cmd/meta-service/main.go`
- old API handlers -> `internal/<domain>/transport/http`
- old app orchestration -> `internal/<domain>/application`
- old domain types and rules -> `internal/<domain>/domain`
- old store and infrastructure-backed persistence -> `internal/<domain>/repository`
- old config bootstrap -> `internal/platform/config`
- old router and service assembly -> domain modules plus root-level application assembly
- old shared HTTP, logging, telemetry, bootstrap, observability, and runtime helpers -> `internal/platform/*`

The migration intentionally removes these patterns:
- `modules/meta-service` as the long-term service home
- `shared/`
- generic `model` packages
- catch-all `common` or `util` naming

The current docs should map as follows:

- old `docs/recovery.md` -> `docs/system/recovery.md`
- old `docs/architecture.md` -> `docs/system/architecture.md`
- old `docs/constraints.md` -> `docs/system/constraints.md`
- old `docs/observability.md` -> `docs/system/observability.md`
- old `docs/docker.md` -> `docs/policies/docker-baseline.md`
- old service-local readme content for `meta-service` -> `docs/services/meta-service.md`
- old flat docs landing -> `docs/README.md` plus `docs/index/*`

## Docker and Go baseline

The repository Go baseline moves to `1.26.2`.

That upgrade applies to:
- `go.mod`
- local verification
- CI
- builder images
- service build and packaging scripts

The Docker contract changes with it:
- service Dockerfiles must not run install commands
- build-time tools must be present in controlled builder images
- runtime installation dependencies must be present in controlled runtime base images
- service Dockerfiles should package artifacts only

Acceptable service Dockerfile actions:
- choose a controlled base image
- copy built artifacts and tracked runtime files
- declare runtime metadata such as `EXPOSE`, `ENTRYPOINT`, or `CMD`

Unacceptable service Dockerfile actions:
- `apk add`
- `apt-get`
- `yum`
- `dnf`
- `go install`
- curl-pipe shell bootstraps
- any other install or bootstrap action that belongs in the base image contract

## Verification contract

The migrated repo must prove the new contract with these checks:

1. `gofmt ./...`
2. `go vet ./...`
3. `golangci-lint run`
4. `go test ./...`
5. `go build -o bin/meta-service ./cmd/meta-service`
6. `docker build`
7. CI that composes the same sequence

`scripts/verify.sh` should become the single repo-local entrypoint that orchestrates the same proof stack.

## Implementation order

The migration should run in this order:

1. update `AGENTS.md`, repo docs, Docker contract docs, and repo verification to the new layout
2. restructure docs into `index/`, `system/`, `services/`, and `policies/` with updated local authority rules
3. move `meta-service` entrypoint to `cmd/meta-service`
4. create `internal/platform` and absorb current infrastructure helpers there
5. split current service code into explicit domains under `internal/<domain>`
6. repair imports, build scripts, Dockerfiles, and CI to the new locations
7. rerun the full verification stack

This order is deliberate.
It prevents code movement from outrunning the repo contract and avoids leaving verification wired to stale paths.

## Risks and controls

### Risk: path churn breaks imports and tests

Control:
- move code by responsibility, not by mechanical filename mirroring
- keep behavior unchanged while relocating packages
- use the verification stack after each logical migration step

### Risk: `internal/platform` becomes a new dumping ground

Control:
- reject any package with domain terms in `internal/platform`
- keep repository and application logic inside explicit domains

### Risk: Docker drift persists after the refactor

Control:
- update Docker docs, base image contract, and verification together
- fail verification when service Dockerfiles contain install commands

### Risk: docs describe a different repo than the code

Control:
- update `AGENTS.md`, `README.md`, `docs/README.md`, `docs/index/*`, `docs/system/*`, `docs/services/*`, `docs/policies/*`, and `scripts/README.md` as one set
- do not claim the migration is done until those docs and the verification script align

## Success criteria

The migration is successful when all of the following are true:
- the repository root is the active service layout
- `meta-service` builds from `cmd/meta-service`
- business code lives under explicit domains in `internal/`
- infrastructure code lives under `internal/platform`
- `shared/`, `common/`, and `util` are not used as catch-all areas
- service Dockerfiles contain no install steps
- the repo verifies with formatting, vet, lint, tests, build, Docker build, and CI

## Deferred work

These items are intentionally deferred:
- applying the same layout to future services
- revisiting whether the long-term monorepo should become multi-module again
- deeper domain redesign inside `meta-service`
- release of any public API contract beyond the current local migration scope
