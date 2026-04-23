# Recovery

## Reader and outcome

This document is the single repository-local recovery authority for `devflow-service`.
After reading it, a fresh engineer or agent should know:
- the active milestone and slice goal
- what the last-known good verification sequence is
- which command to rerun first after interruption
- how to localize failures between repo verification, Docker policy, and the migrated service build
- which docs to read next before making changes
- how this slice advances the current requirements without relying on external session memory

## Current phase ownership

- Milestone: `M005`
- Slice: `S05`
- Slice goal: unify the repository-level and service-level verification path in `devflow-service` and refresh the repository-local recovery contract so a fresh agent can resume from truthful in-repo state after interruption
- Current task status in this slice:
  - `T01` complete: composed the root verifier with the real `modules/meta-service/scripts/build.sh` proof and refreshed `scripts/README.md`
  - `T02` active: refresh recovery/observability/startup docs so they all describe the same interruption-safe command order and failure-routing guidance

## What remains true from earlier slices

Earlier slices already established these repo-local truths:
- root `go.mod` exists with module path `github.com/bsonger/devflow-service`
- the repository uses a single root module baseline at `go 1.25.8`
- extracted infrastructure-only shared packages live under `shared/httpx`, `shared/loggingx`, `shared/otelx`, `shared/pyroscopex`, `shared/observability`, `shared/routercore`, and `shared/bootstrap`
- the first migrated owner-service lives under `modules/meta-service`
- the controlled Docker contract is documented in `docs/docker.md` and enforced by `scripts/check-docker-policy.sh`
- the canonical repository-local proof entrypoint is `bash scripts/verify.sh`

Those truths remain active in S05. This slice does not replace them; it makes them easier to resume and re-prove after interruption from inside this repo alone.

## What S05 changes

S05 closes the gap between the repo-level verifier and the real service-level build surface.
The canonical verifier is now a composed proof, not just a structural check:
1. repo-local contract checks and required literal assertions in `scripts/verify.sh`
2. Docker policy enforcement via `bash scripts/check-docker-policy.sh`
3. real migrated-service build proof via `bash modules/meta-service/scripts/build.sh`
4. final root compile/test proof via `go test ./...`

That means a fresh reader can now enter `devflow-service`, rerun the canonical proof from the repo root, and tell whether drift is in the docs/structure layer, Docker policy layer, service build layer, or root compile/test layer.

## Requirement impact

This slice now advances these requirements directly:
- `R049`: repo-local recovery is truthful and interruption-safe because this document names the active slice goal, current proof order, and next command to rerun
- `R050`: observability and failure triage are repo-local because verifier failures now route to the next inspection command without requiring outside memory

This slice also preserves previously established proof:
- `R047`: the repository-level verification path remains real and rerunnable from the repo root
- `R052`: the migrated-service proof remains honest because `bash scripts/verify.sh` now composes the real `modules/meta-service/scripts/build.sh` build surface rather than only checking file presence

## Cold-start read order

If you are landing here cold, read in this order:
1. `docs/recovery.md`
2. `README.md`
3. `AGENTS.md`
4. `docs/README.md`
5. `docs/docker.md`
6. `docs/architecture.md`
7. `docs/constraints.md`
8. `docs/observability.md`
9. `scripts/README.md`
10. `modules/meta-service/README.md`

Only go back to `../devflow/devflow-control/docs/target-architecture/` if the task changes migration sequencing, ownership boundaries, or the intended final workspace shape. For local execution and recovery, this repo's docs are the authoritative truth.

## Last-known good verification sequence

Run these commands from the `devflow-service` repo root:

```sh
bash modules/meta-service/scripts/build.sh
bash scripts/check-docker-policy.sh
bash scripts/verify.sh
```

All three commands passed together in the last-known good state for S05.

The first two are the fastest drill-down checks for the migrated service and Docker contract.
The third command is the canonical composed proof surface for handoff and interruption recovery.

## First command to rerun after interruption

After any interruption, rerun this first from the repo root:

```sh
bash scripts/verify.sh
```

