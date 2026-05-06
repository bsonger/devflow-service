# Trace Retention Policy

## Purpose

DevFlow services export all non-filtered application traces. Trace retention and
downsampling are Collector responsibilities, not application-code decisions.

The pre-production trace gateway is defined in:

- `deployments/pre-production/otel-trace-gateway.yaml`

Services send OTLP traces to that gateway. The gateway applies tail sampling and
then forwards retained traces to the existing Signoz collector.

## Application Boundary

Application code must:

- create traces for normal application routes
- keep the SDK sampler as `always_on`
- filter only low-value routes before trace creation
- attach exemplars for HTTP 5xx metric samples when an active trace exists

Application code must not:

- ratio-sample traces
- make storage-budget retention decisions
- add `trace_id` or `span_id` as metric labels

Low-value route filtering currently covers health, readiness, liveness, metrics,
favicon, internal status, pprof, and swagger paths. This is route filtering, not
downsampling.

## Collector Boundary

The Collector owns retention decisions because it can evaluate a whole trace
after all spans arrive.

Pre-production retention policy:

- keep traces with error status
- keep traces with latency at or above 1000 ms
- sample ordinary successful traces according to the configured storage budget

The committed pre-production gateway currently samples ordinary successful
traces at `20%`. Adjust that number in `tail_sampling.policies.sample-success`
when storage pressure or debugging needs change.

## Why 5xx Traces Must Be Kept

HTTP 5xx traces are incident evidence. They connect:

- the user-facing failed request
- structured logs through `trace_id` and `span_id`
- Prometheus exemplar samples
- downstream dependency spans

Dropping these traces breaks the fastest root-cause path from dashboard symptom
to request-level execution.

## Low-Value Trace Paths

Low-value probe and scrape paths are filtered inside the service before trace
creation. That keeps backend trace storage from filling with periodic health and
metrics traffic.

Failed or slow low-value requests still produce logs and metrics according to
the log and metric policy. They do not have trace exemplars when the route was
filtered before trace creation.

## Rollout Version Correlation

`service.version` must identify the deployed binary or image revision closely
enough to correlate a trace with a rollout.

The pre-production service manifests set common OpenTelemetry resource labels
through container environment variables:

- `OTEL_SERVICE_NAME`
- `OTEL_SERVICE_NAMESPACE`
- `DEPLOYMENT_ENVIRONMENT`
- `SERVICE_VERSION`

`DEPLOYMENT_ENVIRONMENT` must carry the human-readable environment name such as
`pre-production`, not the DevFlow environment UUID. If a workflow also needs
the environment ID, keep it in business labels/fields such as
`devflow.environment.id` instead of overloading `deployment.environment.name`.

`OTEL_RESOURCE_ATTRIBUTES` is optional legacy compatibility only. New manifests
should not duplicate `service.namespace`, `service.version`, and
`deployment.environment.name` there when the dedicated environment variables
already define those values.

Rollout automation should patch `SERVICE_VERSION` to a git SHA, image digest,
or CI build identifier in the same change that updates the image.

Do not encode static calendar dates such as `preproduction-meta-20260425` as
`service.version`; they cannot distinguish two rollouts on the same service.
