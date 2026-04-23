# Observability

## Purpose

This repository currently exposes inspection surfaces rather than runtime telemetry.
In bootstrap state, observability means a fresh reader or agent can determine the repository phase, reserved boundaries, and next verification step without needing prior session memory.

## Primary inspection surfaces

Use these files as the root observability surfaces:
- `README.md` for repo purpose and current bootstrap state
- `AGENTS.md` for startup order, constraints, and the preferred structural verification command
- `docs/architecture.md` for monorepo shape and boundary intent
- `docs/constraints.md` for what must not be created or collapsed yet
- `scripts/README.md` for current script status and the expected future verifier path

## Current verification signal

The current verification signal is structural presence:
- required root docs exist
- reserved top-level directories exist
- the repo advertises a concrete verification command from the root

The preferred bootstrap check is:

```sh
test -f AGENTS.md && test -f README.md && test -f docs/architecture.md && test -f scripts/README.md && test -d cmd && test -d modules && test -d shared && test -d gateway
```

A passing result means the repo still exposes the minimum root recovery and navigation surfaces this slice owns.

## Future observability direction

Later slices should replace or extend structural inspection with real repo-local verification such as:
- verifier scripts under `scripts/`
- workspace validation once build files exist
- module-level verification wired into one root command

Those later additions should preserve the cold-start property: a fresh agent should still be able to find the preferred verification path from the repo root without external session context.
