# Logging Specification

## Purpose

This spec defines the source boundary for DevFlow log fields.
It prevents application code, OpenTelemetry SDK setup, and the Kubernetes-local
OpenTelemetry Collector from all trying to write the same facts.

The rule is: each layer writes the fields it owns, and downstream enrichment
must not be hard-coded back into business code.

## Field ownership

### Application code and Gin middleware

Application code owns facts that are known at the HTTP or business-operation
edge. These fields are emitted by the logger or Gin middleware when available:

| Field | Owner | Notes |
|---|---|---|
| `timestamp` | zap encoder | Event time emitted by the application logger. |
| `severity_text` | zap encoder | Log severity text. Older readers may call this `level`. |
| `body` | zap encoder | Log message body. Older readers may call this `msg`. |
| `request_id` | Gin middleware | Generated from `X-Request-Id` / `X-Request-ID`, or created when absent. |
| `logger.name` | zap logger | Logger name for the running service logger. |
| `component` | application code | Stable component such as `http_server` or `dependency_client`. |
| `event.outcome` | Gin middleware | `success` or `failure` for request completion. |
| `result` | application code | Domain-specific outcome such as `2xx`, `4xx`, `started`, `success`, `error`. |
| `http.request.method` | Gin middleware | Request method. |
| `http.route` | Gin middleware | Gin route template, not raw unbounded path IDs. |
| `url.path` | Gin middleware | Raw request path. |
| `http.response.status_code` | Gin middleware | Numeric response status. |
| `http.response.status_class` | Gin middleware | Low-cardinality class such as `2xx` or `5xx`. |
| `http.request.body.size` | Gin middleware | Request body size in bytes when known. |
| `http.response.body.size` | Gin middleware | Response body size in bytes when known. |
| `duration_ms` | Gin middleware | Request duration in milliseconds for quick human scanning. |
| `http.server.request.duration` | Gin middleware | Request duration in seconds for semantic alignment with metrics/traces. |
| `client.address` | Gin middleware | Client address as observed by Gin. |
| `user_agent.original` | Gin middleware | Raw user-agent header. |
| `devflow.project.id` | Gin middleware / business code | From stable headers, query params, or route params when available. |
| `devflow.application.id` | Gin middleware / business code | From stable headers, query params, or route params when available. |
| `devflow.service.id` | Gin middleware / business code | From stable headers, query params, or route params when available. |
| `devflow.environment.id` | Gin middleware / business code | From stable headers, query params, or route params when available. |
| `devflow.release.id` | Gin middleware / business code | From stable headers, query params, or route params when available. |
| `devflow.manifest.id` | Gin middleware / business code | Optional; emit only when the request or operation actually carries it. |

Business code should still log critical operation decisions, failures, and
state transitions. It should not add Kubernetes, host, or cloud fields.

### OpenTelemetry SDK and instrumentation

The SDK and instrumentation own trace correlation and service resource facts.
They read from the active context and OTEL environment/configuration values,
including `OTEL_SERVICE_NAME` and `OTEL_RESOURCE_ATTRIBUTES`.

| Field | Source |
|---|---|
| `trace_id` | Current span context from OpenTelemetry instrumentation. |
| `span_id` | Current span context from OpenTelemetry instrumentation. |
| `trace_flags` | Current span context from OpenTelemetry instrumentation. |
| `service.name` | `OTEL_SERVICE_NAME`, then `service.name` in `OTEL_RESOURCE_ATTRIBUTES`, then service bootstrap fallback. |
| `service.namespace` | `service.namespace` in `OTEL_RESOURCE_ATTRIBUTES`, then `OTEL_SERVICE_NAMESPACE`, then `devflow`. |
| `service.version` | `service.version` in `OTEL_RESOURCE_ATTRIBUTES`, then `SERVICE_VERSION` / `VERSION`. Pre-production service manifests set this through environment variables instead of service config. |
| `deployment.environment.name` | `deployment.environment.name` in `OTEL_RESOURCE_ATTRIBUTES`; legacy `deployment.environment` is accepted only as fallback. |

`trace_id` and `span_id` are the key join columns for Trace -> Log correlation:
when an operator opens a slow or failed trace, the same identifiers let them find
application logs that happened inside that trace/span without guessing by time,
pod, or route alone. `trace_flags` helps identify whether the trace was sampled.

### OpenTelemetry Collector enrichment

The Collector owns Kubernetes, host, and cloud enrichment because those facts are
runtime-placement facts, not business facts. In Kubernetes, the same application
image can move between namespaces, pods, nodes, or clusters without any code
change. Hard-coding those fields in business code creates stale logs after
reschedules, rollouts, or cluster migrations.

Collector-owned fields include:

| Field | Collector source |
|---|---|
| `k8s.cluster.name` | Collector resource/enrichment config. |
| `k8s.namespace.name` | `k8sattributes` processor. |
| `k8s.deployment.name` | `k8sattributes` processor. |
| `k8s.replicaset.name` | `k8sattributes` processor. |
| `k8s.pod.name` | `k8sattributes` processor. |
| `k8s.pod.uid` | `k8sattributes` processor. |
| `k8s.container.name` | `k8sattributes` processor. |
| `k8s.node.name` | `k8sattributes` processor. |
| `host.name` | Collector resource detection. |
| `host.ip` | Collector resource detection or deployment-side resource config. |
| `cloud.provider` | Collector resource detection or deployment-side resource config. |
| `cloud.region` | Collector resource detection or deployment-side resource config. |

The committed pre-production log collector manifest is
`deployments/pre-production/otel-log-collector-daemonset.yaml`.
It tails only `devflow-pre-production` pod logs from `/var/log/pods`, parses the
container envelope plus JSON application body, enriches records with Kubernetes
metadata, and exports them through OTLP HTTP.

## Duplicate-field policy

Do not emit duplicate aliases for the same fact in new logs.

Recommended fields:

- `service.name`, not `service`
- `deployment.environment.name`, not `environment`
- `service.version`, not `service_version`
- `http.request.method`, not `method`
- `http.route`, not `route`
- `url.path`, not `path`
- `http.response.status_code`, not `status_code`
- `client.address`, not `client_ip`
- `user_agent.original`, not `user_agent`

Legacy fields such as `service`, `environment`, `service_version`, `method`,
`route`, `path`, `status_code`, `client_ip`, and `user_agent` may appear in
older logs or metrics, but they are compatibility fields and are not recommended
for new structured log events.

Metrics use Prometheus-safe names instead of dotted log keys. See
`docs/observability/metrics-spec.md` for the canonical log/metric/trace mapping.

## HTTP request logging filter

The request logger filters low-value infrastructure paths by default:

- `/healthz`
- `/readyz`
- `/livez`
- `/metrics`
- `/favicon.ico`

Filtering those paths keeps dashboards and log search focused on user-visible or
operator-visible behavior. Health, readiness, and metrics endpoints can be hit
many times per minute by kubelet, Prometheus, and probes. Logging every success
creates noise, storage cost, and false traffic volume.

The filter must not hide signals that are useful during incidents. The request
logger must still emit:

- `http.response.status_code >= 500`
- `duration_ms >= 1000`
- 4xx requests
- `ERROR` / `WARN` logs
- critical business operation logs

Ordinary successful 2xx business requests may be sampled in the future, but
sampling must never suppress the incident signals listed above.
