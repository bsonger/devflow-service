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
| Dependency action | `dependency_operation` or `action` | `action` | `action` |
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

The application must not path-filter trace creation for low-value HTTP paths.
Probe and scrape noise should be removed by Collector-side trace policy, while
application log and metric filters may still suppress fast successful low-value
requests.

## Release workflow metrics

Release workflow metrics must use this label set:

- `service_name`
- `service_namespace`
- `deployment_environment_name`
- `release_type`

Do not add `release_id`, `manifest_id`, `application_id`, or `environment_id` to
release metric labels. Those are high-cardinality identifiers and belong in logs
or traces. If a release metric needs sample-level debugging context in the
future, use exemplars instead of labels.

## Dependency metrics

Dependency metrics must use this label set:

- `service_name`
- `service_namespace`
- `deployment_environment_name`
- `dependency`
- `action`
- `result`

Avoid duplicate labels such as `devflow.dependency` and `devflow.action` that
repeat `dependency` and `action`.

## Low-value path filtering

HTTP metrics should follow the same low-value path policy as request logs:

- do not record fast successful probe/scrape/static requests for `/healthz`,
  `/readyz`, `/livez`, `/metrics`, or `/favicon.ico`
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
