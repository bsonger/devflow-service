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
- Active migration: finish the remaining repo-root cleanup after moving `meta-service` into the repository root layout
- Active doc migration: move repo docs from a flat `docs/` layout into `docs/index/`, `docs/system/`, `docs/services/`, and `docs/policies/`
- Active runtime assembly: `cmd/meta-service` now boots through `internal/app` and `internal/platform/{config,db,runtime}`
- Active image packaging: root `Dockerfile` now performs a multi-stage build directly from `cmd/meta-service`

This repository is in an intentional transition state.
Current local docs are authoritative even when the code migration is not complete yet.

## First command to rerun after interruption

Run this first from the repo root:

```sh
bash scripts/verify.sh
```

Use it first because it is the canonical repo-local proof surface and should remain the highest-value recovery command during the migration.

## Current verification stack

The target repo-local verification stack is:

```sh
gofmt ./...
go vet ./...
golangci-lint run
go test ./...
go build -o bin/meta-service ./cmd/meta-service
docker build -t devflow-service:local -f Dockerfile .
bash scripts/verify.sh
```

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
6. `docs/policies/`
7. `scripts/README.md`

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

### If migration-boundary decisions are unclear

Inspect next:
1. `docs/system/architecture.md`
2. `../devflow-control/docs/target-architecture/devflow-service.md`
3. `../devflow-control/docs/target-architecture/devflow-service-migration-handoff.md`

## Guardrails during recovery

- Do not create a parallel recovery tracker.
- Do not reintroduce `shared/`, `common/`, or `util/` as catch-all directories.
- Do not add install commands into service Dockerfiles.
- Do not treat stale docs as acceptable during the migration; update docs and verification together.
