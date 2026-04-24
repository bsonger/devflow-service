# Verification Policy

This document defines the target verification contract for `devflow-service`.

## Canonical proof stack

Run these commands from the repo root:

```sh
gofmt ./...
go vet ./...
golangci-lint run
go test ./...
go build -o bin/meta-service ./cmd/meta-service
docker build
bash scripts/verify.sh
```

The repo-level convenience target for the same contract is:

```sh
make ci
```

The matching CI workflow is:

```text
.github/workflows/ci.yml
```

## Expectations

- formatting, vet, lint, tests, build, Docker build, and repo verification must agree
- repo docs and verification must describe the same paths and command order
- failures are real contract drift to fix, not accepted migration noise

## Verification ownership

- `scripts/verify.sh` is the canonical repo-local verification entrypoint
- `scripts/check-docker-policy.sh` enforces Docker policy
- `scripts/README.md` explains script behavior and side effects
