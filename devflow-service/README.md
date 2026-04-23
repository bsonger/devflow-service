# DevFlow Service

`devflow-service` is the backend monorepo landing repository for DevFlow.
As of M005/S05 it has a real root Go module, extracted infrastructure-only shared packages, the first migrated owner-service under `modules/meta-service`, and a unified repo-local verification/recovery path for interruption-safe resume.

## Purpose

This repo gives a fresh engineer or agent one place to answer:
- what build baseline the backend monorepo currently uses
- which root surfaces are already real versus still staged
- where shared code may live before additional module migrations begin
- which Docker packaging contract future service work must follow
- which command proves the repo-local contract is still intact after interruption

## Current state

Today this repo contains:
- one root `go.mod` for the repository-wide baseline
- shared infrastructure packages under `shared/` (`httpx`, `loggingx`, `otelx`, `pyroscopex`, `observability`, `routercore`, `bootstrap`)
- the first migrated owner-service under `modules/meta-service`
- root documentation describing the monorepo contract, the Docker contract in `docs/docker.md`, and the recovery/observability contract in `docs/recovery.md`
- one canonical repo-local verification entrypoint in `scripts/verify.sh`

Today this repo does **not** yet contain:
- a root `go.work`
- per-service `go.mod` files
- generated artifacts or placeholder runtime services committed to the repo
- final deployment/runtime rollout completion for `meta-service`

## Build baseline

The repository baseline is now:
- module path: `github.com/bsonger/devflow-service`
- Go version: `1.25.8`
- Docker builder baseline: `registry.cn-hangzhou.aliyuncs.com/devflow/golang-builder:1.25.8`

`go 1.25.8` matches the published builder image tag in `../devflow-control/docker/golang-builder.Dockerfile` and supersedes the older `1.25.6` patch still present in sibling service repos.
Per D020 and this migration slice, the repo uses a **single root module first**; later slices can revisit the final workspace shape once more real module migrations exist.
Per D021, future service packaging also follows the controlled Docker baseline documented in `docs/docker.md` rather than ad hoc inline package installation inside service Dockerfiles.

## Monorepo shape

The intended top-level layout remains:
- `cmd/` — reserved for runnable process entrypoints only
- `modules/` — reserved for explicit owner-service migration targets
- `shared/` — infrastructure-only packages extracted for cross-module reuse
- `gateway/` — edge and Kong-facing backend surfaces
- `docs/` — monorepo-wide architecture, constraints, observability, and recovery guidance
- `scripts/` — repo-level verification and support scripts

Only `shared/` and `modules/meta-service` have real code in this slice.
That does **not** authorize putting owner-specific business logic into `shared/` or assuming the remaining services have already migrated.

## Extracted shared surface

The current extracted shared seam is:
- `shared/httpx` for response and pagination helpers already used by API handlers
- `shared/loggingx` for structured logger setup and context/request-id enrichment
- `shared/otelx` for OpenTelemetry tracer and metric-provider setup
- `shared/pyroscopex` for profiling bootstrap
- `shared/observability` for runtime observability initialization plus metrics/pprof servers and dependency-call instrumentation
- `shared/routercore` for Gin middleware, logging, request-id, recovery, and HTTP metrics helpers
- `shared/bootstrap` for service startup wiring that composes config load, runtime init, router run, and observability sidecars

These are the authoritative in-repo packages `modules/meta-service` already imports, which keeps the shared extraction real rather than aspirational.

## Read this first

If you are landing here cold, read in this order:
1. `docs/recovery.md`
2. `AGENTS.md`
3. `docs/README.md`
4. `docs/docker.md`
5. `docs/architecture.md`
6. `docs/constraints.md`
7. `docs/observability.md`
8. `scripts/README.md`
9. `modules/meta-service/README.md`

## What belongs elsewhere

This repository is future-state backend scope only.
Use sibling repos for authority that still lives outside this monorepo:
- `devflow-control` for migration governance and target-architecture history
- `devflow-platform-web` for frontend code and browser behavior

## Verification and recovery

Use `docs/recovery.md` as the single repository-local status and continuation surface.
It records the active slice goal, last-known verification sequence, the first command to rerun after interruption, requirement impact, and how to localize failures between repo verification, Docker policy, and the migrated service build.
Use `docs/docker.md` for the controlled image catalog, artifact-first packaging rule, and the explicit ban on inline install commands in future service Dockerfiles under `modules/`.

The last-known good verification sequence from the repo root is:

```sh
bash modules/meta-service/scripts/build.sh
bash scripts/check-docker-policy.sh
bash scripts/verify.sh
```

The canonical first rerun after interruption is:

```sh
bash scripts/verify.sh
```

That verifier checks the root module file, confirms the docs mention the root-module/shared extraction contract, asserts the required extracted shared packages are present, confirms `modules/meta-service` plus its `scripts/build.sh`, `scripts/regen-swagger.sh`, and `Dockerfile` exist, reruns Docker policy enforcement, reruns the real migrated-service build proof, and then runs `go test ./...` as the authoritative compile/test proof for the code already landed in this repo.
