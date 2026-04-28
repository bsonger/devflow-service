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

## Core rules

1. Logs must be structured.
2. Field names must use `snake_case`.
3. Request, workflow, and dependency logs must prefer stable low-cardinality keys.
4. Metrics must not use high-cardinality labels.
5. Logs and traces must be correlatable through shared identifiers.
6. Sensitive values must never be emitted to logs, metrics, or trace attributes.

## Structured log contract

### Required baseline fields

Service runtime logs should carry these fields whenever available:
- `service`
- `environment`
- `service_version`
- `trace_id`
- `span_id`
- `request_id`

### Preferred operation fields

Decision-point logs should prefer these keys:
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
- `application_id`
- `project_id`
- `environment_id`
- `cluster_id`
- `manifest_id`
- `release_id`
- `intent_id`

Prefer explicit names such as `cluster_server`, `pipeline_run_id`, `listen_port`, and `filter_name` over ambiguous names such as `server`, `name`, `count`, or `addr`.

## Naming rules

### Allowed

- `snake_case` field names
- explicit identifiers such as `release_id`
- explicit counters such as `project_count`
- explicit filter names such as `filter_project_id`

### Disallowed

- dotted field names such as `release.id` or `error.message`
- camelCase field names such as `pipelineRun`
- generic names when a stable explicit name is available

## Metrics policy

### Required low-cardinality dimensions

For HTTP and service metrics, prefer labels like:
- `service`
- `environment`
- `method`
- `route`
- `status_code`

For release and workflow metrics, prefer stable labels such as:
- `service`
- `environment`
- `release_type`

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

### Recommended metric label templates

Use these templates when adding new metric attributes or labels.
Prefer a small stable set over a large expressive set.

#### HTTP server metrics

Recommended labels:
- `service`
- `environment`
- `method`
- `route`
- `status_code`

Example:

```text
http_server_requests_total{
  service="meta-service",
  environment="prod",
  method="GET",
  route="/api/v1/applications/:id",
  status_code="200"
}
```

#### Release workflow metrics

Recommended labels:
- `service`
- `environment`
- `release_type`

Example:

```text
release_total{
  service="release-service",
  environment="prod",
  release_type="upgrade"
}
```

#### Dependency metrics

Recommended labels:
- `service`
- `dependency`
- `action`
- `result`

Example:

```text
devflow_dependency_calls_total{
  service="release-service",
  dependency="runtime_service",
  action="get_runtime_spec",
  result="ok"
}
```

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
route="..."
release_type="..."
status_code="200"
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
5. are all field names `snake_case`
6. did I avoid sensitive values
7. did I avoid introducing a high-cardinality metric label

## Recommended log templates

Use these as templates, not rigid copy-paste rules.
Choose the smallest set of fields that explains the decision or outcome clearly.

### HTTP request completion

Use for request summary logs emitted by HTTP middleware.

Recommended fields:
- `component`
- `result`
- `method`
- `route`
- `path`
- `status_code`
- `duration_ms`
- `request_size_bytes`
- `response_size_bytes`
- `client_ip`
- `user_agent`
- `trace_id`
- `request_id`

Example:

```text
message="http request"
component="http_server"
result="2xx"
method="POST"
route="/api/v1/releases"
path="/api/v1/releases"
status_code=201
duration_ms=143
request_size_bytes=512
response_size_bytes=244
client_ip="10.0.0.8"
user_agent="curl/8.7.1"
trace_id="..."
request_id="..."
```

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
- `operation`
- `resource`
- `component`
- `dependency`
- `dependency_kind`
- `dependency_operation`
- `dependency_duration_seconds`
- `result`

Example:

```text
message="dependency call failed"
operation="dependency_call"
resource="dependency"
component="dependency_client"
dependency="runtime_service"
dependency_kind="http"
dependency_operation="get_runtime_spec"
dependency_duration_seconds=1.42
result="error"
error_code="upstream_unavailable"
trace_id="..."
```

### Startup and initialization

Use for service bootstrap and client initialization.

Recommended fields:
- `operation`
- `resource`
- `result`
- explicit port or address fields such as `listen_port`, `metrics_listen_port`, `server_address`

Example:

```text
message="starting service"
service="release-service"
operation="service_start"
resource="service"
result="starting"
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
