# Observability

## Purpose

This repository exposes interruption-safe inspection and verification surfaces for a fresh engineer or agent.
Observability here means a reader can enter `devflow-service`, find the local recovery authority, rerun the verification stack, and localize whether drift is in docs, Docker policy, repo verification, or the ongoing `meta-service` migration.

## Primary inspection surfaces

Use these files as the primary repo-local observability surfaces:
- `AGENTS.md` for startup and routing
- `docs/system/recovery.md` for current recovery and failure routing
- `docs/system/architecture.md` for current repo-local structure
- `docs/policies/observability-logging.md` for structured logging, metric-label, and trace-correlation rules
- `docs/system/release-writeback.md` for token-gated release observer callback behavior
- `docs/services/meta-service.md` for current service-specific behavior and diagnostics
- `docs/resources/` for current resource contracts and API behavior
- `docs/policies/docker-baseline.md` for packaging and base-image rules
- `docs/policies/verification.md` for the target proof stack
- `scripts/README.md` for repo script behavior
- `go.mod` for the active Go baseline

## Runtime inspection endpoints

The active HTTP services expose small runtime inspection endpoints:

- `/healthz` for liveness
- `/readyz` for readiness
- `/internal/status` for a compact operator-facing status summary

`/internal/status` should stay lightweight and safe.
It is the place for:

- `service`
- `environment`
- `version`
- `request_id`
- `trace_id`
- startup time and uptime
- enabled HTTP module list
- observability wiring summary such as OTLP configuration presence
- last recorded runtime failure summary when one exists

It must not expose secrets, tokens, kubeconfigs, or large internal payloads.

## Verification signal

The target verification signal for this repo is:

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

A passing result means:
- docs and verification agree on the active repo contract
- Docker policy holds
- the current code compiles and tests
- the runnable service entrypoints build from the root layout

Packaging selection for `config-service`, `network-service`, `release-service`, and `runtime-service` is intentionally expressed through committed Tekton manifests rather than local Docker verification steps.

## Failure interpretation

- docs/path failure -> inspect `AGENTS.md`, `docs/system/*`, `docs/services/*`, `docs/resources/*`, `docs/policies/*`
- Docker failure -> inspect `docs/policies/docker-baseline.md`, `scripts/check-docker-policy.sh`, and the failing `Dockerfile`
- compile or test failure -> inspect the failing package under `cmd/` or `internal/`
- release writeback failure -> inspect `docs/system/release-writeback.md`, `internal/release/transport/http/*`, and release config wiring
- migration-boundary ambiguity -> inspect local system docs first, then `devflow-control` target docs

## Future direction

As the migration proceeds, this observability surface should become simpler, not more layered.
The end state should be one honest startup contract, one honest verification contract, and one honest root build path.

Application-level observability naming and correlation rules now live in the observability logging policy.
