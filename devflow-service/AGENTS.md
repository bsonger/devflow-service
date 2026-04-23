# AGENTS

## Startup
Read in this order:
1. `docs/recovery.md`
2. `README.md`
3. `docs/README.md`
4. `docs/architecture.md`
5. `docs/constraints.md`
6. `docs/observability.md`
7. `scripts/README.md`

Public API: not yet.
This repo currently owns the root Go module, shared infrastructure extraction surface, and future backend landing zones for `cmd/`, `modules/`, and `gateway/`.
If ownership, migration authority, or final workspace-shape questions appear, go back to `../devflow/devflow-control/docs/target-architecture/devflow-service.md` and `../devflow/devflow-control/docs/target-architecture/devflow-service-migration-handoff.md`, but treat this repo's root-module contract as the current local execution truth for M005/S02.

## Commands
- `bash scripts/verify.sh`
- `go test ./...`

## Current repository rules
- Treat `go.mod` at the repo root as the active build contract.
- Use `go 1.25.8`; this matches the controlled builder image tag and supersedes sibling repos still on `1.25.6`.
- Do not create `go.work` or per-service `go.mod` files in this slice.
- Do not create fake service implementations, placeholder binaries, or pretend verification output just to make the tree look complete.
- Keep owner-service boundaries explicit when real code migration begins; do not hide domain ownership inside `shared/` or `gateway/`.
- `shared/` is for infrastructure-only packages such as transport and observability helpers that multiple future modules can import.

## Before handoff
- Rerun `bash scripts/verify.sh` from the repo root.
- Confirm `docs/recovery.md` still describes the repository phase, root module path, and pending work honestly.
- Confirm the root docs still describe the repository honestly.
- Confirm no new `go.work`, per-service `go.mod`, fake `cmd/` binaries, or fake `modules/` code were introduced.

## When to go back to devflow-control
Go back when the task changes migration sequencing, ownership boundaries, gateway/governance expectations, or the eventual post-S02 workspace design.
