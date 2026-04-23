# DevFlow Service

`devflow-service` is the future backend monorepo for DevFlow.
It is the landing repository for the backend services that are currently split across multiple repos.

This repository is intentionally in bootstrap state during M005/S01.
It provides the root entrypoints, navigation surfaces, and monorepo skeleton that future migration slices will fill in.
It does **not** yet contain migrated service code, a root `go.mod`, a `go.work`, or per-module `go.mod` files.

## Purpose

This repo exists so a fresh engineer or agent can answer three questions immediately from inside the repository:
- what this monorepo is for
- what structure is already reserved
- what documents and verification entrypoints to use before making changes

## Current state

Today this repo contains:
- monorepo root documentation
- root-level navigation for future module and gateway areas
- placeholder directories for `cmd/`, `modules/`, `shared/`, and `gateway/`
- repo-local script guidance

Today this repo does **not** contain:
- migrated backend services
- generated artifacts
- runnable binaries
- workspace assembly files

## Planned monorepo shape

M005 uses a staged bootstrap.
Later slices will add the real Go workspace contract and migrated module contents.
The intended long-term shape is:
- root process entrypoints under `cmd/`
- explicit owner-service modules under `modules/`
- infrastructure-only shared code under `shared/`
- edge and Kong-facing configuration under `gateway/`

Per the M005 target architecture and D020 migration direction, the early migration path uses a **single root `go.mod` in later slices** before the repository converges on the final multi-module workspace contract.
This slice only reserves the structure and documents the rules; it does not create those files yet.

## Read this first

If you are landing here cold, read in this order:
1. `docs/recovery.md`
2. `AGENTS.md`
3. `docs/README.md`
4. `docs/architecture.md`
5. `docs/constraints.md`
6. `docs/observability.md`
7. `scripts/README.md`

## Directory guide

- `cmd/` — reserved for runnable process entrypoints only
- `modules/` — reserved for explicit owner-service migration targets
- `shared/` — reserved for infrastructure helpers absorbed from common backend packages
- `gateway/` — reserved for Kong and edge-facing backend surfaces
- `docs/` — monorepo-wide architecture, constraints, observability, and navigation
- `scripts/` — repo-level verification and support scripts

Each reserved top-level area includes a local README so a fresh reader can understand intent before code migration begins.

## What belongs elsewhere

This repository is future-state backend scope only.
Use sibling repos for current authority that still lives outside this monorepo:
- `devflow-control` for current-state system truth, target architecture authority, and migration governance
- `devflow-platform-web` for frontend code and browser-facing behavior

## Verification

Use `docs/recovery.md` as the repository-local status and continuation surface.
It records the current milestone/slice, what S01 established, what remains intentionally pending, and what a fresh agent should read or run next.

The canonical repo-local handoff check is:

```sh
bash scripts/verify.sh
```

This verifier checks the required local repository skeleton and recovery surfaces, then reruns the upstream frozen-doc verifiers in `devflow-control` so this repo stays aligned with migration authority.
