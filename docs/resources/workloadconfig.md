# WorkloadConfig

## 这个文档解决什么问题

这份文档说明 `WorkloadConfig` 如何表达应用级 workload 基线，以及它如何进入 `Manifest.workload_config_snapshot`。

读完后，读者应该能回答：

- `WorkloadConfig` 为什么是 application-scoped
- 它和 `AppConfig` 的边界是什么
- 为什么 release render 会消费它，但不会把它变成环境级资源

## Ownership

- active service boundary: `config-service`
- runnable host process: `config-service`
- domain package: `internal/workloadconfig/domain`
- handler package: `internal/workloadconfig/transport/http`
- service package: `internal/workloadconfig/service`

## Purpose

`WorkloadConfig` stores the application-scoped runtime workload contract that downstream manifest and release flows freeze and later translate into Kubernetes container fields.

The active write contract is intentionally constrained:

- one active `WorkloadConfig` per `application_id`
- `resources` accepts a canonical `size_class` selector on create/update writes
- caller-supplied `resources.requests` and `resources.limits` are rejected on writes as legacy wide payload fields
- `probes` is a typed object with only `liveness`, `readiness`, and `startup` slots
- probe `path` values must start with `/`, and `port` is required whenever a probe `path` is set
- `env` remains an ordered array of repeated `{name,value}` rows and rejects duplicate `name` entries
- `metrics` is a typed contract with `enabled`, `port`, and `scrape_profile`
- `labels` and `annotations` remain explicit user inputs
- rollout strategy is **not** stored here; it belongs to `Release.strategy`
- render-time expansion into Kubernetes `resources`, `livenessProbe`, `readinessProbe`, `startupProbe`, and similar fields happens downstream during manifest/release rendering

## Common base fields

| Field | Type | Required | Writable | Description |
|---|---|---|---|---|
| `id` | `uuid.UUID` | server-generated | no | Resource primary key |
| `created_at` | `time.Time` | server-generated | no | Create timestamp |
| `updated_at` | `time.Time` | server-generated | no | Last update timestamp |
| `deleted_at` | `*time.Time` | optional | system-managed | Soft-delete timestamp |

## Field table

| Field | Type | Required | Writable | Description |
|---|---|---|---|---|
| `application_id` | `uuid.UUID` | required | create-only | Owning application ID. One active workload config is allowed per application. |
| `replicas` | `int` | required | user | Desired replica count. Current contract requires `>= 0`. |
| `service_account_name` | `string` | optional | user | Pod `serviceAccountName` to apply at render time. |
| `resources` | `WorkloadResourceRequirements` | optional | user | Constrained resource selector. Writes must provide a valid `size_class` and must not send `requests` or `limits`. |
| `probes` | `WorkloadProbes` | optional | user | Typed HTTP probe contract with only `liveness`, `readiness`, and `startup` slots. |
| `metrics` | `WorkloadMetrics` | optional | user | Typed metrics exposure contract. `enabled=true` requires `port > 0`. `scrape_profile` defaults to `default` and is constrained to `default`, `fast`, or `slow`. |
| `env` | `[]EnvVar` | optional | user | Ordered literal environment variable rows. Duplicate `name` values are rejected on writes. |
| `labels` | `map[string]string` | optional | user | Labels copied into rendered workload metadata. |
| `annotations` | `map[string]string` | optional | user | Annotations copied into rendered workload metadata. |

## Nested types

### `EnvVar`

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | `string` | required | Environment variable name. Must be non-empty and unique within the `env` array. |
| `value` | `string` | required | Environment variable value |

### `WorkloadResourceList`

`WorkloadResourceList` remains part of the shared frozen model because downstream manifest and release consumers expand a size class into concrete Kubernetes CPU and memory values. It is **not** a valid create/update write shape.

| Field | Type | Required | Description |
|---|---|---|---|
| `cpu` | `string` | optional | Kubernetes CPU quantity string |
| `memory` | `string` | optional | Kubernetes memory quantity string |

### `WorkloadResourceRequirements`

