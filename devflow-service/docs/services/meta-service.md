# Meta Service

`meta-service` is the current active service being migrated into the root `devflow-service` layout.

## Current role

The service preserves the existing runtime identity `meta-service` while the repository moves away from the older nested structure.
This migration changes package layout and repo-local contracts first.
It does not yet aim to rename the service or redesign its business behavior.

## Current migration target

The target local layout for this service is:
- `cmd/meta-service/main.go`
- business code under `internal/<domain>/{service,domain,repository,transport}`
- infrastructure code under `internal/platform/...`
- optional stable shared helpers under `internal/shared/...`
- packaging and verification contracts rooted at the repository root

`meta-service` no longer keeps any runtime Go packages under `modules/`.
Build, Swagger generation, and Docker packaging now run from root repo surfaces.
The current domain set in this repo is `application`, `project`, `cluster`, and `environment`.

## Current diagnostics

Use these surfaces when working on or diagnosing this service:
1. `AGENTS.md`
2. `docs/system/recovery.md`
3. `docs/system/architecture.md`
4. `docs/policies/docker-baseline.md`
5. `docs/policies/verification.md`
6. `scripts/README.md`
7. `docs/services/application.md`, `project.md`, `cluster.md`, and `environment.md` for current resource contracts

## Build and verification target

The service should eventually prove cleanly with:

```sh
go test ./...
go build -o bin/meta-service ./cmd/meta-service
docker build -t devflow-service:local -f Dockerfile .
```

During the migration, failures should be treated as real contract drift to fix, not as accepted transition noise.
