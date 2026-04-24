# Observability

## Purpose

This repository exposes interruption-safe inspection and verification surfaces for a fresh engineer or agent.
Observability here means a reader can enter `devflow-service`, find the local recovery authority, rerun the verification stack, and localize whether drift is in docs, Docker policy, repo verification, or the ongoing `meta-service` migration.

## Primary inspection surfaces

Use these files as the primary repo-local observability surfaces:
- `AGENTS.md` for startup and routing
- `docs/system/recovery.md` for current recovery and failure routing
- `docs/system/architecture.md` for current repo-local structure
- `docs/services/meta-service.md` for current service-specific behavior and diagnostics
- `docs/policies/docker-baseline.md` for packaging and base-image rules
- `docs/policies/verification.md` for the target proof stack
- `scripts/README.md` for repo script behavior
- `.github/workflows/ci.yml` for the automated proof surface
- `go.mod` for the active Go baseline

## Verification signal

The target verification signal for this repo is:

```sh
gofmt ./...
go vet ./...
golangci-lint run
go test ./...
go build -o bin/meta-service ./cmd/meta-service
docker build
bash scripts/verify.sh
```

A passing result means:
- docs and verification agree on the active repo contract
- Docker policy holds
- the current code compiles and tests
- `meta-service` builds from the root layout

## Failure interpretation

- docs/path failure -> inspect `AGENTS.md`, `docs/system/*`, `docs/services/*`, `docs/policies/*`
- Docker failure -> inspect `docs/policies/docker-baseline.md`, `scripts/check-docker-policy.sh`, and the failing `Dockerfile`
- compile or test failure -> inspect the failing package under `cmd/` or `internal/`
- migration-boundary ambiguity -> inspect local system docs first, then `devflow-control` target docs

## Future direction

As the migration proceeds, this observability surface should become simpler, not more layered.
The end state should be one honest startup contract, one honest verification contract, and one honest root build path.
