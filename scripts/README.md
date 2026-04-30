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
- production code under `internal/*/service` does not bypass repository boundaries with direct DB access
- active code and docs do not retain Mongo-era dependency or naming remnants after the PostgreSQL migration
- API error envelopes and handler mappings stay aligned with the error-handling policy
- HTTP handlers stay aligned with the shared handler policy for response helpers, pagination, and HTTP-edge parsing
- write-side selector location stays aligned with the resource API policy: `GET` may use query filters, while `POST` and `DELETE` selectors must use JSON body fields
- HTTP handlers reuse shared UUID parsing helpers from `internal/platform/httpx` instead of repeating local `uuid.Parse(...)` error handling
- HTTP handlers reuse shared `BindJSON`, shared pagination helpers, and stable `internal error` response helpers from `internal/platform/httpx`
- HTTP handlers prefer specialized `httpx` helpers such as `WriteInvalidArgument`, `WriteFailedPrecondition`, and `WriteUnauthorized` instead of repeating equivalent `WriteError(...)` envelopes
- service-layer code stays aligned with the service-layer policy and does not depend on Gin, `httpx`, or HTTP transport packages
- downstream runtime-boundary clients stay aligned with the downstream-client policy and reuse shared downstream HTTP behavior
- HTTP-based runtime lookup code in service, support, or runtime packages reuses `internal/shared/downstreamhttp` instead of hand-rolled `net/http` clients
- repository packages stay aligned with the repository-layer policy and do not depend on Gin, `httpx`, handler packages, or service packages
- service and repository generic validation errors reuse `internal/shared/errs` instead of repeating ad-hoc required-field strings
- runtime helper packages stay aligned with the worker-runtime policy and do not depend on Gin, `httpx`, or HTTP handler packages
- resource-facing handler behavior and `docs/resources/*.md` stay aligned with the resource-api policy
- structured log field names under `cmd/` and `internal/` stay aligned to the observability logging policy
- metric attribute labels under `cmd/` and `internal/` do not use forbidden high-cardinality or sensitive identifiers
- alias-only forwarding files such as `support_alias.go` are not reintroduced
- `internal/shared` does not accumulate catch-all directory names such as `common`, `util`, `utils`, `base`, or `model`
- `internal/platform` and `internal/shared` do not import business-domain packages directly
- `meta-service` builds from the active root layout
- `config-service` builds from the active root layout
- `network-service` builds from the active root layout
- `release-service` builds from the active root layout with verify ingress absorbed into it
- `runtime-service` builds from the active root layout for extracted runtime APIs
- `go test ./...` still passes

The target proof stack for the repo is:

```sh
make fmt-check
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

The repo-level convenience entrypoint is:

```sh
make ci
```

Local ad-hoc Docker image builds are not part of the active verification or deployment contract.
For packaging-related work, verify the root `Dockerfile` and Docker policy instead.

When debugging `runtime-service`, pair `bash scripts/verify.sh` with `docs/system/runtime-storage-model.md`: the verifier enforces the no-Postgres runtime-domain guardrail, and the runtime doc explains the accepted cold-start window where observer-backed in-memory state is temporarily empty after restart.

Only runnable repo entrypoints under `cmd/` may be packaged this way.
Current runnable entries are `meta-service`, `config-service`, `network-service`, `release-service`, and `runtime-service`.


## What this verifier should not claim

`verify.sh` should not pretend that the migration is already complete while old paths are still in use.
It should verify the active local contract honestly.

## Related docs

- `docs/policies/verification.md`
- `docs/policies/docker-baseline.md`
- `docs/system/recovery.md`
