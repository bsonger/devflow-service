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
| `caller` | zap encoder | Debugging aid only. Keep it in logs, but do not treat it as a primary observability dimension. |
| `logger.name` | zap logger | Logger name for the running service logger. |
| `http.request.method` | Gin middleware | Request method. |
| `http.route` | Gin middleware | Gin route template, not raw unbounded path IDs. |
| `url.path` | Gin middleware | Raw request path. |
| `http.response.status_code` | Gin middleware | Numeric response status. |
| `http.request.body.size` | Gin middleware | Request body size in bytes when known. Omit empty `GET` / `HEAD` zero-body noise. |
| `http.response.body.size` | Gin middleware | Response body size in bytes when known. |
| `duration_ms` | Gin middleware | Request duration in floating-point milliseconds for quick human scanning. |
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
It should also avoid repeating service resource facts that are already attached
by the base logger or OpenTelemetry resource configuration.

### OpenTelemetry SDK and instrumentation

The SDK and instrumentation own trace correlation and service resource facts.
They read from the active context and OTEL environment/configuration values.
Current DevFlow service templates derive service identity primarily from the
service bootstrap/runtime environment, while `OTEL_SERVICE_NAME` and
`OTEL_RESOURCE_ATTRIBUTES` remain compatibility fallbacks rather than the
recommended primary path.

| Field | Source |
|---|---|
| `trace_id` | Current span context from OpenTelemetry instrumentation. |
| `span_id` | Current span context from OpenTelemetry instrumentation. |
| `service.name` | service bootstrap / `SERVICE_NAME`, then `OTEL_SERVICE_NAME`, then `service.name` in `OTEL_RESOURCE_ATTRIBUTES`. |
| `service.namespace` | `OTEL_SERVICE_NAMESPACE`, then `service.namespace` in `OTEL_RESOURCE_ATTRIBUTES`, then `devflow`. |
| `service.version` | `SERVICE_VERSION` / `VERSION`, then `service.version` in `OTEL_RESOURCE_ATTRIBUTES`. When the raw value is a full digest, application logging and tracing normalize it to a shorter service version. |
| `deployment.environment.name` | preferred from downstream resource enrichment or Collector-side resource metadata. If a service explicitly provides it, `DEPLOYMENT_ENVIRONMENT` or `deployment.environment.name` in `OTEL_RESOURCE_ATTRIBUTES` may still be consumed as compatibility input. Request logs do not need to repeat it in every record. |

`trace_id` and `span_id` are required on request, workflow, dependency, and
worker logs whenever the current context has an active span. They are the key
join columns for Trace -> Log correlation:
when an operator opens a slow or failed trace, the same identifiers let them find
application logs that happened inside that trace/span without guessing by time,
pod, or route alone.

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
| `k8s.pod.name` | `k8sattributes` processor. |
| `k8s.pod.uid` | `k8sattributes` processor. |
| `k8s.container.name` | `k8sattributes` processor. |
| `k8s.node.name` | `k8sattributes` processor. |
| `host.name` | Collector resource detection. |
| `host.ip` | Collector resource detection or deployment-side resource config. |
| `cloud.provider` | Collector resource detection or deployment-side resource config. |
| `cloud.region` | Collector resource detection or deployment-side resource config. |

The committed shared log collector manifest is
`deployments/pre-production/otel-log-collector-daemonset.yaml`.
It tails `devflow-pre-production` and `devflow` pod logs from `/var/log/pods`,
parses the container envelope plus JSON application body, enriches records with
Kubernetes metadata, and exports them through OTLP HTTP.

## Duplicate-field policy

Do not emit duplicate aliases for the same fact in new logs.

Recommended fields:

- `http.request.method`, not `method`
- `http.route`, not `route`
- `url.path`, not `path`
- `http.response.status_code`, not `status_code`
- `client.address`, not `client_ip`
- `user_agent.original`, not `user_agent`
- `service.name`, not `service`, only when service identity is explicitly needed
- `deployment.environment.name`, not `environment`, only when environment identity is explicitly needed
- `service.version`, not `service_version`, only when version identity is explicitly needed

