# WorkloadConfig

## Ownership

- active service boundary: `config-service`
- runnable host process: `config-service`
- domain package: `internal/workloadconfig/domain`
- handler package: `internal/workloadconfig/transport/http`
- service package: `internal/workloadconfig/service`

## Purpose

`WorkloadConfig` stores the base Deployment runtime shape for one application.

Current contract:

- one active `WorkloadConfig` per `application_id`
- current workload rendering target is `Deployment`
- rollout strategy is **not** stored here; it belongs to `Release.strategy`

## Common base fields

| Field | Type | Required | Writable | Description |
|---|---|---|---|---|
| `id` | `uuid.UUID` | server-generated | no | 资源主键 |
| `created_at` | `time.Time` | server-generated | no | 创建时间 |
| `updated_at` | `time.Time` | server-generated | no | 最后更新时间 |
| `deleted_at` | `*time.Time` | optional | system-managed | 软删除时间 |

## Field table

| Field | Type | Required | Writable | Description |
|---|---|---|---|---|
| `application_id` | `uuid.UUID` | required | create-only | 所属应用 ID；每个应用只能有一个有效 workload config |
| `replicas` | `int` | required | user | Deployment 副本数，必须 `>= 0` |
| `service_account_name` | `string` | optional | user | Pod 使用的 Kubernetes `serviceAccountName` |
| `resources` | `map[string]any` | optional | user | 容器资源配置，推荐使用 `requests` / `limits` 结构 |
| `probes` | `map[string]any` | optional | user | 容器探针配置，推荐使用 `livenessProbe` / `readinessProbe` / `startupProbe` |
| `env` | `[]EnvVar` | optional | user | 容器环境变量列表；当前只支持字面量 `{name, value}` |
| `labels` | `map[string]string` | optional | user | 写入 Deployment / Pod metadata 的 labels |
| `annotations` | `map[string]string` | optional | user | 写入 Deployment / Pod metadata 的 annotations |

## Nested types

### `EnvVar`

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | `string` | required | 环境变量名 |
| `value` | `string` | required | 环境变量值 |

## API surface

- `POST /api/v1/workload-configs`
- `GET /api/v1/workload-configs`
- `GET /api/v1/workload-configs/{id}`
- `PUT /api/v1/workload-configs/{id}`
- `DELETE /api/v1/workload-configs/{id}`

The current HTTP surface is still collection-shaped, but the resource contract is application-scoped:

- `application_id` is the business lookup key
- one active record per application

## Create / update rules

### Create

required fields:

- `application_id`
- `replicas`

optional fields:

- `service_account_name`
- `resources`
- `probes`
- `env`
- `labels`
- `annotations`

server-managed fields:

- `id`
- `created_at`
- `updated_at`

conflict rules:

- if an active record already exists for the same `application_id`, create returns `conflict`

### Update

mutable fields:

- `replicas`
- `service_account_name`
- `resources`
- `probes`
- `env`
- `labels`
- `annotations`

immutable/system-managed fields:

- `application_id`
- `id`
- `created_at`
- `deleted_at`

### Delete

- soft delete through the handler surface

## Recommended payload shapes

### `resources`

```json
{
  "requests": {
    "cpu": "100m",
    "memory": "64Mi"
  },
  "limits": {
    "cpu": "500m",
    "memory": "512Mi"
  }
}
```

### `probes`

```json
{
  "livenessProbe": {
    "httpGet": {
      "path": "/healthz",
      "port": "http"
    },
    "initialDelaySeconds": 10,
    "periodSeconds": 10,
    "timeoutSeconds": 5,
    "failureThreshold": 3
  },
  "readinessProbe": {
    "httpGet": {
      "path": "/readyz",
      "port": "http"
    },
    "initialDelaySeconds": 5,
    "periodSeconds": 10,
    "timeoutSeconds": 5,
    "failureThreshold": 3
  },
  "startupProbe": {
    "httpGet": {
      "path": "/startupz",
      "port": "http"
    },
    "periodSeconds": 5,
    "failureThreshold": 12
  }
}
```

### Example create / update body

```json
{
  "application_id": "999c0c88-1f1f-41d1-a67a-8159d07c878c",
  "replicas": 1,
  "service_account_name": "default",
  "resources": {
    "requests": {
      "cpu": "100m",
      "memory": "64Mi"
    },
    "limits": {
      "cpu": "500m",
      "memory": "512Mi"
    }
  },
  "probes": {
    "livenessProbe": {
      "httpGet": {
        "path": "/healthz",
        "port": "http"
      }
    },
    "readinessProbe": {
      "httpGet": {
        "path": "/readyz",
        "port": "http"
      }
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

## Validation notes

- invalid UUID path or query parameters return `invalid_argument`
- missing records return `not_found`
- duplicate create for the same `application_id` returns `conflict`
- `replicas` must be `>= 0`
- list endpoints support `application_id` and `include_deleted`

## Rendering boundary

`WorkloadConfig` does **not** decide whether release rendering produces:

- `Deployment`
- `Rollout`

That decision belongs to `Release.strategy`:

- `rolling` -> `Deployment`
- `blueGreen` / `canary` -> `Rollout`

## Source pointers

- module: `internal/workloadconfig/module.go`
- domain: `internal/workloadconfig/domain/workload_config.go`
- service: `internal/workloadconfig/service/workload_config.go`
- handler: `internal/workloadconfig/transport/http/handler.go`
