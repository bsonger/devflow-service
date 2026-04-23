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
- Slice: `S03`
- Slice goal: land the controlled per-service Docker baseline for future service migration work while preserving the real root-module baseline from S02
- Current task status in this slice:
  - `T01` in progress: define the monorepo Docker contract and controlled-image catalog

## What S02 established

S02 turned `devflow-service` from a docs-only skeleton into a real Go repository baseline.
The slice established these repository-local surfaces:
- root `go.mod` with module path `github.com/bsonger/devflow-service`
- root Go baseline `1.25.8`, matching the controlled builder image tag in `../devflow-control/docker/golang-builder.Dockerfile`
- extracted infrastructure-only shared packages under `shared/httpx`, `shared/loggingx`, `shared/otelx`, `shared/pyroscopex`, `shared/observability`, `shared/routercore`, and `shared/bootstrap`
- root docs and agent entrypoints updated to describe the root-module contract honestly
- one repo-local verifier entrypoint at `scripts/verify.sh` that checks the root-module/docs contract, asserts the extracted shared-package surfaces, and then runs `go test ./...`

This means a fresh reader can now diagnose whether the repo is missing build metadata, stale docs, missing shared-package surfaces, or a real Go test failure from inside this repo alone.

## What S03 now adds

S03 starts the repository-local Docker baseline before any migrated service code lands under `modules/`.
The current Docker contract surface is:
- `docs/docker.md` for the approved Aliyun registry, controlled builder baseline, runtime-image expectations, and artifact-first packaging rule
- root docs that point future agents to `docs/docker.md` before adding per-service Dockerfiles
- a documented ban on inline install commands such as `apk add`, `apk upgrade`, `apt-get`, `yum`, `dnf`, and `go install` inside future service Dockerfiles

This does **not** mean any migrated service already exists in this repo.
It means future slices must consume a documented Docker contract instead of inventing one during migration.

## Current local build contract

Treat the following as the active local truth for this repo:
- module path: `github.com/bsonger/devflow-service`
- Go version: `1.25.8`
- build model: **single root module**
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

These are the packages later service-migration slices should import instead of `devflow-service-common` when moving code into this monorepo.

## What is intentionally pending for S03/S04/S05

The following remain intentionally deferred:
- owner-service migrations under `modules/`
- runnable binaries under `cmd/`
- gateway/Kong implementation under `gateway/`
- repo-local Docker assets and verifier-enforced Docker policy checks planned for later S03 tasks
- any final post-migration workspace reshaping, if later slices choose to revisit it
- generated assets, fake APIs, or placeholder runtime behavior

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

## Canonical verification command

Run this from the `devflow-service` repo root:

```sh
bash scripts/verify.sh
```

This verifier is the canonical repo-local handoff check for S02 and the future extension point for S03 Docker-policy enforcement.
It fails fast and reports whether the repo is missing its root module, stale contract docs, required shared-package surfaces, or passing Go test proof.

## What `scripts/verify.sh` proves

A passing run means:
- `go.mod` exists at the repo root and is non-empty
- the root docs and recovery surfaces mention the root-module contract and the canonical verifier command
- the extracted shared infrastructure packages exist at the expected paths under `shared/`
- `go test ./...` passes for the code currently landed in the repo

A passing run does **not** mean migrated owner services, binaries, or gateway code already exist.
It only proves the root-module baseline and its documented recovery/verification contract are intact.
Until later S03 tasks land the static Docker-policy checker, `docs/docker.md` remains the human-readable source of truth for future per-service packaging rules.

## If verification fails

Use the first failing line to decide where to inspect next:
- missing `go.mod` or missing root-module literals → restore the root build contract and root docs
- missing shared package files → restore the extracted baseline packages under `shared/`
- failing `go test ./...` → inspect the failing package and fix the real compile/test regression
- stale recovery/doc references → rewire `README.md`, `AGENTS.md`, `docs/observability.md`, or `scripts/README.md`

Do not add fake code or fake binaries just to satisfy the verifier.
If the verifier reveals a real contract change, update code, docs, and verification together so the repository stays honest.
If a future service Dockerfile question arises before static policy enforcement lands, inspect `docs/docker.md` first.