Legacy fields such as `service`, `environment`, `service_version`,
`method`, `route`, `path`, `status_code`, `client_ip`, and
`user_agent` may appear in older logs or metrics, but they are compatibility
fields and are not recommended for new structured log events.

Metrics use Prometheus-safe names instead of dotted log keys. See
`docs/observability/metrics-spec.md` for the canonical log/metric/trace mapping.

## Logger naming and caller policy

- `caller` stays in every log as a debugging aid.
- `caller` is not a core observability field.
- `caller` must not be used as a metrics label.
- `caller` must not be used as a Loki stream label.
- `caller` must not be exposed as a Grafana dashboard variable.
- log classification should use `logger.name`, not `caller`

Current HTTP logger names:

- normal request completion: `http.access`
- HTTP 4xx / 5xx request completion: `http.error`
- panic recovery log: `http.error`

Current HTTP message bodies:

- normal request completion: `http request`
- client-canceled request completion: `http request canceled`
- slow 2xx request completion: `slow http request`
- HTTP 4xx request completion: `http client error`
- HTTP 5xx request completion: `http server error`
- panic recovery: `panic recovered`

Business and dependency logs may still use service- or component-specific
`logger.name` values, but request-class dashboards and filters should classify
HTTP logs by the names above.

Shared baseline fields for structured logs:

- `timestamp`
- `severity_text`
- `body`
- `logger.name`
- `caller`
- `trace_id` when an active span exists
- `span_id` when an active span exists

`logger.name` is the category field. `caller` is a debugging field only and must
not be used as a metric label, Loki stream label, dashboard variable, or primary
classification key.

Do not treat `service.name`, `service.namespace`, `service.version`, or
`deployment.environment.name` as shared baseline application fields. Those are
service resource facts owned by the OpenTelemetry SDK, runtime resource
configuration, or downstream enrichment. Application loggers must not
unconditionally attach them to every logger category.

## Per-logger field contract

The active contract is: shared baseline first, then a small category-specific
field set chosen by `logger.name`.

### `http.access`

Use for:

- ordinary successful request completion
- slow 2xx request completion
- client-canceled request completion

Required fields in addition to shared baseline:

- `http.request.method`
- `http.route`
- `url.path`
- `http.response.status_code`
- `http.response.body.size`
- `duration_ms`
- `client.address`
- `user_agent.original`

Conditional fields:

- `http.request.body.size` when a body is present or the method is not `GET` / `HEAD`
- request-scoped `devflow.*.id` fields only for slow requests or requests that otherwise need incident context
- `error_message` only for client-canceled requests

Do not add by default:

- `result`
- `component`
- `event.outcome`
- unconditional request-scoped `devflow.*.id`
- unconditional `service.name`
- unconditional `service.namespace`
- unconditional `service.version`

### `http.error`

Use for:

- 4xx request completion
- real 5xx request completion
- panic recovery

Required fields in addition to shared baseline:

- `http.request.method`
- `http.route`
- `url.path`
- `http.response.status_code`
- `http.response.body.size`
- `duration_ms`
- `client.address`
- `user_agent.original`

Conditional fields:

- `http.request.body.size` when present
- request-scoped `devflow.*.id` fields
- `error_message`
- `panic` for panic recovery only

Rules:

- 4xx stays `WARN`
- real 5xx stays `ERROR`
- `context canceled` and other client disconnect paths must not be classified as `http.error`

### `dependency.client`

Use for:

- dependency call completion
- dependency call failure

Required fields in addition to shared baseline:

- `dependency`
- `action`
- `result`
- `dependency_duration_seconds`

Optional fields:

- `dependency_kind`
- `error`
- `error_code`

Do not add by default:

- HTTP request fields when the event is not an HTTP request log
- unconditional `service.name`
- unconditional `service.namespace`
- unconditional `service.version`

### `otel.exporter`

Use for:

- OpenTelemetry runtime/exporter failures

Required fields in addition to shared baseline:

- `error`

Add extra context only when it is stable and low-cardinality.

### `release.lifecycle`

Use for:

- release workflow milestones
- release state transitions
- release coordination failures

Required fields in addition to shared baseline:

- `operation`
- `resource`

Common optional fields:

- `resource_id`
- `result`
- `error`
- `error_code`
- `devflow.release.id`
- `devflow.application.id`
- `devflow.environment.id`
- stable workflow fields such as `step_name`, `step_message`, `status`, `previous_status`

### `runtime.state`

Use for:

- runtime observer sync
- runtime inspection milestones
- runtime-side release state decisions

Required fields in addition to shared baseline:

- `operation`
- `resource`

Common optional fields:

- `resource_id`
- `result`
- `error`
- `error_code`
- stable runtime fields such as `sync_source`, `strategy`, `task_name`

### `worker.lifecycle`

Use for:

- background worker start/stop/claim/process lifecycle

Required fields in addition to shared baseline:

- `operation`
- `resource`

Common optional fields:

- `resource_id`
- `result`
- `error`
- `error_code`
- worker-specific stable identifiers such as `intent_id`

### `service.lifecycle`

Use for:

- domain service create/update/delete or attach/detach milestones

Required fields in addition to shared baseline:

- `operation`
- `resource`

Common optional fields:

- `resource_id`
- `result`
- `error`
- `error_code`
- stable domain identifiers such as `devflow.application.id`, `devflow.environment.id`

### `db.query`

Use for:

- database-oriented operation milestones when a dedicated DB lifecycle event is required

Required fields in addition to shared baseline:

- `operation`
- `resource`

Common optional fields:

- `resource_id`
- `result`
- `error`
- `error_code`

Do not log raw SQL text, bind values, secrets, or tokens.

### `business.event`

Use for:

- business/domain events that do not fit a narrower lifecycle logger

Required fields in addition to shared baseline:

- the smallest stable context needed to explain the event

Preferred fields:

- `operation`
- `resource`
- `resource_id`
- `result`
- `error`
- `error_code`

## Dependency logging contract

Dependency logs should prefer this field set:

- `dependency`
- `action`
- `result`
- `dependency_duration_seconds`

Optional:

- `dependency_kind`
- `error_code`

Do not emit both `dependency_operation` and `action` for the same dependency
event. Use `action` as the canonical dependency operation field.

## HTTP request logging filter

The request logger filters low-value infrastructure paths by default:

- `/health`
- `/healthz`
- `/readyz`
- `/livez`
- `/metrics`
- `/favicon.ico`
- `/internal/status`
- `/debug/pprof*`
- `/swagger*`

Filtering those paths keeps dashboards and log search focused on user-visible or
operator-visible behavior. Health, readiness, and metrics endpoints can be hit
many times per minute by kubelet, Prometheus, and probes, while debug endpoints
such as pprof and swagger can create low-value background traffic. Logging every
success creates noise, storage cost, and false traffic volume.

The filter must not hide signals that are useful during incidents. The request
logger must still emit:

- `http.response.status_code >= 500`
- `duration_ms >= 1000`
- 4xx requests
- `ERROR` / `WARN` logs
- critical business operation logs

Ordinary successful 2xx business requests may be sampled in the future, but
sampling must never suppress the incident signals listed above.

For ordinary non-slow 2xx access logs, keep the payload minimal. Do not add:

- `component`
- `event.outcome`
- `result`
- `trace_flags`
- `deployment.environment.name`
- `container.image.digest`
- empty `GET` / `HEAD` `http.request.body.size=0`
- `http.response.status_class`
- request-scoped `devflow.application.id`
- request-scoped `devflow.release.id`
- request-scoped `devflow.manifest.id`