Use it first because it is the highest-value recovery command:
- it rechecks the root documentation and structure contract
- it reruns Docker policy enforcement
- it reruns the real `modules/meta-service` build proof
- it reruns `go test ./...`
- it fails fast with the earliest broken surface, which keeps diagnosis local and deterministic

If you only suspect `meta-service` packaging/build drift, you may start with `bash modules/meta-service/scripts/build.sh`, but the default recovery command is still `bash scripts/verify.sh`.

## Failure routing

Use the first failing command or first failing verifier line to choose the next inspection command.

### If `bash modules/meta-service/scripts/build.sh` fails

This means the service-local build surface is broken before or during binary/artifact staging.
Inspect next:
1. `modules/meta-service/README.md`
2. `modules/meta-service/scripts/build.sh`
3. `modules/meta-service/scripts/regen-swagger.sh`
4. `modules/meta-service/Dockerfile`
5. the failing package under `modules/meta-service/...`

Typical causes:
- root-module import drift affecting `./modules/meta-service/cmd`
- optional Swagger regeneration noise versus a real build error
- missing host certificate source used for staging
- missing tracked module assets or a broken staging path

### If `bash scripts/check-docker-policy.sh` fails

This means Docker packaging drift violated the controlled contract.
Inspect next:
1. `docs/docker.md`
2. `scripts/check-docker-policy.sh`
3. the reported `modules/**/Dockerfile*` path, currently `modules/meta-service/Dockerfile`

Typical causes:
- unapproved `FROM` reference
- inline install commands such as `apk add`, `apt-get install`, `go install`, `curl | sh`, or similar package/bootstrap drift
- packaging steps that no longer match the artifact-first model

### If `bash scripts/verify.sh` fails before the build step

This means repo-local structural or documentation drift occurred.
Inspect next:
1. `README.md`
2. `AGENTS.md`
3. `docs/README.md`
4. `docs/recovery.md`
5. `docs/observability.md`
6. `scripts/README.md`
7. any path named in the failing verifier message

Typical causes:
- stale literals about the root module or verifier command
- missing required root docs or directories
- missing shared package surfaces under `shared/`
- missing migrated-service surfaces under `modules/meta-service/`

### If `bash scripts/verify.sh` fails during the build step

This is equivalent to a `modules/meta-service` build regression surfaced through the canonical repo verifier.
Use the same drill-down path as the direct service-build failure:

```sh
bash modules/meta-service/scripts/build.sh
```

Then inspect `modules/meta-service/README.md`, `modules/meta-service/scripts/build.sh`, and the failing package or staging path.

### If `bash scripts/verify.sh` fails at `go test ./...`

This means the repo structure and service build still exist, but the landed code no longer compiles or tests cleanly.
Inspect next:
1. the specific failing package from test output
2. any changed shared package under `shared/...`
3. any changed service package under `modules/meta-service/...`

Do not treat this as a docs-only failure. Fix the real compile/test regression, then rerun `bash scripts/verify.sh`.

## What the canonical verifier proves

A passing `bash scripts/verify.sh` means:
- required root docs and directories exist and remain non-empty
- root entrypoints still point readers at `docs/recovery.md` and `bash scripts/verify.sh`
- the root module path and Go baseline remain truthful
- expected shared baseline packages still exist under `shared/`
- `modules/meta-service` still exposes its README, build script, Swagger helper, Dockerfile, and router identity test
- Docker policy enforcement passes
- the real migrated-service build still succeeds
- `go test ./...` still passes for the currently landed repo code

A pass does **not** claim that all future owner-service migrations, top-level binaries, or rollout work are complete.
It proves the current S05 repo-local recovery and verification contract remains truthful and rerunnable.

## Guardrails during recovery

- Do not create a parallel status file or separate recovery tracker; this document remains the single recovery authority
- Do not add fake binaries, fake generated assets, or placeholder modules to satisfy the verifier
- Do not introduce `go.work` or per-service `go.mod` files in this phase
- Keep `shared/` infrastructure-only; do not hide owner-service logic there
- If the contract changes, update docs and verification together so the repository stays honest
