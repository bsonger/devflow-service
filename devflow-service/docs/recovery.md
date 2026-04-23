# Recovery

## Reader and outcome

This document is for a fresh engineer or agent landing in `devflow-service` without prior session memory.
After reading it, the reader should know:
- what M005/S02 already established
- which root build contract is currently real
- which Docker packaging contract now applies to future service migrations
- what remains intentionally pending for S03/S04/S05
- which command to run next to verify the repository-local contract
- which docs to read in order before changing the repo

## Current phase ownership

- Milestone: `M005`
- Slice: `S04`
- Slice goal: migrate the first real owner-service as `modules/meta-service`, retarget it onto the extracted shared packages, and add truthful local build/package/verification surfaces
- Current task status in this slice:
  - `T01` complete: migrated the former `devflow-app-service` code into `modules/meta-service` under the root module and retargeted imports to `shared/...`
  - `T02` in progress: add service-local build/package surfaces and extend repo-local verification/recovery docs to prove the migration honestly

## What S02 and S03 established

S02 turned `devflow-service` from a docs-only skeleton into a real Go repository baseline, and S03 added the controlled Docker policy that future service packaging must follow.
Those slices established these repository-local surfaces:
- root `go.mod` with module path `github.com/bsonger/devflow-service`
- root Go baseline `1.25.8`, matching the controlled builder image tag in `../devflow-control/docker/golang-builder.Dockerfile`
- extracted infrastructure-only shared packages under `shared/httpx`, `shared/loggingx`, `shared/otelx`, `shared/pyroscopex`, `shared/observability`, `shared/routercore`, and `shared/bootstrap`
- root docs and agent entrypoints updated to describe the root-module contract honestly
- one repo-local verifier entrypoint at `scripts/verify.sh`
- the Docker contract in `docs/docker.md` plus static policy enforcement in `scripts/check-docker-policy.sh`

This means a fresh reader can now diagnose whether the repo is missing build metadata, stale docs, missing shared-package surfaces, Docker policy drift, or a real Go test failure from inside this repo alone.

## What S04 now adds

S04 makes the first owner-service migration real instead of hypothetical.
The current migrated-service surface is:
- `modules/meta-service/` containing the former `devflow-app-service` code retargeted onto `shared/...`
- `modules/meta-service/scripts/build.sh` for deterministic Linux artifact staging under `.build/staging/meta-service/`
- `modules/meta-service/Dockerfile` for artifact-first scratch packaging using approved `FROM` references only
- `modules/meta-service/README.md` documenting what assets are real now versus still deferred
- repo-local verification and recovery docs that explicitly fail if the migrated service, its build script, or its Dockerfile disappear

This still does **not** mean deployment/runtime rollout is complete.
It means the first migrated owner-service is now present, builds truthfully from this repository, and is visible in repo-local diagnosis surfaces.

## Current local build contract

Treat the following as the active local truth for this repo:
- module path: `github.com/bsonger/devflow-service`
- Go version: `1.25.8`
- build model: **single root module**
- migrated owner-service present: `modules/meta-service`
- service-local build surface: `modules/meta-service/scripts/build.sh`
- service-local packaging surface: `modules/meta-service/Dockerfile`
- Docker builder baseline: `registry.cn-hangzhou.aliyuncs.com/devflow/golang-builder:1.25.8`

This deliberately supersedes the older `1.25.6` patch still visible in sibling service repos.
It also means this slice does **not** use `go.work` or per-service `go.mod` files yet.
If older memory entries mention a `go.work` or multi-module baseline here, treat them as stale until a later slice updates the repo-local contract explicitly.

## Current extracted shared seam

The authoritative shared infrastructure packages now in-repo are:
- `shared/httpx` for response, list, error, and pagination helpers used by API handlers
- `shared/loggingx` for zap logger setup plus request-id and tracing context enrichment
- `shared/otelx` for OpenTelemetry tracer and metric-provider setup
- `shared/pyroscopex` for Pyroscope bootstrap
- `shared/observability` for runtime initialization, metrics/pprof servers, and dependency-call instrumentation
- `shared/routercore` for shared Gin middleware including request logging, panic recovery, request IDs, and HTTP metrics
- `shared/bootstrap` for service startup orchestration that wires config load, runtime init, ports, router launch, and sidecar observability servers

These are the packages the migrated `meta-service` now imports instead of `devflow-service-common`.

## What is intentionally pending for S04/S05

The following remain intentionally deferred:
- remaining owner-service migrations under `modules/`
- runnable binaries under top-level `cmd/`
- gateway/Kong implementation under `gateway/`
- final deployment/runtime rollout proof beyond the service-local build/package surfaces already added for `meta-service`
- any final post-migration workspace reshaping, if later slices choose to revisit it
- committed generated artifacts, fake APIs, or placeholder runtime behavior

Do not pre-create those surfaces just to make the repo look more complete.

## Read this next

If you are landing here cold, read in this order:
1. `README.md` — repo purpose and current root-module baseline
2. `AGENTS.md` — startup rules and canonical verification commands
3. `docs/README.md` — docs map
4. `docs/docker.md` — controlled Docker baseline and future service-packaging policy
5. `docs/architecture.md` — monorepo shape and ownership boundaries
6. `docs/constraints.md` — what must not be created yet
7. `docs/observability.md` — inspection and verification surfaces
8. `scripts/README.md` — repo-local verifier contract

If migration-history or long-term target-architecture questions remain after that, consult `../devflow/devflow-control/docs/target-architecture/` and note any divergence from the current repo-local contract before changing code.

## Canonical verification commands

Run these from the `devflow-service` repo root:

```sh
bash modules/meta-service/scripts/build.sh
bash scripts/check-docker-policy.sh
bash scripts/verify.sh
```

`bash scripts/verify.sh` remains the canonical repo-local handoff check.
The service-local build and Docker-policy commands are the first drill-down surfaces when the migrated service itself is in doubt.

## What `scripts/verify.sh` proves

A passing run means:
- `go.mod` exists at the repo root and is non-empty
- the root docs and recovery surfaces mention the root-module contract and the canonical verifier command
- the extracted shared infrastructure packages exist at the expected paths under `shared/`
- `modules/meta-service/` exists and includes `scripts/build.sh`, `Dockerfile`, and `README.md`
- repo-local docs mention the migrated `meta-service` boundary and the fact that it consumes `shared/...`
- `go test ./...` passes for the code currently landed in the repo

A passing run does **not** mean deployment/runtime rollout is complete for `meta-service`.
It proves the first migrated service is present, its documented build/package surfaces exist, and the root-module/shared baseline still compiles and tests honestly.

## If verification fails

Use the first failing line to decide where to inspect next:
- missing `go.mod` or missing root-module literals → restore the root build contract and root docs
- missing shared package files → restore the extracted baseline packages under `shared/`
- missing `modules/meta-service` surfaces → restore the migrated service, its `scripts/build.sh`, `Dockerfile`, or `README.md`
- failing `go test ./...` → inspect the failing package and fix the real compile/test regression
- stale recovery/doc references → rewire `README.md`, `docs/recovery.md`, `docs/architecture.md`, or `scripts/README.md`
- Docker policy failure → inspect `modules/meta-service/Dockerfile` against `docs/docker.md` and `scripts/check-docker-policy.sh`

Do not add fake code or fake binaries just to satisfy the verifier.
If the verifier reveals a real contract change, update code, docs, and verification together so the repository stays honest.
