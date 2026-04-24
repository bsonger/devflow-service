# DevFlow Service

`devflow-service` is the backend monorepo destination for the current DevFlow backend consolidation work.
The active local migration focuses on one service, `meta-service`, and on moving the repository to a root-level `cmd` and `internal` layout.

## Purpose

This repo gives a fresh engineer or agent one place to answer:
- what the current repo-local layout contract is
- how docs are layered and where current truth lives
- which service is actively being migrated
- which Docker and verification rules must hold during the migration
- which command to rerun first after interruption

## Current state

Today this repo is in a transition state:
- the current active service name remains `meta-service`
- the target code layout is root `cmd/` plus root `internal/`
- the docs have moved to a layered structure under `docs/index/`, `docs/system/`, `docs/services/`, and `docs/policies/`
- the canonical repo-local verification entrypoint remains `bash scripts/verify.sh`

This repo does **not** currently treat `shared/` or `modules/meta-service` as the desired end state.
Those are migration surfaces to remove, not patterns to preserve.

## Build baseline

The target repository baseline is:
- module path: `github.com/bsonger/devflow-service`
- target Go version: `1.26.2`
- target builder/runtime contract: controlled base images with all installation behavior moved out of service Dockerfiles

Service Dockerfiles are expected to become packaging-only surfaces.

## Repo shape

The active target top-level layout is:
- `cmd/` — runnable process entrypoints only
- `internal/` — repo-private implementation
- `api/` — stable contracts such as OpenAPI or protobuf
- `deployments/` — deployment artifacts that belong in-repo
- `test/` — integration and e2e verification surfaces
- `docs/` — layered repo-local docs
- `scripts/` — repo-level verification and support scripts

## Read this first

If you are landing here cold, read in this order:
1. `AGENTS.md`
2. `docs/system/recovery.md`
3. `docs/system/architecture.md`
4. `docs/services/meta-service.md`
5. `docs/policies/docker-baseline.md` only if the task touches packaging, Docker, or CI
6. `docs/policies/verification.md` and `scripts/README.md` only if the task touches verification
7. `../devflow-control/docs/target-architecture/devflow-service.md` only if local docs are not enough for a migration-boundary question

## Docs layout

Use the docs tree by purpose:
- `docs/index/` — navigation only
- `docs/system/` — current repo-local truth
- `docs/services/` — current service-specific behavior and diagnostics
- `docs/policies/` — durable repo rules
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
docker build
bash scripts/verify.sh
```

The repo-level automation entrypoint for the same stack is:

```sh
make ci
```

The matching CI workflow lives at:

```text
.github/workflows/ci.yml
```
