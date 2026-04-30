# Verification Policy

This document defines the target verification contract for `devflow-service`.

## Canonical proof stack

Run these commands from the repo root:

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

Local ad-hoc Docker image builds are intentionally outside the canonical proof stack.
For packaging-related changes, verify the root `Dockerfile` and `scripts/check-docker-policy.sh`; CI manifests are not part of the canonical verification contract.

The repo-level convenience target for the same contract is:

```sh
make ci
```


## Expectations

- formatting, vet, lint, tests, build, and repo verification must agree
- repo docs and verification must describe the same paths and command order
- failures are real contract drift to fix, not accepted migration noise
- release-flow contract drift between code, docs, and verifier surfaces is a real verifier failure; when `start_deployment`, `observe_rollout`, `finalize_release`, or release writeback ownership wording drifts, treat that mismatch as contract drift and update the authoritative docs plus verifier surfaces together
- use `docs/system/flow-overview.md`, `docs/system/release-steps.md`, and `docs/system/release-writeback.md` as the lifecycle/writeback authority when verifying release-flow wording in `docs/resources/*`, `docs/services/*`, or recovery/script guidance
- observability, logging, and trace-correlation changes must follow `docs/policies/observability-logging.md`
- API error envelope and handler mapping changes must follow `docs/policies/error-handling.md`
- HTTP transport behavior changes must follow `docs/policies/http-handler.md`
- `GET` may use query filters, but `POST` and `DELETE` business selectors must not be carried on query strings; follow `docs/policies/resource-api.md`
- HTTP handlers should use shared UUID parse helpers from `internal/platform/httpx` instead of hand-rolled `uuid.Parse(...)` plus repeated error writes
- HTTP handlers should use shared `BindJSON`, pagination helpers, and stable internal-error helpers from `internal/platform/httpx` instead of repeating local parsing and 5xx response patterns
- HTTP handlers should prefer specialized `httpx` helpers such as `WriteInvalidArgument`, `WriteFailedPrecondition`, and `WriteUnauthorized` over generic `WriteError(...)` calls when the shared helper already covers the response
- service-layer boundary changes must follow `docs/policies/service-layer.md`
- downstream HTTP client and runtime-boundary adapter changes must follow `docs/policies/downstream-client.md`
- HTTP-based runtime lookup code outside dedicated downstream adapters should reuse `internal/shared/downstreamhttp` instead of hand-rolled `net/http` clients in service, support, or runtime packages
- repository boundary changes must follow `docs/policies/repository-layer.md`
- service and repository generic validation errors should prefer `internal/shared/errs` over repeated ad-hoc `errors.New(...)` strings
- worker, runtime, or background execution changes must follow `docs/policies/worker-runtime.md`
- resource-facing HTTP behavior and `docs/resources/*.md` changes must follow `docs/policies/resource-api.md`
- new structured log fields must use `snake_case`
- new metrics labels must stay low-cardinality and must not include identifiers such as `trace_id`, `request_id`, `release_id`, or user-specific values
- production code under `internal/*/service` must not call `db.Postgres()` or `store.DB()` directly; repository-owned persistence must stay in `internal/*/repository`
- production code under `internal/*/service` must not depend on Gin, `internal/platform/httpx`, or `internal/*/transport/http`
- repo-local production code and active docs must not retain Mongo-era dependencies or terminology such as `mongo-driver`, `mongodb`, `bson`, or `ObjectID` after the PostgreSQL migration
- alias-only forwarding files such as `support_alias.go` must not be reintroduced; packages should import the owning implementation package directly once the transition layer is no longer needed
- `internal/shared` must not grow catch-all directories such as `common`, `util`, `utils`, `base`, or `model`
- `internal/platform` and `internal/shared` must not import business-domain packages directly; domain-facing translation belongs outside the platform layer
- code changes that affect the public API, domain models, or service behavior must include matching doc updates; follow `docs/policies/doc-synchronization.md`

## Verification ownership

- `scripts/verify.sh` is the canonical repo-local verification entrypoint
- `scripts/check-docker-policy.sh` enforces Docker policy
- `scripts/README.md` explains script behavior and side effects
