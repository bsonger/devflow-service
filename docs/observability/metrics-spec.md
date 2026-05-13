# Metrics Specification

## Purpose

This spec defines DevFlow application metric label names and how they map to the
structured log and trace field contract.

Metrics, logs, and traces should describe the same facts, but they do not use the
same physical field syntax:

- logs use approved OpenTelemetry semantic dotted keys such as `service.name`
- traces use OpenTelemetry semantic attributes such as `service.name`
- Prometheus metrics use Prometheus-safe label names such as `service_name`

Do not add high-cardinality identifiers such as `trace_id`, `request_id`, or
`release_id` to metric labels. Use logs, traces, and Prometheus exemplars for
those joins.

For business and workflow metrics, do not automatically repeat service identity
dimensions such as `service_name`, `service_namespace`, or
`deployment_environment_name` when the same facts are already available from
resource metadata or scrape target metadata. Reserve those dimensions for
infrastructure-facing metrics such as HTTP server metrics.

## Canonical field mapping

| Meaning | Logs | Metrics | Traces |
|---|---|---|---|
| Service name | `service.name` | `service_name` | `service.name` |
| Service namespace | `service.namespace` | `service_namespace` | `service.namespace` |
| Service version | `service.version` | not recommended | `service.version` |
| Deployment environment | `deployment.environment.name` | `deployment_environment_name` | `deployment.environment.name` |
| HTTP method | `http.request.method` | `http_request_method` | `http.request.method` |
| HTTP route | `http.route` | `http_route` | `http.route` |
| HTTP status code | `http.response.status_code` | `http_response_status_code` | `http.response.status_code` |
| HTTP status class | `http.response.status_class` | `http_response_status_class` | `http.response.status_class` |
| Dependency target | `dependency` | `dependency` | `dependency` |
| Dependency action | `action` | `action` | `action` |
| Operation result | `result` | `result` | `result` |
| Release type | `release_type` | `release_type` | `devflow.release.type` |
| Trace ID | `trace_id` | forbidden | trace context |
| Span ID | `span_id` | forbidden | trace context |
| Request ID | `request_id` | forbidden | optional span attribute only when explicitly needed |
| Release ID | `devflow.release.id` | forbidden | `devflow.release.id` |
| Manifest ID | `devflow.manifest.id` | forbidden | `devflow.manifest.id` |
| Application ID | `devflow.application.id` | forbidden | `devflow.application.id` |
| Environment ID | `devflow.environment.id` | forbidden | `devflow.environment.id` |

## HTTP server metrics

HTTP request metrics must use this low-cardinality label set:

- `service_name`
- `service_namespace`
- `deployment_environment_name`
- `http_request_method`
- `http_route`
- `http_response_status_code`
- `http_response_status_class`

Example:

```text
http_server_requests_total{
  service_name="meta-service",
  service_namespace="devflow",
  deployment_environment_name="pre-production",
  http_request_method="GET",
  http_route="/api/v1/applications/:id",
  http_response_status_code="200",
  http_response_status_class="2xx"
}
```

`http_route` must be the route template, not a raw path containing IDs.

HTTP 5xx measurements should force an exemplar offer so the emitted Prometheus
sample can carry the active `trace_id` and `span_id` without turning either one
into a metric label. Normal sampled trace contexts may also produce exemplars.

Exemplars are correlation metadata attached to an individual sample. They are
not part of the metric time-series identity and must not be used as labels in
queries or dashboards.

Application SDK tracing is configured to offer/export all traces and leave
retention decisions to the OpenTelemetry Collector. Collector-side tail sampling
or filtering should keep every 5xx trace and may downsample low-value successful
traffic.

The application may path-filter trace creation only for these approved
low-value HTTP paths:

- `/health`
- `/healthz`
- `/readyz`
- `/livez`
- `/metrics`
- `/favicon.ico`
- `/internal/status`
- `/debug/pprof*`
- `/swagger*`

This is route filtering, not trace downsampling. The application must not apply
ratio-based trace sampling to the remaining traffic.

Recommended Collector policy:

