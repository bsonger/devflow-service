# DevFlow Service

`devflow-service` is the backend monorepo landing repository for DevFlow.
As of M005/S02 it now has a real root Go module and the first extracted infrastructure-only shared packages, while owner-service migrations under `modules/` remain intentionally pending.

## Purpose

This repo gives a fresh engineer or agent one place to answer:
- what build baseline the backend monorepo currently uses
- which root surfaces are already real versus still staged
- where shared code may live before module migrations begin
- which command proves the repo-local contract is still intact

## Current state

Today this repo contains:
- one root `go.mod` for the repository-wide baseline
- shared infrastructure packages under `shared/` (`httpx`, `loggingx`)
- root documentation describing the monorepo contract and pending migration work
- one repo-local verification entrypoint in `scripts/verify.sh`

Today this repo does **not** yet contain:
- owner-service code migrated under `modules/`
- runnable binaries under `cmd/`
- a root `go.work`
- per-service `go.mod` files
- generated artifacts or placeholder runtime services

## Build baseline

The repository baseline is now:
- module path: `github.com/bsonger/devflow-service`
- Go version: `1.25.8`

`go 1.25.8` matches the published builder image tag in `../devflow-control/docker/golang-builder.Dockerfile` and supersedes the older `1.25.6` patch still present in sibling service repos.
Per D020 and this migration slice, the repo uses a **single root module first**; later slices can revisit the final workspace shape once real module migrations exist.

## Monorepo shape

The intended top-level layout remains:
- `cmd/` — reserved for runnable process entrypoints only
- `modules/` — reserved for explicit owner-service migration targets
- `shared/` — infrastructure-only packages extracted for cross-module reuse
- `gateway/` — edge and Kong-facing backend surfaces
- `docs/` — monorepo-wide architecture, constraints, observability, and recovery guidance
- `scripts/` — repo-level verification and support scripts

Only `shared/` has real code in this slice.
That does **not** authorize putting owner-specific business logic there.

## Read this first

If you are landing here cold, read in this order:
1. `docs/recovery.md`
2. `AGENTS.md`
3. `docs/README.md`
4. `docs/architecture.md`
5. `docs/constraints.md`
6. `docs/observability.md`
7. `scripts/README.md`

## What belongs elsewhere

This repository is future-state backend scope only.
Use sibling repos for authority that still lives outside this monorepo:
- `devflow-control` for migration governance and target-architecture history
- `devflow-platform-web` for frontend code and browser behavior

## Verification

Use `docs/recovery.md` as the repository-local status and continuation surface.
It records the current milestone/slice, the chosen Go baseline and root module path, and what work is still pending for S03/S04/S05.

The canonical repo-local check is:

```sh
bash scripts/verify.sh
```

The verifier now checks the root module file, confirms the docs mention the root-module/shared extraction contract, and then runs `go test ./...` as the authoritative compile/test proof for the code already landed in this repo.
