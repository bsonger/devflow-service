# Scripts

This directory contains repo-level verification and support scripts.

## Reader and outcome

This guide is for a fresh engineer or agent landing in `devflow-service`.
After reading it, the reader should know which repo-local script to run first, what each focused verifier is allowed to claim, and where the final anti-drift gate lives.

## Canonical verifier

Run this from the repo root before handoff or after changing docs, verification rules, Docker policy, repo layout, or build paths:

```sh
bash scripts/verify.sh
```

## Platform deploy helper

When you need to deploy through the DevFlow platform release flow instead of applying Kubernetes YAML directly, use:

```sh
PLATFORM_BASE_URL=https://devflow.bei.com \
APPLICATION_ID=<application-uuid> \
ENVIRONMENT_ID=<environment-uuid> \
SERVICE_NAME=<service-name> \
PLATFORM_AUTH_HEADER='Authorization: Bearer <token>' \
bash scripts/deploy-platform-release.sh
```

The helper:
- optionally validates `SERVICE_NAME` against the application's network-service records before release creation
- creates or reuses an `Available` manifest for `APPLICATION_ID + GIT_REVISION`
- waits for the manifest to become `Available`
- creates a release with `manifest_id + environment_id + strategy`
- optionally polls the release until it reaches a terminal status

Useful knobs:
- `GIT_REVISION` defaults to `main`
- `STRATEGY` defaults to `rolling`
- `SERVICE_NAME` lets the operator assert which application service they intend to deploy
- `WAIT_FOR_MANIFEST` and `WAIT_FOR_RELEASE` default to `true`
- `MANIFEST_TIMEOUT_SECONDS` defaults to `900`
- `RELEASE_TIMEOUT_SECONDS` defaults to `1800`

Current contract note:
- the active backend manifest/release APIs are still application-scoped, so `SERVICE_NAME` is currently used as a safety validation input rather than a backend single-service release selector

This script is the repo-local way to trigger a platform deployment from the service repository. It intentionally does not call `kubectl apply` or depend on repo-local Kubernetes manifests.

## Platform staging helper

When you want to deploy the 5 backend services plus the frontend to one staging environment in sequence through the platform, use:

```sh
PLATFORM_BASE_URL=https://devflow.bei.com \
PROJECT_ID=<project-uuid> \
ENVIRONMENT_ID=<staging-environment-uuid> \
PLATFORM_AUTH_HEADER='Authorization: Bearer <token>' \
bash scripts/deploy-platform-staging-all.sh
```

Canonical staging defaults for this repo are documented in `docs/guides/staging-deploy.md`.

Default service set:
- `meta-service`
- `config-service`
- `network-service`
- `release-service`
- `runtime-service`
- `platform-web`

This helper first resolves application IDs from `PROJECT_ID`, then delegates each deployment to `scripts/deploy-platform-release.sh`.

## Exemplar smoke test

When Prometheus has recent pre-production 5xx samples, run:

```sh
PROMETHEUS_URL=https://prometheus.bei.com bash scripts/verify-exemplars.sh
```

This live check queries `/api/v1/query_exemplars` and verifies that matching
HTTP 5xx metric samples carry `trace_id` and `span_id` exemplar labels. It is
not part of `scripts/verify.sh` because it depends on live Prometheus data and
recent incident traffic.

## OpenAPI sync

For API contract sync work:

```sh
make openapi-check
```

That target validates:
- `api/openapi/meta-service.yaml`
- `api/openapi/network-service.yaml`
- `api/openapi/config-service.yaml`
- `api/openapi/release-service.yaml`
- `api/openapi/runtime-service.yaml`
- `api/openapi/devflow.yaml`

Before changing those files, read:
- `docs/api/contract-guide.md`
- `docs/guides/openapi-workflow.md`
- `api/openapi/README.md`

For Codex-assisted contract refresh:

```sh
bash scripts/sync-openapi-with-codex.sh
```

That helper runs the fixed Codex prompt, then `make openapi-check`, then prints the resulting `git diff`.

This remains the canonical repo-local handoff check while the repository migrates from the older nested shape to the root `cmd/` and `internal/` layout.

For broader local development command order, also read:
- `docs/guides/local-development.md`
- `docs/guides/backend-change-playbook.md`

## Proof split

Use the verification surfaces in this order:

1. focused Go seam tests for behavioral proof
2. `bash scripts/verify-metadata-audit.sh` for metadata/doc routing consistency only
3. `bash scripts/verify.sh` for the final repo-wide anti-drift gate

That split is intentional:

- typed tests own behavior such as callback ownership, rollout progression, and finalized-release terminality
- `verify-metadata-audit.sh` only checks that code seams, evidence docs, and verifier guidance still describe the same metadata contract and proof routing
- `verify.sh` remains the broad repo-local contract check before handoff

## What `verify.sh` should prove

The verifier should fail fast and prove:
- repo-local startup and docs surfaces exist under the layered docs structure
- the active Go baseline matches the current contract
- Docker policy is enforced from the policy docs and script checks
- the release → Argo → runtime proof route remains rerunnable from canonical surfaces: release-side metadata production and bundle rendering live in `internal/release/service`, runtime-side rollout observation and callback step emission live in `internal/runtime/observer`, release-side callback/writeback acceptance and status normalization live in `internal/release/transport/http` plus `internal/release/service`, and repo-wide anti-drift still terminates at `bash scripts/verify.sh`
- release-flow contract wording stays aligned across `docs/system/flow-overview.md`, `docs/system/release-steps.md`, `docs/system/release-writeback.md`, and the reader-facing release/runtime docs that summarize `start_deployment`, `observe_rollout`, and `finalize_release`
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
- `verify.sh` validates repo-local code, docs, and scripts only; it does not claim to validate Kubernetes manifests that no longer live in this repository

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

When debugging release-flow contract drift, pair `bash scripts/verify.sh` with `docs/system/flow-overview.md`, `docs/system/release-steps.md`, and `docs/system/release-writeback.md`: those docs define the authoritative ownership split between the release-service handoff step (`start_deployment`) and callback-owned progression/finalization steps such as `observe_rollout` and `finalize_release`.

The focused release → Argo → runtime behavioral proof route that should be rerun before broad repo debugging is:

```sh
go test ./internal/runtime/transport/http ./internal/runtime/observer ./internal/release/transport/http ./internal/release/service -run 'TestDeleteRuntimePodReturnsAcknowledgement|TestRolloutRuntimeReturnsAcknowledgement|TestWriteReleaseStepsRollingObserverSkipsReleaseOwnedHandoffStep|TestHandleArgoEventUpdatesReleaseStatus|TestReleaseStatusConvergenceRequiresReleaseOwnedStartDeploymentBeforeClosingRelease'
```

Read the behavioral proof in layers when that command fails:
- `internal/runtime/transport/http` proves operator-facing runtime read/action HTTP mapping plus acknowledgement payload shape, including `convergence_state=pending_observation`
- `internal/runtime/observer` proves runtime-side release label consumption and callback-owned step emission
- `internal/release/transport/http` proves release-side callback/writeback normalization at the HTTP boundary
- `internal/release/service` proves final release status stays `Running` until the release-owned `start_deployment` handoff step succeeds and the full canonical graph converges

The focused metadata/doc routing proof route that should be rerun after metadata-contract or verifier-guidance edits is:

```sh
bash scripts/verify-metadata-audit.sh
```

Read that consistency proof in layers when it fails:
- release bundle checks prove the canonical label overlay and supplementary-annotation filter still exist at the release-owned seam
- Argo checks prove restartedAt ignore-difference targeting remains narrow and workload-kind-aware (`Deployment` for rolling, `Rollout` for blue-green/canary)
- runtime observer checks prove release/application/environment correlation still comes from workload labels
- doc checks prove `docs/resources/metadata-contract-audit.md` and `docs/resources/metadata-drift-proof.md` still present themselves as evidence artifacts and still route readers to the canonical system docs plus the correct verifier order

After the focused seams pass, `bash scripts/verify.sh` remains the final repo-wide anti-drift rerun.

Only runnable repo entrypoints under `cmd/` may be packaged this way.
Current runnable entries are `meta-service`, `config-service`, `network-service`, `release-service`, and `runtime-service`.

## What this verifier should not claim

`verify.sh` should not pretend that the migration is already complete while old paths are still in use.
It should verify the active local contract honestly.

`verify-metadata-audit.sh` should not claim to prove runtime behavior, release terminality, or callback execution semantics by itself.
It is a routing and consistency check that complements, but does not replace, the focused Go seam tests.

## Related docs

- `docs/policies/verification.md`
- `docs/policies/docker-baseline.md`
- `docs/system/recovery.md`
