# Architecture

## Role of this repository

`devflow-service` is the backend monorepo destination for the current DevFlow backend consolidation work.
The active local migration focuses on `meta-service`, while `config-service`, `network-service`, `release-service`, and `runtime-service` have also been brought into the same root-level `cmd` and `internal` layout.
Inside `internal/`, release-owned business resources now follow the same top-level split pattern as the rest of the repo: `internal/manifest`, `internal/intent`, plus release-specific assembly and adapters in `internal/release`.
At the service-dependency layer, `release-service` is the main cross-service composer: it reads upstream truth from `meta-service`, `config-service`, and `network-service`, then freezes that data into release-owned `Manifest` and `Release` records. `runtime-service` is a Kubernetes-facing runtime operations and observation service.

## Root structure

The target top-level structure for current local work is:
- `cmd/` for runnable process entrypoints only
- `internal/` for repo-private implementation
- `internal/platform/` for infrastructure-only capabilities
- `internal/shared/` for optional, stable, cross-domain helpers only
- `api/` for stable contracts such as OpenAPI or protobuf
- `deployments/` for deployment artifacts that belong in-repo
- `test/` for integration and end-to-end verification surfaces
- `docs/` for repo-local documentation, layered by purpose
- `scripts/` for verification and support scripts

This replaces the older local staging shape built around `modules/`.
It does not reintroduce a broad shared-code layer; any future `internal/shared/` use must stay narrow and infrastructure-like.

## Build model

The active local execution contract is:
- one root Go module at the repo root
- target Go baseline `1.26.2`
- current runnable process entries include `meta-service`, `config-service`, `network-service`, `release-service`, and `runtime-service`
- pre-production data persistence is backed by the Kubernetes `database/pg18-next` CloudNativePG cluster exposed through `pg18-next-rw.database:5432`

This repository should not introduce `go.work` or per-service `go.mod` files during the current migration.

## Code organization model

The current target code layout is:

- `cmd/meta-service/main.go` for entrypoint-only startup logic
- `cmd/config-service/main.go` for the extracted config-service entrypoint
- `cmd/network-service/main.go` for the extracted network-service entrypoint
- `cmd/release-service/main.go` for the migrated release-service entrypoint
- `cmd/runtime-service/main.go` for the extracted runtime-service entrypoint
- `internal/platform/` for infrastructure-only capabilities such as config, db, logger, otel, httpx, and runtime lifecycle
- `internal/shared/` for a small number of stable cross-domain helpers when duplication is no longer justified
- `internal/<domain>/domain` for domain objects and rules
- `internal/<domain>/service` for use-case orchestration
- `internal/<domain>/repository` for data-access interfaces and implementations
- `internal/<domain>/transport` for external protocol adapters
- `internal/<domain>/module.go` for domain assembly

The point of this shape is to keep ownership explicit and avoid hidden dumping grounds.

## Documentation model

Repo-local docs are layered by purpose:
- `docs/index/` for navigation only
- `docs/system/` for current repo-local execution truth
- `docs/services/` for current service-specific behavior and diagnostics
- `docs/resources/` for current resource contracts, API behavior, and validation rules
- `docs/policies/` for durable repo rules
- `docs/generated/` for generated artifacts only
- `docs/archive/` for historical material only

`AGENTS.md` remains the startup contract, not the place where all durable repo rules are copied.

For a visual companion to this document, use:

- `docs/system/diagrams.md`
- `docs/system/diagrams/service-dependencies.md`
- `docs/system/diagrams/release-flow.md`
- `docs/system/diagrams/runtime-flow.md`
- `docs/system/diagrams/resource-ownership.md`
- `docs/system/flow-overview.md`

For shared ingress routing and backend-local path rewriting, use:

- `docs/system/ingress-routing.md`

For the current extraction state and remaining same-repo implementation realities, use:

- `docs/system/current-service-extraction-reality.md`

For runtime-service memory, observer, and remaining PostgreSQL-backed support boundaries, use:

- `docs/system/runtime-storage-model.md`

## Non-goals

This migration does not currently aim to:
- design final layouts for future services
- preserve `modules/meta-service` as the long-term service home
- create a broad `shared/`, `common/`, or `util/` dumping ground
- rewrite business behavior while restructuring packages
