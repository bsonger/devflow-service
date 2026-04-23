# AGENTS

## Startup
Read in this order:
1. `docs/recovery.md`
2. `README.md`
3. `docs/README.md`
4. `docs/docker.md`
5. `docs/architecture.md`
6. `docs/constraints.md`
7. `docs/observability.md`
8. `scripts/README.md`

Public API: not yet.
This repo currently owns the root Go module, extracted shared infrastructure packages, the repository-local Docker contract in `docs/docker.md`, and future backend landing zones for `cmd/`, `modules/`, and `gateway/`.
If ownership, migration authority, or final workspace-shape questions appear, go back to `../devflow/devflow-control/docs/target-architecture/devflow-service.md` and `../devflow/devflow-control/docs/target-architecture/devflow-service-migration-handoff.md`, but treat this repo's root-module contract as the current local execution truth for M005/S02 and its controlled Docker baseline as the current local execution truth for M005/S03.

## Commands
- `bash scripts/verify.sh`
- `go test ./...`
- inspect `docs/docker.md` before adding any future service Dockerfile under `modules/`

## Current repository rules
- Treat `go.mod` at the repo root as the active build contract.
- Use `go 1.25.8`; this matches the controlled builder image tag and supersedes sibling repos still on `1.25.6`.
- Follow `docs/docker.md` for the controlled Docker baseline: approved Aliyun registry references, artifact-first packaging, and the ban on inline install commands in future service Dockerfiles.
- Do not create `go.work` or per-service `go.mod` files in this slice.
- Do not create fake service implementations, placeholder binaries, or pretend verification output just to make the tree look complete.
- Keep owner-service boundaries explicit when real code migration begins; do not hide domain ownership inside `shared/` or `gateway/`.
- `shared/` is for infrastructure-only packages such as transport, bootstrap, router, and observability helpers that multiple future modules can import.
- The current extracted shared seam is `httpx`, `loggingx`, `otelx`, `pyroscopex`, `observability`, `routercore`, and `bootstrap`.

## Before handoff
- Rerun `bash scripts/verify.sh` from the repo root.
- Confirm `docs/recovery.md` still describes the repository phase, root module path, extracted shared packages, Docker contract, and pending work honestly.
- Confirm the root docs still describe the repository honestly and point readers to `docs/docker.md` before future service packaging work.
- Confirm no new `go.work`, per-service `go.mod`, fake `cmd/` binaries, or fake `modules/` code were introduced.

## When to go back to devflow-control
Go back when the task changes migration sequencing, ownership boundaries, gateway/governance expectations, or the eventual post-S02 workspace design.
