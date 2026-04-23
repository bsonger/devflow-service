# Observability

## Purpose

This repository currently exposes cold-start inspection surfaces plus a real compile/test signal.
In M005/S02, observability means a fresh reader or agent can determine the active root module, Go baseline, extracted shared packages, pending migration work, and next verification step without relying on prior session memory.
In M005/S03, that observability surface also includes the controlled Docker baseline in `docs/docker.md` so packaging-policy drift can be diagnosed from the repo root.

## Primary inspection surfaces

Use these files as the root observability surfaces:
- `docs/recovery.md` for current milestone/slice status, module path, Go baseline, extracted shared packages, pending work, and the next read/run order
- `README.md` for repo purpose and current monorepo baseline state
- `AGENTS.md` for startup order, constraints, and the preferred repo-local verification commands
- `docs/docker.md` for the controlled image catalog, artifact-first packaging rule, and banned inline-install patterns
- `docs/architecture.md` for module shape and boundary intent
- `docs/constraints.md` for what must not be created or collapsed yet
- `scripts/README.md` for the repo-local verifier contract
- `go.mod` for the active root build baseline

## Current verification signal

The current verification signal is a real repo-local verifier composed from local contract checks and Go tests:
- `go.mod` exists and declares the root module baseline
- root docs and recovery surfaces exist and mention the root-module contract
- shared package surfaces expected by this slice exist
- `go test ./...` passes as the authoritative compile/test proof for code currently landed in the repo

The preferred root check is:

```sh
bash scripts/verify.sh
```

A passing result means the repo still exposes the minimum root recovery and build-contract surfaces this slice owns and that the currently extracted shared packages compile and test successfully.

## Failure interpretation

A failing `scripts/verify.sh` should tell a future agent which class of drift occurred:
- missing `go.mod` or wrong root-module references → build-contract drift
- missing recovery/doc literals → stale repo-local handoff documentation
- missing shared package surfaces → extracted baseline code drift
- failing `go test ./...` → real compile/test regression in landed repo code

Because `shared/observability` and `shared/routercore` now provide the repo's first real runtime-oriented diagnostics, a failing compile/test run often also implies drift in metrics, pprof, tracing, panic recovery, or dependency instrumentation wiring.

## Future observability direction

Later slices should extend this inspection surface with:
- repo-wide migration checks once owner modules land
- root verification that composes module-level test suites
- Docker policy checks for missing contract files, controlled-image references, and banned inline-install patterns under `modules/`
- runtime-oriented observability once real binaries and services exist

Those additions should preserve the cold-start property: a fresh agent should still be able to find the preferred verification path from the repo root without external session context.