| Field | Type | Required | Description |
|---|---|---|---|
| `size_class` | `WorkloadSizeClass` | required on write | Canonical workload size selector: `small`, `medium`, `large`, `xlarge` |
| `requests` | `WorkloadResourceList` | read-only / legacy payload rejection on write | Concrete request values derived downstream from the canonical size-class mapping table |
| `limits` | `WorkloadResourceList` | read-only / legacy payload rejection on write | Concrete limit values derived downstream from the canonical size-class mapping table |

### `WorkloadProbe`

| Field | Type | Required | Description |
|---|---|---|---|
| `path` | `string` | optional | HTTP probe path. When set, it must start with `/`. |
| `port` | `string` | conditionally required | Named or numeric port string. Required whenever `path` is set. |
| `initial_delay_seconds` | `int` | optional | Delay before first probe |
| `period_seconds` | `int` | optional | Probe interval |
| `timeout_seconds` | `int` | optional | Per-probe timeout |
| `failure_threshold` | `int` | optional | Failure count before probe is considered failed |

### `WorkloadProbes`

| Field | Type | Required | Description |
|---|---|---|---|
| `liveness` | `*WorkloadProbe` | optional | Liveness probe contract |
| `readiness` | `*WorkloadProbe` | optional | Readiness probe contract |
| `startup` | `*WorkloadProbe` | optional | Startup probe contract |

### `WorkloadMetrics`

| Field | Type | Required | Description |
|---|---|---|---|
| `enabled` | `bool` | optional | Whether the workload exposes a Prometheus metrics listener. |
| `port` | `int` | required when `enabled=true` | Metrics listener port. Must be `> 0` when metrics are enabled. |
| `scrape_profile` | `WorkloadMetricsScrapeProfile` | optional | Shared `ServiceMonitor` profile. Allowed values: `default`, `fast`, `slow`. Defaults to `default` when metrics are enabled. |

## Fixed size-class mapping table

The canonical source of truth is `internal/workloadconfig/domain.WorkloadSizeClassResources`.
All downstream validation, migration, docs, and render-time expansion must use this shared table instead of inventing parallel defaults.

| Size class | Requests CPU | Requests Memory | Limits CPU | Limits Memory |
|---|---:|---:|---:|---:|
| `small` | `100m` | `128Mi` | `500m` | `512Mi` |
| `medium` | `250m` | `256Mi` | `1` | `1Gi` |
| `large` | `500m` | `512Mi` | `2` | `2Gi` |
| `xlarge` | `1` | `1Gi` | `4` | `4Gi` |

## API surface

Service-internal route surface:

- `POST /api/v1/workload-configs`
- `GET /api/v1/workload-configs`
- `GET /api/v1/workload-configs/{id}`
- `PUT /api/v1/workload-configs/{id}`
- `DELETE /api/v1/workload-configs/{id}`

Pre-production shared ingress external surface:

- `POST /api/v1/config/workload-configs`
- `GET /api/v1/config/workload-configs`
- `GET /api/v1/config/workload-configs/{id}`
- `PUT /api/v1/config/workload-configs/{id}`
- `DELETE /api/v1/config/workload-configs/{id}`

The current HTTP surface is collection-shaped, but the business contract is application-scoped:

- `application_id` is the business lookup key
- one active record per application

## Create / update rules

### Create

Required fields:

- `application_id`
- `replicas`
- `resources.size_class`

Optional fields:

- `service_account_name`
- `probes`
- `metrics`
- `env`
- `labels`
- `annotations`

Server-managed fields:

- `id`
- `created_at`
- `updated_at`

Conflict rule:

- if an active record already exists for the same `application_id`, create returns `conflict`

### Update

Mutable fields:

- `replicas`
- `service_account_name`
- `resources`
- `probes`
- `metrics`
- `env`
- `labels`
- `annotations`

Immutable/system-managed fields:

- `application_id`
- `id`
- `created_at`
- `deleted_at`

### Delete

- soft delete through the handler surface

