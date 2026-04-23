# Observability

## Purpose

This repository currently exposes inspection surfaces rather than runtime telemetry.
In bootstrap state, observability means a fresh reader or agent can determine the repository phase, reserved boundaries, and next verification step without needing prior session memory.

## Primary inspection surfaces

Use these files as the root observability surfaces:
- `docs/recovery.md` for current milestone/slice status, pending work, and the next read/run order
- `README.md` for repo purpose and current bootstrap state
- `AGENTS.md` for startup order, constraints, and the preferred repo-local verification command
- `docs/architecture.md` for monorepo shape and boundary intent
- `docs/constraints.md` for what must not be created or collapsed yet
- `scripts/README.md` for the repo-local verifier contract

## Current verification signal

The current verification signal is a real repo-local verifier composed from local and upstream checks:
- required root docs exist
- reserved top-level directories exist
- the repository-local recovery and verification entrypoints are wired from the root
- upstream frozen-doc verifiers still pass

The preferred bootstrap check is:

```sh
bash scripts/verify.sh
```

A passing result means the repo still exposes the minimum root recovery and navigation surfaces this slice owns and remains aligned with upstream migration authority.

## Future observability direction

Later slices should replace or extend structural inspection with real repo-local verification such as:
- verifier scripts under `scripts/`
- workspace validation once build files exist
- module-level verification wired into one root command

Those later additions should preserve the cold-start property: a fresh agent should still be able to find the preferred verification path from the repo root without external session context.
