# AGENTS

## Purpose

This is the canonical startup and operating guide for agents working in `devflow-service`.
Read this before broad exploration.
Use it to decide what to read first, which local docs own the current fact, when to consult `devflow-control`, and how to continue the `meta-service` root migration without reviving the old layout or drifting away from the repo's Go monorepo rules.

## Canonical startup contract

`AGENTS.md` is the only canonical startup contract for agents in this repo.

Default startup read set:
1. this file: `AGENTS.md`
2. `docs/system/recovery.md`
3. `README.md`
4. `docs/system/architecture.md`
5. `docs/policies/go-monorepo-layout.md` if the task touches directories, layering, naming, imports, or package boundaries
6. `docs/policies/docker-baseline.md` only if the task touches build, image, packaging, or CI
7. `docs/policies/verification.md` and `scripts/README.md` only if the task touches verification or repo scripts
8. `../devflow-control/docs/target-architecture/devflow-service.md` only if the task changes migration boundaries or repo shape beyond current local docs
9. `../devflow-control/docs/target-architecture/devflow-service-migration-handoff.md` only if the task changes migration sequencing or needs future-state lane guidance

Do not read every doc by default.
Do not use `devflow-control` future-state docs to override this repo's active local execution contract.

## Reader outcome

After reading this file, a fresh engineer or agent should be able to:
- make normal repo changes without regressing the current contract
- continue the `meta-service` restructuring under the new root-level layout
- distinguish between this repo's current execution truth and `devflow-control`'s future-state guidance

## Authority ladder

When documents disagree, trust them in this order:
1. `docs/policies/go-monorepo-layout.md` for directory, layering, naming, and dependency rules
2. this repo's current implementation-facing docs and verification surfaces for current execution truth
3. this file as the canonical startup and routing contract
4. `../devflow-control/docs/target-architecture/*.md` for future-state migration boundaries only
5. `docs/superpowers/specs/` and `docs/superpowers/plans/` for historical design context only

If a conflict remains after reading the highest-authority current docs, stop and resolve it instead of guessing.

## Task-intent routing

### If the task is repo-local implementation work
Read:
1. `docs/system/recovery.md`
2. `README.md`
3. `docs/system/architecture.md`
4. `docs/policies/go-monorepo-layout.md`
5. one additional local doc only if the task needs it

### If the task is Docker, build-image, or CI work
Read:
1. `docs/system/recovery.md`
2. `docs/policies/docker-baseline.md`
3. `docs/policies/verification.md`
4. `scripts/README.md`

### If the task is verification or handoff hardening
Read:
1. `docs/system/recovery.md`
2. `docs/system/observability.md`
3. `docs/policies/verification.md`
4. `scripts/README.md`

### If the task is migration-boundary or future-shape work
Read:
1. `docs/system/recovery.md`
2. `docs/system/architecture.md`
3. `../devflow-control/docs/target-architecture/devflow-service.md`
4. `../devflow-control/docs/target-architecture/devflow-service-migration-handoff.md`

### If the task is doc-only governance inside this repo
Read:
1. `docs/README.md`
2. `docs/index/README.md`
3. the smallest local authoritative docs implicated by the change

## Current migration focus

- The current service name remains `meta-service`.
- The current migration target is to move `meta-service` into the repository root layout.
- Do not expand this work into future services during the current pass.
- Do not preserve old structure just to reduce diff size.

## Current local rules

- This repository root is the active `repo/` for current work.
- Use `cmd` to isolate process entrypoints.
- Use `internal` to isolate implementation.
- Use business domains to isolate ownership boundaries.
- Use `service`, `domain`, `repository`, and `transport` to control responsibilities.
- `internal/shared/` is allowed only for a small set of stable cross-domain helpers such as `errs`, `response`, `middleware`, and `idgen`.
- Do not create catch-all `common/`, `util/`, `base/`, or business-heavy `shared/` areas.
- Keep service Dockerfiles free of install commands; installation belongs in controlled base images.
- Keep behavior stable while restructuring; do not use this migration as cover for unrelated business logic rewrites.

## Before handoff

- Rerun the repo verification stack from the repo root.
- Confirm `AGENTS.md`, `README.md`, `docs/system/*`, `docs/services/*`, `docs/policies/*`, and `scripts/README.md` describe the same layout and command order.
- Confirm the active service still builds as `meta-service`.
- Confirm no catch-all `common/`, `util/`, or business-heavy `shared/` directories were reintroduced.

## When to go back to devflow-control

Go back to `devflow-control` when the task changes:
- migration sequencing across multiple future services
- ownership boundaries between domains
- gateway or Kong governance expectations
- future-state CI/governance contracts beyond the current repo migration
- the decision to reintroduce multi-module or workspace-level Go structure