## Write payloads

### Accepted write shape

```json
{
  "application_id": "999c0c88-1f1f-41d1-a67a-8159d07c878c",
  "replicas": 1,
  "service_account_name": "default",
  "resources": {
    "size_class": "medium"
  },
  "probes": {
    "liveness": {
      "path": "/healthz",
      "port": "http",
      "initial_delay_seconds": 10,
      "period_seconds": 10,
      "timeout_seconds": 5,
      "failure_threshold": 3
    },
    "readiness": {
      "path": "/readyz",
      "port": "http",
      "initial_delay_seconds": 5,
      "period_seconds": 10,
      "timeout_seconds": 5,
      "failure_threshold": 3
    },
    "startup": {
      "path": "/startupz",
      "port": "http",
      "period_seconds": 5,
      "failure_threshold": 12
    }
  },
  "metrics": {
    "enabled": true,
    "port": 9090,
    "scrape_profile": "default"
  },
  "env": [
    {
      "name": "LOG_LEVEL",
      "value": "info"
    },
    {
      "name": "APP_MODE",
      "value": "worker"
    }
  ],
  "labels": {
    "team": "platform"
  },
  "annotations": {
    "sidecar.istio.io/inject": "true"
  }
}
```

### Rejected legacy wide write shape

The following fields are rejected on create/update writes because they belong to the old wide resource model rather than the constrained backend contract:

```json
{
  "application_id": "999c0c88-1f1f-41d1-a67a-8159d07c878c",
  "replicas": 1,
  "resources": {
    "size_class": "medium",
    "requests": {
      "cpu": "250m",
      "memory": "256Mi"
    },
    "limits": {
      "cpu": "1",
      "memory": "1Gi"
    }
  }
}
```

Transport behavior distinguishes two failure classes:

- malformed constrained payloads return `invalid_argument`
- explicit legacy wide resource writes return `failed_precondition`

## Validation semantics

The active handler/service contract enforces these write rules:

- invalid UUID path or query parameters return `invalid_argument`
- missing records return `not_found`
- duplicate create for the same `application_id` returns `conflict`
- `replicas` must be `>= 0`
- `resources.size_class` must be one of `large`, `medium`, `small`, `xlarge`
- `resources.requests` must not be provided on write
- `resources.limits` must not be provided on write
- probe `path` must start with `/` when set
- probe `port` is required whenever a probe `path` is set
- duplicate `env[*].name` entries are rejected
- empty `env[*].name` entries are rejected
- `metrics.port` must be `> 0` when `metrics.enabled=true`
- `metrics.scrape_profile` must be `default`, `fast`, or `slow`
- list endpoints support `application_id` and `include_deleted`

## Legacy cleanup policy: migrate or delete on read

Older stored rows may still carry the previous free-form resource or probe shape. The active repository read contract is deterministic:

1. **migrate in place on read** when a stored row can be translated losslessly into the constrained contract
2. **soft-delete on read** when a stored row cannot be translated without inventing data or preserving an unsupported shape
3. **reject on write** when a caller sends legacy wide resource fields or any other unsupported create/update payload shape

The repository boundary enforces this behavior during `Get` and `List` reads so downstream manifest and release consumers either see:

- the constrained canonical `WorkloadConfig`, or
- the same outcome as a missing row

They do **not** need compatibility branches for legacy stored shapes.

### Deterministic migration rule

A stored row is preserved only when every legacy field maps exactly to the constrained model already implemented in the service:

- `resources` must match one row in `internal/workloadconfig/domain.WorkloadSizeClassResources`
- a typed `resources.size_class` row is kept only when any stored `requests`/`limits` values, if present, exactly match that size class's canonical values
- legacy `requests` + `limits` maps are migrated only when they exactly match one canonical size-class row
- `probes` must already decode into the typed `liveness` / `readiness` / `startup` structure
- `env` must decode into the ordered `[]EnvVar` shape with non-empty unique names
- empty `labels`, `annotations`, `resources`, or `env` payloads are normalized to the current canonical empty form

