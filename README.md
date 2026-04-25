# DevFlow Service

`devflow-service` is the backend monorepo destination for the current DevFlow backend consolidation work.
The active local migration still focuses on `meta-service`, while the repo contract is aligned to a root-level Go monorepo layout and now also carries runnable `config-service`, `network-service`, `release-service`, and `runtime-service` entrypoints for extracted config/network/runtime boundaries.

## Purpose

This repo gives a fresh engineer or agent one place to answer:
- what the current repo-local layout contract is
- how docs are layered and where current truth lives
- which service is actively being migrated
- which verification and packaging rules must hold during the migration
- which command to rerun first after interruption

## Current state

Today this repo is in a transition state:
- the current active service name remains `meta-service`
- `config-service` now also boots from the root layout at `cmd/config-service` and owns the extracted config API surface for `AppConfig` and `WorkloadConfig`
- `network-service` now also boots from the root layout at `cmd/network-service` and owns the extracted network API surface for `Service` and `Route`
- `release-service` now also boots from the root layout at `cmd/release-service` with verify ingress folded into its release-owned HTTP surface
- `runtime-service` now also boots from the root layout at `cmd/runtime-service` and now owns extracted runtime spec, runtime revision, and runtime observed-pod APIs
- release-owned resource domains are split into `internal/image`, `internal/manifest`, `internal/intent`, with release-specific orchestration remaining in `internal/release`
- the target code layout is root `cmd/` plus root `internal/`
- business code follows `internal/<domain>/{service,domain,repository,transport}`
- the docs have moved to a layered structure under `docs/index/`, `docs/system/`, `docs/services/`, `docs/resources/`, and `docs/policies/`
- the canonical repo-local verification entrypoint remains `bash scripts/verify.sh`

This repo does **not** treat `modules/` as a valid end-state structure.
`internal/shared/` is allowed only as a small, controlled area for stable cross-domain helpers such as errors, response helpers, middleware, or id generation.
It is not a place for business logic, private models, or generic `common`/`util` dumping grounds.

## Kubernetes Database Baseline

Pre-production service manifests in this repo currently share one PostgreSQL 18 cluster in the Kubernetes `database` namespace:

- cluster: `pg18-next`
- writer endpoint: `pg18-next-rw.database:5432`
- database: `app`
- owner: `app`

The repo-managed install and bootstrap artifacts for that database now live under:

```text
deployments/pre-production/database/
```

The repo-local operational reference for this contract is:

```text
docs/system/postgresql.md
```

## Build baseline

The target repository baseline is:
- module path: `github.com/bsonger/devflow-service`
- target Go version: `1.26.2`
- target builder/runtime contract: controlled base images with all installation behavior moved out of service Dockerfiles

Service Dockerfiles should use thin multi-stage builds and keep installation behavior inside controlled base images only.
The root `Dockerfile` defaults to building `meta-service`.
Non-default service image selection for `config-service`, `network-service`, `release-service`, and `runtime-service` is a committed cluster-build concern and must be expressed through checked-in Tekton manifests under `deployments/tekton/` rather than ad-hoc local Docker commands.

## Repo shape

The active target top-level layout is:
- `cmd/` — runnable process entrypoints only
- `internal/` — repo-private implementation
- `internal/platform/` — infrastructure-only capabilities
- `internal/shared/` — optional, tightly-scoped shared helpers only
- `api/` — stable contracts such as OpenAPI or protobuf
- `deployments/` — deployment artifacts that belong in-repo
- `test/` — integration and e2e verification surfaces
- `docs/` — layered repo-local docs
- `scripts/` — repo-level verification and support scripts

For directory, layering, naming, and dependency decisions, the primary policy is:

```text
docs/policies/go-monorepo-layout.md
```

## Read this first

If you are landing here cold, read in this order:
1. `AGENTS.md`
2. `docs/system/recovery.md`
3. `docs/system/architecture.md`
4. `docs/policies/go-monorepo-layout.md`
5. `docs/services/meta-service.md`
6. `docs/resources/` only if the task needs current resource contracts
7. `docs/system/postgresql.md` only if the task touches PostgreSQL, Kubernetes database bootstrap, or service DSNs
8. `docs/policies/docker-baseline.md` only if the task touches packaging, Docker, or CI
9. `docs/policies/verification.md` and `scripts/README.md` only if the task touches verification
10. `../devflow-control/docs/target-architecture/devflow-service.md` only if local docs are not enough for a migration-boundary question

## Docs layout

Use the docs tree by purpose:
- `docs/index/` — navigation only
- `docs/system/` — current repo-local truth
- `docs/services/` — current service-specific behavior and diagnostics
- `docs/resources/` — current resource contracts and API behavior
- `docs/policies/` — durable repo rules, including Go monorepo layout policy
- `docs/generated/` — generated artifacts only
- `docs/archive/` — historical material only

## Verification and recovery

Use `docs/system/recovery.md` as the single repository-local recovery authority.
Use `docs/policies/verification.md` for the target verification stack.
Use `docs/policies/docker-baseline.md` for the base-image and packaging rules.

The first command to rerun after interruption is:

```sh
bash scripts/verify.sh
```

The target verification stack from the repo root is:

```sh
gofmt ./...
go vet ./...
golangci-lint run
go test ./...
go build -o bin/meta-service ./cmd/meta-service
go build -o bin/config-service ./cmd/config-service
go build -o bin/network-service ./cmd/network-service
go build -o bin/release-service ./cmd/release-service
go build -o bin/runtime-service ./cmd/runtime-service
bash scripts/verify.sh
```

Local ad-hoc Docker image builds are intentionally **not** part of that proof stack.
Service image selection belongs in committed Tekton manifests such as:

```text
deployments/tekton/config-service-preproduction-build-pipelinerun.yaml
deployments/tekton/network-service-preproduction-build-pipelinerun.yaml
deployments/tekton/release-service-preproduction-build-pipelinerun.yaml
deployments/tekton/runtime-service-preproduction-build-pipelinerun.yaml
```

The repo-level automation entrypoint for the same stack is:

```sh
make ci
```

The matching CI workflow lives at:

```text
.github/workflows/ci.yml
```

For the detailed Go monorepo layout contract, read:

```text
docs/policies/go-monorepo-layout.md
```
