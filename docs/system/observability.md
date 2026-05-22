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
- `docs/observability/trace-retention-policy.md` for trace retention and Collector-side sampling boundaries
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

The active Go services can also expose Prometheus metrics on `/metrics` through
the shared `METRICS_PORT` environment variable. The runtime bootstrap still
accepts older service-specific `*_METRICS_PORT` values as a compatibility
fallback, but new manifests and release rendering should use `METRICS_PORT`.

Pre-production Prometheus Operator discovery is represented by one committed
manifest:

- `deployments/pre-production/service-monitor.yaml`

That manifest now contains three `ServiceMonitor` objects for scrape profiles
`default`, `fast`, and `slow`. They select Services labeled with:

- `observability.devflow.io/scrape=true`
- `observability.devflow.io/scrape-profile=<profile>`

and scrape the named `metrics` port at `/metrics`. The committed namespace
selector currently covers both `devflow-pre-production` and `devflow`.

Pre-production log collection is also represented by one committed manifest:

- `deployments/pre-production/otel-log-collector-daemonset.yaml`

That DaemonSet runs a node-local OpenTelemetry Collector in namespace
`observability`, tails `devflow-pre-production` and `devflow` pod stdout logs
from `/var/log/pods`, parses the container envelope plus structured JSON
application log body, enriches the records with Kubernetes metadata, and
exports them over OTLP HTTP to the existing Signoz collector endpoint.

Pre-production trace retention is represented by one committed gateway manifest:

- `deployments/pre-production/otel-trace-gateway.yaml`

The five DevFlow services export all non-filtered application traces to that
gateway. The gateway applies Collector-side tail sampling, keeps error and slow
traces, samples ordinary successful traces, and forwards retained traces to the
existing Signoz collector. Application code must not do ratio-based trace
sampling.

Pre-production Grafana dashboard-as-code assets live under:

- `deployments/pre-production/grafana/`
- `deployments/devflow/grafana/`

Use `scripts/verify-exemplars.sh` for a live Prometheus exemplar smoke test when
there has been recent 5xx traffic.

`/internal/status` should stay lightweight and safe.
It is the place for:

- `service`
- `environment`
- `version`
- `trace_id`
- startup time and uptime
- enabled HTTP module list
- observability wiring summary such as OTLP configuration presence
- last recorded runtime failure summary when one exists

It must not expose secrets, tokens, kubeconfigs, or large internal payloads.

## Runtime release signals

The active runtime release lane now exposes focused business diagnostics through
structured `runtime.state` logs plus low-cardinality Prometheus metrics.

Current runtime release lifecycle logs include:

- `queue_add`
- `queue_handle_start`
- `queue_handle_requeue_after`
- `queue_handle_rate_limited`
- `queue_handle_done`
- `release_reconcile_start`
- `release_reconcile_workload_missing`
- `release_reconcile_state_computed`
- `release_reconcile_terminal_label_update_failed`
- `release_reconcile_cleanup_completed`
- `release_reconcile_writeback_completed`
- `release_reconcile_requeue_scheduled`
- `runtime_workload_state_changed`

Those events are intended to answer, without joining multiple systems by hand:

- whether a `Running` release was actually enqueued
- whether reconcile found the expected release-owned workload
- whether the computed rollout phase was still `running` or already terminal
- whether terminal label convergence failed before or after writeback
- whether observed workload state actually changed or the observer is only replaying the same fact

Current runtime release metrics stay intentionally low-cardinality:

- `runtime_release_reconcile_total` with labels `phase`, `result`
- `runtime_release_writeback_total` with labels `step_code`, `result`, `http_response_status_code`
- `runtime_terminal_label_update_total` with labels `workload_kind`, `result`, `error_code`
- `runtime_observed_workload_state_total` with labels `summary_status`, `observe_state`

These metrics must not include `release_id`, `request_id`, or other per-object identifiers.

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

## Runtime Observer Execution Model

Current runtime processing is split into three active lanes plus one legacy release fallback lane:

- workload discovery lane: the Kubernetes runtime observer keeps runtime workload and pod state current and now prefers informer-backed cache reads for workload inspection
- release reconcile lane: queue-driven release processing behind `observer.release_runtime_enabled`, with candidate enqueue driven by workload watch events when cluster config is available
- manifest reconcile lane: queue-driven Tekton manifest processing behind `observer.manifest_runtime_enabled`, with informer-backed Tekton snapshots and Tekton watch-driven enqueue when cluster config is available
- legacy release polling lane: the old release rollout observer remains compatibility fallback only when `observer.release_runtime_enabled=false`

Manifest runtime no longer has a polling compatibility lane.
`observer.manifest_runtime_enabled` is now the only manifest startup switch:

- when `observer.manifest_runtime_enabled=true`, runtime-service starts the manifest runtime reconciler
- when `observer.manifest_runtime_enabled=false`, runtime-service does not start the manifest runtime lane

Release reconcile rules:

- derive candidate release IDs from release-owned workload watch events when runtime workload cache is available, with polling retained only as compatibility fallback
- only process releases whose persisted status is `Running`
- only process workloads owned by the current `control_plane_id`
- write release steps through the shared runtime release writer and preserve the existing `/api/v1/verify/release/steps` contract
- when the runtime observer namespace is unset, discovery is cluster-wide rather than implicitly falling back to the observer pod namespace; namespace narrowing must be configured explicitly

Manifest reconcile rules:

- derive candidate manifest IDs from informer-backed Tekton snapshots filtered by control plane
- enqueue reconcile keys from `PipelineRun` and `TaskRun` watch events
- when `observer.tekton_pipeline` is configured, only reconcile PipelineRuns whose `spec.pipelineRef.name` matches that pipeline
- reconcile from current snapshot state instead of trusting individual event payloads
- write manifest status, task, and result callbacks through `/api/v1/release/manifests/tekton/*`
- preserve legacy Tekton result payload semantics for `commit_hash`, `image_ref`, `image_tag`, and `image_digest`
- runtime-service no longer retries those callbacks against `/api/v1/manifests/tekton/*`

Operational posture:

- queue workers reconcile from current cache or store snapshots, not from event payload truth
- manifest runtime is informer-backed when cluster config is available; there is no separate legacy manifest poller startup path anymore
- release runtime prefers workload watch events when cluster config is available, and falls back to the older running-release polling source only when workload watch bootstrap cannot be constructed
- enabling the release queue-driven lane disables its matching legacy release poller so duplicate writebacks are not produced by parallel execution
- disabling `observer.manifest_runtime_enabled` leaves manifest runtime disabled instead of returning to a legacy polling mode
- disabling `observer.release_runtime_enabled` returns release runtime responsibility to the legacy release polling behavior

Cutover prerequisite:

- queue-driven release runtime enabled and verified in at least one environment
- queue-driven manifest runtime enabled and verified in at least one environment
- the removed legacy manifest poller produces no unique writeback behavior missing from reconcile workers
- runtime and observability docs stay consistent with the enabled execution model

## Future direction

As the migration proceeds, this observability surface should become simpler, not more layered.
The end state should be one honest startup contract, one honest verification contract, and one honest root build path.

Application-level observability naming and correlation rules now live in the observability logging policy.
The log-field ownership contract lives in `docs/observability/logging-spec.md`;
use it to decide whether a field belongs in application code, OpenTelemetry SDK
resource configuration, or Collector-side Kubernetes/host/cloud enrichment.
