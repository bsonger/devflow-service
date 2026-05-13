# Observability and Logging Policy

## Reader and outcome

This policy is for engineers and agents changing `devflow-service` code.

After reading it, a fresh reader should be able to:
- add logs, metrics, and traces without drifting from the repo standard
- choose stable field names for logs and attributes
- avoid high-cardinality metrics and sensitive-data leaks
- keep metrics, logs, and traces joinable across service, environment, trace, and release flows

## Scope

This policy governs:
- structured application logs
- application metrics and metric labels
- request and workflow trace correlation
- observability field naming used in code under `cmd/` and `internal/`

This policy complements the system-level observability overview and the verification policy.

## Signal pipeline contract

Application logs, metrics, and traces intentionally use different transport paths:

- Logs: services write structured JSON logs to stdout; the Kubernetes-local OpenTelemetry Collector DaemonSet tails pod logs from `/var/log/pods` and exports them through the logs pipeline.
- Metrics: services expose `/metrics`; Prometheus scrapes the named `metrics` service port.
- Traces: services export OTLP traces to the configured OpenTelemetry Collector endpoint.

The application logger should not bypass stdout with a second ad-hoc log sink unless the policy is updated in the same change.
The deployment-side logs collector must avoid scraping its own collector logs and should scope collection to the intended DevFlow namespaces.

## Core rules

1. Logs must be structured.
2. Application-owned field names should use `snake_case` unless the field is an approved OpenTelemetry semantic key.
3. Request, workflow, and dependency logs must prefer stable low-cardinality keys.
4. Metrics must not use high-cardinality labels.
5. Logs and traces must be correlatable through shared identifiers.
6. Sensitive values must never be emitted to logs, metrics, or trace attributes.

## Structured log contract

The detailed source-of-truth for log field ownership is `docs/observability/logging-spec.md`.
In short:

- application middleware writes request facts and DevFlow resource identifiers
- OpenTelemetry SDK / instrumentation supplies trace identifiers and service resource attributes from context and OTEL environment variables
- the OpenTelemetry Collector enriches logs with Kubernetes, host, and cloud resource attributes

Do not copy Collector-owned Kubernetes fields into business code.

### Required baseline fields

Service runtime logs should carry these fields whenever available:
- `service.name`
- `service.namespace`
- `service.version`
- `trace_id`
- `span_id`
- `trace_flags`
- `request_id`

Legacy fields such as `service`, `environment`, and `service_version` may still appear in older logs or metric labels.
They are compatibility fields, not recommended structured-log fields for new application log events.

### Preferred operation fields

Business decision-point logs should prefer these keys:
- `operation`
- `resource`
- `resource_id`
- `result`

Use them to answer:
- what operation was attempted
- what resource was affected
- which concrete instance was affected
- whether the outcome was `started`, `success`, `error`, or another explicit state

### Resource-specific fields

After the baseline fields, add domain-specific stable identifiers such as:
- `devflow.application.id`
- `devflow.project.id`
- `devflow.environment.id`
- `devflow.service.id`
- `devflow.manifest.id`
- `devflow.release.id`
- `intent_id`

Legacy snake_case identifiers such as `application_id`, `project_id`, `environment_id`, `manifest_id`, and `release_id`
may remain in existing business logs, but new HTTP request middleware logs should prefer the `devflow.*.id` keys.

Prefer explicit names such as `cluster_server`, `pipeline_run_id`, `listen_port`, and `filter_name` over ambiguous names such as `server`, `name`, `count`, or `addr`.

## Naming rules

### Allowed

- `snake_case` field names
- approved OpenTelemetry semantic log keys such as `service.name`, `deployment.environment.name`, `container.image.digest`, `http.request.method`, and `devflow.application.id`
- explicit identifiers such as `release_id`
- explicit counters such as `project_count`
- explicit filter names such as `filter_project_id`

### Disallowed

- unapproved dotted field names such as `release.id` or `error.message`
- camelCase field names such as `pipelineRun`
- generic names when a stable explicit name is available

## Metrics policy

The detailed source-of-truth for metric label naming is `docs/observability/metrics-spec.md`.
Metrics must describe the same facts as logs and traces, but use Prometheus-safe label names instead of dotted OpenTelemetry keys.

### Required low-cardinality dimensions

For HTTP and service metrics, prefer labels like:
- `service_name`
- `service_namespace`
- `deployment_environment_name`
- `http_request_method`
- `http_route`
- `http_response_status_code`
- `http_response_status_class`

For release and workflow metrics, prefer stable labels such as:
- `release_type`

Only add `stage` when the metric is explicitly a release-stage metric.