- keep all traces received from the services where any server span has
  `http.response.status_code >= 500`
- keep slow traces above the incident latency threshold
- sample ordinary successful 2xx API traffic based on storage budget

The committed pre-production gateway policy lives in
`deployments/pre-production/otel-trace-gateway.yaml`; the rationale lives in
`docs/observability/trace-retention-policy.md`.

Low-value paths filtered by the application will still keep incident logs and
metrics according to the low-value path policy, but they will not have trace
exemplars because no trace is created for those paths.

## Workload metrics exposure contract

Application workload configuration now exposes a structured metrics contract:

- `metrics.enabled`
- `metrics.port`
- `metrics.scrape_profile`

`metrics.scrape_profile` is constrained to:

- `default`
- `fast`
- `slow`

Rules:

- `metrics.enabled=false` means the workload does not expose a metrics listener
  and must not be selected by the shared `ServiceMonitor`
- `metrics.enabled=true` requires `metrics.port > 0`
- `metrics.enabled=true` with an empty `metrics.scrape_profile` defaults to
  `default`
- `/metrics` remains the fixed scrape path; it is not workload-configurable

Release rendering projects that structured contract into Kubernetes objects:

- container env `METRICS_PORT=<metrics.port>`
- container port `metrics`
- the primary Service port `metrics`
- Service labels:
  - `observability.devflow.io/scrape=true`
  - `observability.devflow.io/scrape-profile=<metrics.scrape_profile>`

The shared pre-production `ServiceMonitor` uses those Service labels to select
targets. Metrics scrape contract must not be sourced from ad-hoc annotations.

Release completion also treats the metrics contract as a runtime promise:

- if `metrics.enabled=false`, no metrics endpoint verification is required
- if `metrics.enabled=true`, the deployed workload must actually answer
  `GET /metrics` on the rendered primary Service metrics port before the release
  can finish successfully

This prevents workloads from declaring a metrics port in configuration while the
process never binds that listener.

## Release workflow metrics

Release workflow metrics must use this label set:

- `release_type`

Do not add `release_id`, `manifest_id`, `application_id`, or `environment_id` to
release metric labels. Those are high-cardinality identifiers and belong in logs
or traces. If a release metric needs sample-level debugging context in the
future, use exemplars instead of labels.

Step-level release metrics may additionally use:

- `stage`
- `result`

Argo Application create metrics may additionally use:

- `strategy`
- `result`

## Manifest workflow metrics

Manifest workflow metrics must use this label set:

- `pipeline_type`
- `result`

Do not add manifest, application, pipeline, trace, git revision, image, or
control-plane identity values to metric labels. Those fields belong in structured
logs or trace attributes.

## Runtime action metrics

Runtime action metrics must use this label set:

- `action`
- `result`

Allowed runtime action values are bounded to operator/runtime write paths such as
`sync_runtime_workload`, `sync_runtime_pod`, `delete_pod`, and
`restart_deployment`. Do not add pod names, deployment names, namespaces,
application IDs, or runtime spec IDs as metric labels.

## Dependency metrics

Dependency metrics must use this label set:

- `dependency`
- `action`
- `result`

Avoid duplicate labels such as `devflow.dependency` and `devflow.action` that
repeat `dependency` and `action`.
Do not emit both `dependency_operation` and `action` for the same metric.

## Low-value path filtering

HTTP metrics should follow the same low-value path policy as request logs:

- do not record fast successful requests for:
  - `/health`
  - `/healthz`
  - `/readyz`
  - `/livez`
  - `/metrics`
  - `/favicon.ico`
  - `/internal/status`
  - `/debug/pprof*`
  - `/swagger*`
- always record 4xx requests
- always record 5xx requests
- always record requests with duration >= 1000 ms

This keeps normal probe noise out of request dashboards while preserving incident
signals when probes fail or become slow.

## Legacy compatibility

Older deployed versions may still emit labels such as:

- `service`
- `environment`
- `method`
- `route`
- `status_code`

These are legacy metric labels. Dashboards may temporarily query both old and
new labels during rollout, but new application metrics must use the canonical
Prometheus-safe labels in this spec.
