# Recovery

## Reader and outcome

This document is the single repository-local recovery authority for `devflow-service`.
After reading it, a fresh engineer or agent should know:
- the active migration goal
- which command to rerun first after interruption
- what the current verification stack is
- how to localize failures between docs, Docker policy, verification, and the ongoing `meta-service` migration

## Current migration state

- Active service: `meta-service`
- Additional migrated service entrypoint: `config-service`
- Additional migrated service entrypoint: `network-service`
- Additional migrated service entrypoint: `release-service`
- Additional migrated service entrypoint: `runtime-service`
- Active migration: finish the remaining repo-root cleanup after moving `meta-service` into the repository root layout
- Active config migration: `config-service` now boots from `cmd/config-service` and owns the extracted `AppConfig` and `WorkloadConfig` API surface
- Active network migration: `network-service` now boots from `cmd/network-service` and owns the extracted `Service` and `Route` API surface
- Active release migration: `release-service` now boots from `cmd/release-service` and owns the verify ingress/writeback paths that were previously modeled as `verify-service`
- Active runtime migration: `runtime-service` now boots from `cmd/runtime-service` and owns the extracted runtime inspection, runtime operation, and internal observer/index API surface
- Active doc migration: move repo docs from a flat `docs/` layout into `docs/index/`, `docs/system/`, `docs/services/`, `docs/resources/`, and `docs/policies/`
- Active runtime assembly: `cmd/meta-service` now boots through `internal/app` and `internal/platform/{config,db,runtime}`
- Active image packaging: root `Dockerfile` still defaults to a multi-stage build for `cmd/meta-service`, while non-default service image selection is hardcoded in committed Tekton manifests for `config-service`, `network-service`, `release-service`, and `runtime-service`
- Active database baseline: Kubernetes PostgreSQL now targets the parallel `database/pg18-next` cluster, with repo-managed bootstrap artifacts under `deployments/pre-production/database/`

This repository is in an intentional transition state.
Current local docs are authoritative even when the code migration is not complete yet.

## First command to rerun after interruption

Run this first from the repo root:

```sh
bash scripts/verify.sh
```

Use it first because it is the canonical repo-local proof surface and should remain the highest-value recovery command during the migration. It also carries the mechanical runtime-domain no-Postgres guard for `internal/runtime/**`, so rerunning it is the fastest way to catch accidental PostgreSQL reintroduction in the Kubernetes-first runtime path.

## Current verification stack

The target repo-local verification stack is:

```sh
gofmt ./...
go vet ./...
golangci-lint run
go test ./...
go build -o bin/meta-service ./cmd/meta-service
go build -o bin/config-service ./cmd/config-service
go build -o bin/network-service ./cmd/network-service
go build -o bin/release-service ./cmd/release-service
go build -o bin/runtime-service ./cmd/runtime-service
bash scripts/verify.sh
```

Local ad-hoc Docker builds are intentionally not part of the recovery proof stack.
When packaging work is involved, use the committed Tekton manifests under `deployments/tekton/` instead of treating a local Docker build as authoritative.

During the migration, some of those commands may still require path updates before they pass against the final root layout.
When that happens, fix the repo contract and verification together instead of treating failures as expected noise.

## Failure routing

### If `bash scripts/verify.sh` fails on missing docs or paths

Inspect next:
1. `AGENTS.md`
2. `README.md`
3. `docs/README.md`
4. `docs/system/`
5. `docs/services/`
6. `docs/resources/`
7. `docs/policies/`
8. `scripts/README.md`

### If Docker policy checks fail

Inspect next:
1. `docs/policies/docker-baseline.md`
2. `docs/policies/verification.md`
3. `scripts/check-docker-policy.sh`
4. the failing `Dockerfile`

### If compile, test, or build fails

Inspect next:
1. the failing package from output
2. `cmd/meta-service`
3. `internal/app`
4. `internal/platform/...`
5. the affected `internal/<domain>/...` package

### If PostgreSQL bootstrap or connectivity is unclear

Inspect next:
1. `docs/system/postgresql.md`
2. `deployments/pre-production/database/`
3. `deployments/pre-production/meta-service.yaml`
4. `deployments/pre-production/config-service.yaml`
5. `deployments/pre-production/network-service.yaml`
6. `deployments/pre-production/release-service.yaml`
7. `deployments/pre-production/runtime-service.yaml`

### If release writeback or observer callbacks fail

Inspect next:
1. `docs/system/release-writeback.md`
2. `internal/release/transport/http/router.go`
3. `internal/release/transport/http/writeback_support.go`
4. `internal/release/transport/http/release_writeback.go`
5. `internal/release/config/config.go`

### If migration-boundary decisions are unclear

Inspect next:
1. `docs/system/architecture.md`
2. `../devflow-control/docs/target-architecture/devflow-service.md`
3. `../devflow-control/docs/target-architecture/devflow-service-migration-handoff.md`

## Guardrails during recovery

- Do not create a parallel recovery tracker.
- Do not reintroduce catch-all `common/`, `util/`, or business-heavy `shared/` directories.
- Do not add install commands into service Dockerfiles.
- Do not treat stale docs as acceptable during the migration; update docs and verification together.
 not treat stale docs as acceptable during the migration; update docs and verification together.