For dependency and other business/workflow metrics, prefer only the business
dimensions required for aggregation. Do not automatically repeat service-level
resource identity dimensions when the same information is already available from
resource metadata or scrape target metadata.

### Forbidden high-cardinality labels

Do not use identifiers like these as metric labels:
- `trace_id`
- `request_id`
- `user_id`
- `email`
- `phone`
- `order_id`
- `release_id`
- any raw token, session, or secret value

If a value is needed for debugging, put it in a structured log or trace attribute instead of a metric label.
For HTTP 5xx samples, attach the active trace context as a Prometheus exemplar
instead of adding `trace_id` or `span_id` as labels.

### Recommended metric label templates

Use these templates when adding new metric attributes or labels.
Prefer a small stable set over a large expressive set.

#### HTTP server metrics

Recommended labels:
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

#### Release workflow metrics

Recommended labels:
- `release_type`

Example:

```text
release_total{
  release_type="upgrade"
}
```

#### Dependency metrics

Recommended labels:
- `dependency`
- `action`
- `result`

Example:

```text
devflow_dependency_calls_total{
  dependency="runtime_service",
  action="get_runtime_spec",
  result="ok"
}
```

Use `action` as the canonical dependency operation dimension. Do not emit both
`dependency_operation` and `action` for the same dependency metric or log event.

### Metric label anti-patterns

Do not add labels like these:

```text
trace_id="..."
request_id="..."
release_id="..."
user_id="..."
email="..."
pipeline_run_id="..."
```

These values are too high-cardinality or too sensitive for metric labels.
Put them in logs or trace attributes instead.
`trace_id` and `span_id` may appear only as exemplar labels on individual
samples, not as metric labels on the time series.
Application code should not use in-process trace sampling to decide which traces
survive. Export all SDK traces and apply retention policy in the OpenTelemetry
Collector, where 5xx traces must be kept.
The pre-production Collector-side retention contract is documented in
`docs/observability/trace-retention-policy.md` and deployed through
`deployments/pre-production/otel-trace-gateway.yaml`.
Application middleware may path-filter trace creation only for these approved
low-value routes:

- `/health`
- `/healthz`
- `/readyz`
- `/livez`
- `/metrics`
- `/favicon.ico`
- `/internal/status`
- `/debug/pprof*`
- `/swagger*`

That is route filtering, not downsampling. Application code must not apply
ratio-based trace sampling to the remaining traffic; downsampling belongs in the
OpenTelemetry Collector.
This is slightly different from logs and metrics: low-value path failures still
produce logs and metrics, but they do not have trace exemplars when the route was
filtered before trace creation.

Legacy metric labels such as `service`, `environment`, `method`, `route`, and `status_code`
may appear in dashboards during rollout compatibility windows, but new application metrics must use the canonical labels above.

### Generic metric label anti-patterns

Avoid generic labels when a stable explicit name is available.

Avoid:

```text
name="..."
type="..."
id="..."
status="..."
```

Prefer:

```text
http_route="..."
release_type="..."
http_response_status_code="200"
result="success"
dependency="runtime_service"
```

## Trace correlation policy

HTTP and gRPC request handling must preserve trace context.

When available, logs should include:
- `trace_id`
- `span_id`

Release, image, manifest, and intent flows should also emit stable workflow identifiers so that a reader can move between:
- request logs
- dependency logs
- release workflow logs
- traces

## Sensitive data policy

Do not emit any of the following into logs, metrics, or trace attributes:
- passwords
- tokens
- secrets
- private keys
- cookies
- authorization headers
- full personal identifiers

If a value is operationally useful, log a redacted or summarized form instead.

## Decision-point logging policy

Prefer logs that explain a decision over logs that only announce activity.

Good examples:
- a release status changed
- a dependency call failed
- a bootstrap step failed
- a manifest artifact was published
- a pipeline run was created

Avoid low-value logs that only repeat control flow without context.

## Failure-mode policy

Do not silently swallow production failures.

When an operation fails in a way the caller cannot safely ignore:

- log the failure with structured context
- return an explicit error when possible
- keep the failure state visible through an existing status surface, health surface, or persisted state owned by that subsystem

Avoid empty `catch`-style handling patterns, ignored returned errors, or vague fallback behavior without an explicit signal.

## Health and status surface policy

Long-running processes and servers should expose a cheap status surface whenever practical.

Examples include:

- `healthz`
- `readyz`
- metrics endpoint status
- a runtime status file or equivalent persisted state for background workers

The goal is that a fresh engineer or agent can tell whether the process is healthy, degraded, or stuck without adding ad-hoc debug logging first.

## Verification expectations

When changing observability behavior:
- keep field naming consistent with this policy
- keep metrics labels low-cardinality
- preserve request and trace correlation behavior
- verify the affected packages still pass formatting, tests, and builds under the repo verification contract

