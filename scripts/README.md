# Scripts

This directory contains repo-level verification and support scripts.

## Reader and outcome

This guide is for a fresh engineer or agent landing in `devflow-service`.
After reading it, the reader should know which repo-local script to run first and what that script is supposed to prove during the migration.

## Canonical verifier

Run this from the repo root before handoff or after changing docs, verification rules, Docker policy, repo layout, or build paths:

```sh
bash scripts/verify.sh
```

This remains the canonical repo-local handoff check while the repository migrates from the older nested shape to the root `cmd/` and `internal/` layout.

## What `verify.sh` should prove

The verifier should fail fast and prove:
- repo-local startup and docs surfaces exist under the layered docs structure
- the active Go baseline matches the current contract
- Docker policy is enforced from the policy docs and script checks
- `meta-service` builds from the active root layout
- `release-service` builds from the active root layout with verify ingress absorbed into it
- `go test ./...` still passes

The target proof stack for the repo is:

```sh
gofmt ./...
go vet ./...
golangci-lint run
go test ./...
go build -o bin/meta-service ./cmd/meta-service
go build -o bin/release-service ./cmd/release-service
docker build -t devflow-service:local -f Dockerfile .
bash scripts/verify.sh
```

The repo-level convenience entrypoint is:

```sh
make ci
```

The Docker image build wrapper is:

```sh
bash scripts/docker-build.sh devflow-service:local Dockerfile
```

The matching automated workflow is:

```text
.github/workflows/ci.yml
```

## What this verifier should not claim

`verify.sh` should not pretend that the migration is already complete while old paths are still in use.
It should verify the active local contract honestly.

## Related docs

- `docs/policies/verification.md`
- `docs/policies/docker-baseline.md`
- `docs/system/recovery.md`
