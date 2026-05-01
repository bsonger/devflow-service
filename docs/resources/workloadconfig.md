# WorkloadConfig

## Ownership

- active service boundary: `config-service`
- runnable host process: `config-service`
- domain package: `internal/workloadconfig/domain`
- handler package: `internal/workloadconfig/transport/http`
- service package: `internal/workloadconfig/service`

## Purpose

`WorkloadConfig` stores the application-scoped runtime workload contract that downstream manifest and release flows freeze and later translate into Kubernetes container fields.

Current target contract for this slice:

- one active `WorkloadConfig` per `application_id`
- workload runtime shape is intentionally constrained
- `resources` is a typed object with one canonical size-class mapping table
- `probes` is a typed object with named HTTP probes, not a free-form map
- `env`, `labels`, and `annotations` remain explicit user inputs
- rollout strategy is **not** stored here; it belongs to `Release.strategy`
- render-time expansion into Kubernetes `resources`, `livenessProbe`, `readinessProbe`, `startupProbe`, and similar fields happens downstream during manifest/release rendering

This document freezes the target resource contract for S02/S03/S04. It does **not** claim every validation and legacy-migration mechanic is already fully enforced in the current handlers.

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
| `resources` | `WorkloadResourceRequirements` | optional | user | Constrained resource contract. Preferred input is `size_class`; `requests` / `limits` remain part of the frozen shape for downstream translation and legacy migration handling. |
| `probes` | `WorkloadProbes` | optional | user | Named probe contract with `liveness`, `readiness`, and `startup` slots. |
| `env` | `[]EnvVar` | optional | user | Literal environment variable entries. |
| `labels` | `map[string]string` | optional | user | Labels copied into rendered workload metadata. |
| `annotations` | `map[string]string` | optional | user | Annotations copied into rendered workload metadata. |

## Nested types

### `EnvVar`

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | `string` | required | Environment variable name |
| `value` | `string` | required | Environment variable value |

### `WorkloadResourceList`

| Field | Type | Required | Description |
|---|---|---|---|
| `cpu` | `string` | optional | Kubernetes CPU quantity string |
| `memory` | `string` | optional | Kubernetes memory quantity string |

### `WorkloadResourceRequirements`

| Field | Type | Required | Description |
|---|---|---|---|
| `size_class` | `WorkloadSizeClass` | optional | Canonical workload size selector: `small`, `medium`, `large`, `xlarge` |
| `requests` | `WorkloadResourceList` | optional | Frozen request values used for compatibility, migration, and downstream rendering |
| `limits` | `WorkloadResourceList` | optional | Frozen limit values used for compatibility, migration, and downstream rendering |

### `WorkloadProbe`

| Field | Type | Required | Description |
|---|---|---|---|
| `path` | `string` | optional | HTTP probe path |
| `port` | `string` | optional | Named or numeric port string |
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

## Fixed size-class mapping table

The canonical source of truth is `internal/workloadconfig/domain.WorkloadSizeClassResources`.
All downstream validation, migration, docs, and render-time expansion must use this shared table instead of inventing parallel defaults.

| Size class | Requests CPU | Requests Memory | Limits CPU | Limits Memory |
|---|---:|---:|---:|---:|
| `small` | `100m` | `64Mi` | `500m` | `512Mi` |
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

Optional fields:

- `service_account_name`
- `resources`
- `probes`
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

## Recommended payload shape

The preferred target payload is the constrained typed model.

```json
{
  "application_id": "999c0c88-1f1f-41d1-a67a-8159d07c878c",
  "replicas": 1,
  "service_account_name": "default",
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
  "env": [
    {
      "name": "LOG_LEVEL",
      "value": "info"
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

## Validation semantics

### Implemented now

The current handler/service contract already documents or enforces these baseline rules:

- invalid UUID path or query parameters return `invalid_argument`
- missing records return `not_found`
- duplicate create for the same `application_id` returns `conflict`
- `replicas` must be `>= 0`
- list endpoints support `application_id` and `include_deleted`

### Frozen target rules for downstream slices

The following rules are part of the target contract and should be implemented consistently across validation, migration, and render-time translation as later slices land:

- **size-class alignment**: when `resources.size_class` is present, the effective `requests` and `limits` must match the canonical mapping table above
- **duplicate env rejection**: writes should reject duplicate `env[*].name` entries after normalization instead of silently choosing a winner
- **probe path rejection**: writes should reject malformed or empty probe `path` values when a probe block is present
- **probe slot exclusivity**: only `liveness`, `readiness`, and `startup` are valid top-level probe slots; wide map keys from the legacy shape are not part of the target contract
- **write rejection over silent coercion**: invalid constrained payloads should fail the write path rather than being partially accepted and normalized invisibly

This is intentional wording, not drift: the contract is frozen now so S02/S04 can build to it, while some enforcement still remains to be implemented.

## Legacy cleanup policy: migrate or delete

Older data may still carry the previous free-form resource/probe shape. The target policy is deterministic:

1. **migrate** legacy records that can be translated losslessly into the constrained contract
2. **delete or reject** legacy records that cannot be translated without guessing

Practical rule:

- if a legacy record cleanly maps to one canonical size class plus named probe slots, it should be migrated into the constrained form
- if it depends on arbitrary map keys, conflicting env names, ambiguous probe keys, or resource values that do not match a canonical mapping row, the system must not invent a new shape; the record should be removed or the write rejected

This policy exists to prevent the repo from carrying two equivalent workload contract models indefinitely.

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

Instead:

- this resource stores the frozen typed contract
- manifest/release renderers translate that contract into Kubernetes-shaped output later

## Observability and drift checks

The anti-drift proof surfaces for this contract are:

- `internal/workloadconfig/domain/workload_config_contract_test.go`
- downstream mirror contract tests under `internal/manifest/...` and `internal/release/...`
- generated OpenAPI in `api/openapi/swagger.yaml`
- final repo verification via `bash scripts/verify.sh`

When these surfaces disagree, treat that as contract drift and update code, generated artifacts, and docs together.

## Source pointers

- module: `internal/workloadconfig/module.go`
- domain: `internal/workloadconfig/domain/workload_config.go`
- service: `internal/workloadconfig/service/workload_config.go`
- handler: `internal/workloadconfig/transport/http/handler.go`
