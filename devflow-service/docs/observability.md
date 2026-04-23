# Observability

## Purpose

This repository exposes interruption-safe inspection and verification surfaces for a fresh engineer or agent.
In M005/S05, observability means a reader can enter `devflow-service`, find the single recovery authority in `docs/recovery.md`, rerun the canonical verifier, and localize whether drift is in repo docs/structure, Docker policy, migrated-service build proof, or root compile/test proof without relying on external session memory.

## Primary inspection surfaces

Use these files as the root observability surfaces:
- `docs/recovery.md` for the active milestone/slice, last-known verification sequence, the first rerun command after interruption, requirement impact, and failure-routing guidance
- `README.md` for repo purpose and current monorepo baseline state
- `AGENTS.md` for startup order, constraints, and canonical command order
- `docs/docker.md` for the controlled image catalog, artifact-first packaging rule, and banned inline-install patterns
- `docs/architecture.md` for module shape and boundary intent
- `docs/constraints.md` for what must not be created or collapsed yet
- `scripts/README.md` for the composed repo-level verifier contract
- `modules/meta-service/README.md` for the migrated service's local build/package/diagnostic surfaces
- `go.mod` for the active root build baseline

`docs/recovery.md` remains the single recovery authority. The other surfaces should align to it, not replace it.

## Current verification signal

The current verification signal is a real composed proof surface, not a docs-only checklist.
Run these commands from the repo root:

```sh
bash modules/meta-service/scripts/build.sh
bash scripts/check-docker-policy.sh
bash scripts/verify.sh
```

The preferred first rerun after interruption is still:

```sh
bash scripts/verify.sh
```

That command is the canonical repo-local handoff and recovery proof because it:
- checks root docs, directories, and required literals
- reruns Docker policy enforcement via `bash scripts/check-docker-policy.sh`
- reruns the real migrated-service build via `bash modules/meta-service/scripts/build.sh`
- reruns `go test ./...` as the authoritative compile/test proof

A passing result means the repo still exposes the minimum root recovery surface this slice owns, the controlled Docker baseline still holds, `modules/meta-service` still builds truthfully from the repo root, and the currently landed Go code still compiles/tests honestly.

## Failure interpretation

Use the first failing command or the first failing verifier line to localize the regression.

- `bash modules/meta-service/scripts/build.sh` fails → service-build or artifact-staging drift inside `modules/meta-service`; inspect `modules/meta-service/README.md`, `scripts/build.sh`, `scripts/regen-swagger.sh`, `Dockerfile`, then the failing package
- `bash scripts/check-docker-policy.sh` fails → Docker contract drift; inspect `docs/docker.md`, `scripts/check-docker-policy.sh`, and the reported `modules/**/Dockerfile*`
- `bash scripts/verify.sh` fails before the build step → root structure/doc drift; inspect `README.md`, `AGENTS.md`, `docs/recovery.md`, `docs/observability.md`, `scripts/README.md`, and the specific missing path or literal
- `bash scripts/verify.sh` fails during the build step → rerun `bash modules/meta-service/scripts/build.sh` directly and debug the migrated service build surface
- `bash scripts/verify.sh` fails at `go test ./...` → real compile/test regression in landed code; inspect the reported package under `shared/...` or `modules/meta-service/...`

Because `shared/observability` and `shared/routercore` provide the repo's main runtime-oriented diagnostics, compile/test regressions may also indicate drift in logging, metrics, tracing, panic recovery, pprof, or dependency-call instrumentation wiring.

## Requirement and slice impact

This observability surface supports the current slice goal by making these truths inspectable from inside the repo:
- `R049` is advanced because interruption recovery now points to one truthful repo-local authority and one default rerun command
- `R050` is advanced because verifier failures route directly to the next inspection command
- `R047` and `R052` remain preserved because the repo-level proof and the migrated-service proof are both rerunnable from repo-local state

## Future observability direction

Later slices may extend this surface with:
- repo-wide migration checks as additional owner modules land beyond `meta-service`
- root verification that composes more module-level test suites
- deployment/runtime-oriented observability once real binaries and services are wired for rollout

Those additions should preserve the same cold-start property: a fresh agent should still be able to find the preferred verification path from the repo root with no external session context.