If those checks succeed, the repository rewrites the row with canonical JSON and refreshes `updated_at`.

### Delete-on-incompatible-read rule

A stored row is soft-deleted instead of being surfaced when any read-time normalization check fails, including cases such as:

- resource quantities that do not match any canonical size class
- typed `size_class` data whose stored `requests` or `limits` disagree with the canonical mapping
- malformed JSON in the stored resource, probe, env, label, or annotation payloads
- `env` entries with empty names or duplicates
- any other stored shape that cannot be mapped losslessly into the constrained contract

For `Get`, the caller receives the same result as a missing active row.
For `List`, incompatible rows are omitted from the result set and then soft-deleted after the cursor closes.

This keeps legacy cleanup observable through normal repository-backed list/get behavior instead of ad-hoc database inspection.

## Removed legacy fields

These fields are intentionally no longer part of `WorkloadConfig`:

- `name`
- `description`
- `workload_type`
- `strategy`

Reasons:

- one application has one workload config, so extra naming is not needed
- description has no runtime value
- workload kind is not chosen here
- rollout strategy belongs to `Release`

## Rendering boundary

Manifest creation freezes the canonical workload-config payload into `Manifest.workload_config_snapshot`, and release preview/render flows consume that frozen snapshot plus persisted release bundle records. They do not re-read live workload-config rows during later bundle preview or render paths.

That means verification splits cleanly across two boundaries:

- repository/list/get regressions explain live-row migration or delete behavior
- manifest/release regressions explain frozen snapshot and persisted bundle consumption behavior

`WorkloadConfig` does **not** decide whether downstream rendering produces:

- `Deployment`
- `Rollout`

That decision belongs to `Release.strategy`:

- `rolling` -> `Deployment`
- `blueGreen` / `canary` -> `Rollout`

It also does **not** directly store rendered Kubernetes field names such as:

- `livenessProbe`
- `readinessProbe`
- `startupProbe`
- container `resources`
- Service `metrics` port
- `observability.devflow.io/scrape*` labels

Instead:

- this resource stores the frozen typed contract
- manifest/release renderers translate that contract into Kubernetes-shaped output later, including `METRICS_PORT`, the Service `metrics` port, and shared `ServiceMonitor` selection labels

## Observability and drift checks

The anti-drift proof surfaces for this contract are:

- `internal/workloadconfig/domain/workload_config_contract_test.go`
- `internal/workloadconfig/transport/http/handler_test.go`
- `internal/workloadconfig/repository/repository_test.go` for read-time migrate-or-delete behavior
- downstream mirror contract tests under `internal/manifest/...` and `internal/release/...`, including frozen manifest snapshot and persisted bundle preview coverage
- canonical config-service OpenAPI in `api/openapi/config-service.yaml`
- aggregate OpenAPI in `api/openapi/devflow.yaml`
- generated Swagger snapshot in `api/openapi/swagger.yaml`
- repo verification via `make openapi-check` and `bash scripts/verify.sh`
- pre-production shared-ingress operator proof via `test/workloadconfig/preprod_workload_config_flow.sh` and `test/workloadconfig/README.md`

The tracked pre-production probe is intentionally collection-aware and captures explicit status/body checkpoints for:

- `GET /api/v1/config/workload-configs?application_id=...`
- `GET /api/v1/config/workload-configs/{id}`
- `PUT /api/v1/config/workload-configs/{id}`
- read-after-write re-fetch and post-update list inspection

Use that script to localize whether a live failure belongs to shared-ingress auth/routing, handler validation (`invalid_argument`, `failed_precondition`), or repository cleanup / missing-row behavior.

When these surfaces disagree, treat that as contract drift and update code, OpenAPI contracts, generated artifacts, and docs together.

## Source pointers

- module: `internal/workloadconfig/module.go`
- domain: `internal/workloadconfig/domain/workload_config.go`
- service: `internal/workloadconfig/service/workload_config.go`
- handler: `internal/workloadconfig/transport/http/handler.go`
