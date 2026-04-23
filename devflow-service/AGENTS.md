# AGENTS

## Startup
Read in this order:
1. `README.md`
2. `docs/README.md`
3. `docs/architecture.md`
4. `docs/constraints.md`
5. `docs/observability.md`
6. `scripts/README.md`

Public API: not yet.
This repo currently owns the monorepo root skeleton, root navigation docs, and future backend landing zones for `cmd/`, `modules/`, `shared/`, and `gateway/`.
If ownership, migration authority, or future-state boundary questions appear, go back to `../devflow/devflow-control/docs/target-architecture/devflow-service.md` and `../devflow/devflow-control/docs/target-architecture/devflow-service-migration-handoff.md`.

## Commands
- `test -f AGENTS.md && test -f README.md && test -f docs/architecture.md && test -f scripts/README.md && test -d cmd && test -d modules && test -d shared && test -d gateway`

## Current repository rules
- Treat this repository as a bootstrap monorepo skeleton until later M005 slices migrate real code.
- Do not create fake service implementations, placeholder binaries, or pretend verification output just to make the tree look complete.
- Do not create `go.work`, per-service `go.mod`, or final workspace assembly files in this slice.
- M005 will introduce a single root `go.mod` in later slices before the final workspace contract is completed.
- Keep owner-service boundaries explicit when real code migration begins; do not hide domain ownership inside `shared/` or `gateway/`.

## Before handoff
- Rerun the structural verification command above.
- Confirm the root docs still describe the repository honestly.
- Confirm reserved directories still align with the target monorepo scope.

## When to go back to devflow-control
Go back when the task changes monorepo ownership boundaries, migration sequencing, route/governance expectations, or the documented future-state repository contract.