## Change guidance

When adding a new log statement, check:
1. does it need `operation`
2. does it need `resource`
3. does it need `resource_id`
4. is `result` explicit
5. are all application-owned field names `snake_case`, or is the field an approved OpenTelemetry semantic key
6. did I avoid sensitive values
7. did I avoid introducing a high-cardinality metric label

## Recommended log templates

Use these as templates, not rigid copy-paste rules.
Choose the smallest set of fields that explains the decision or outcome clearly.

### HTTP request completion

Use for request summary logs emitted by HTTP middleware.

Recommended fields:
- `severity_text`
- `timestamp`
- `logger.name`
- `caller`
- `body`
- `http.request.method`
- `http.route`
- `url.path`
- `http.response.status_code`
- `duration_ms`
- `http.server.request.duration`
- `http.response.body.size`
- `client.address`
- `user_agent.original`
- `trace_id`
- `span_id`
- `trace_flags`
- `request_id`

Conditional fields:
- `http.request.body.size` when a request body is present or when the method is not `GET` / `HEAD`
- `trace_flags`
- `request_id`

Conditional fields:
- `http.request.body.size` when a request body is present or when the method is not `GET` / `HEAD`
- `devflow.project.id`
- `devflow.application.id`
- `devflow.service.id`
- `devflow.environment.id`
- `devflow.release.id`
- `devflow.manifest.id`

Ordinary non-slow 2xx access logs should omit:
- `component`
- `event.outcome`
- `result`
- `trace_flags`
- `deployment.environment.name`
- `container.image.digest`
- `http.response.status_class`
- empty `GET` / `HEAD` `http.request.body.size=0`
- request-scoped `devflow.application.id`
- request-scoped `devflow.release.id`
- request-scoped `devflow.manifest.id`

Example:

```text
body="http request"
logger.name="http.access"
http.request.method="GET"
http.route="/api/v1/releases/:id"
url.path="/api/v1/releases/123"
http.response.status_code=200
http.response.status_class="2xx"
duration_ms=143.217
http.server.request.duration=0.143217
http.response.body.size=244
client.address="10.0.0.8"
user_agent.original="curl/8.7.1"
trace_id="..."
span_id="..."
trace_flags="01"
request_id="..."
devflow.release.id="..."
```

HTTP 4xx / 5xx request logs and panic logs must use `logger.name="http.error"`.
`caller` may stay in every record as a debugging aid, but it must not become a
metrics label, Loki stream label, dashboard variable, or primary log category.
Within `http.error`, 4xx should log as client-side request failures and 5xx
should log as server-side failures; keep the severity split (`WARN` for 4xx,
`ERROR` for 5xx).

### Repository read or write

Use for repository-owned persistence logs.

Recommended fields:
- `operation`
- `resource`
- `resource_id`
- `result`
- resource-specific identifiers such as `application_id` or `project_id`

Example:

```text
message="application fetched"
operation="get_application"
resource="application"
resource_id="..."
result="success"
application_name="platform-web"
trace_id="..."
```

### Workflow or release step

Use for release, image, manifest, and intent workflow milestones.

Recommended fields:
- `operation`
- `resource`
- `resource_id`
- `result`
- workflow identifiers such as `release_id`, `manifest_id`, `intent_id`
- stable status or step fields such as `status`, `previous_status`, `step_name`, `step_message`

Example:

```text
message="release status updated"
operation="update_release_status"
resource="release"
resource_id="..."
result="success"
previous_status="pending"
status="syncing"
release_id="..."
manifest_id="..."
trace_id="..."
```

### Dependency call

Use for outbound dependency boundaries.

Recommended fields:
- `dependency`
- `dependency_kind`
- `action`
- `dependency_duration_seconds`
- `result`
- optional `error_code`

Example:

```text
message="dependency call failed"
dependency="runtime_service"
dependency_kind="http"
action="get_runtime_spec"
dependency_duration_seconds=1.42
result="error"
error_code="upstream_unavailable"
trace_id="..."
```

### Startup and initialization

Use for service bootstrap and client initialization.

Recommended fields:
- explicit port or address fields such as `listen_port`, `metrics_listen_port`, `server_address`
- optional lifecycle context such as `server_kind`, `listen_addr`, `application_name`

Example:

```text
body="starting service"
service.name="release-service"
listen_port=8083
metrics_listen_port=9090
pprof_listen_port=6060
```

## Example anti-patterns

Avoid examples like these:

```text
release.id="..."
pipelineRun="..."
name="..."
count=7
```

Prefer:

```text
release_id="..."
pipeline_run_id="..."
filter_name="..."
project_count=7
```
