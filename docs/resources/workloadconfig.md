# WorkloadConfig

## Ownership

- active service boundary: `config-service`
- runnable host process: `config-service`
- domain package: `internal/workloadconfig/domain`
- handler package: `internal/workloadconfig/transport/http`
- service package: `internal/workloadconfig/service`

## Purpose

`WorkloadConfig` stores deployment-shape configuration for an application in a specific environment.
It captures replicas, resource and probe settings, environment variables, workload type, and rollout strategy.

## Common base fields

| Field | Type | Required | Writable | Description |
|---|---|---|---|---|
| `id` | `uuid.UUID` | server-generated | no | 主键 |
| `created_at` | `time.Time` | server-generated | no | 创建时间 |
| `updated_at` | `time.Time` | server-generated | no | 更新时间 |
| `deleted_at` | `*time.Time` | optional | system-managed | 软删除时间 |

## Field table

| Field | Type | Required | Writable | Description |
|---|---|---|---|---|
| `application_id` | `uuid.UUID` | required | user | 关联应用 ID |
| `environment_id` | `string` | optional | user | 环境标识 |
| `name` | `string` | required | user | 配置名 |
| `description` | `string` | optional | user | 配置描述 |
| `replicas` | `int` | required | user | 副本数 |
| `resources` | `map[string]any` | optional | user | 资源约束 |
| `probes` | `map[string]any` | optional | user | 探针配置 |
| `env` | `[]EnvVar` | optional | user | 环境变量 |
| `labels` | `[]LabelItem` | optional | user | 扩展标签 |
| `workload_type` | `string` | optional | user | 工作负载类型 |
| `strategy` | `string` | optional | user | 发布策略 |

## API surface

- `POST /api/v1/workload-configs`
- `GET /api/v1/workload-configs`
- `GET /api/v1/workload-configs/{id}`
- `PUT /api/v1/workload-configs/{id}`
- `DELETE /api/v1/workload-configs/{id}`

## Create / update rules

### Create
- practical required fields:
  - `application_id`
  - `name`
  - `replicas`
- server-managed fields:
  - `id`
  - `created_at`
  - `updated_at`

### Update
- mutable fields:
  - `application_id`, `environment_id`, `name`, `description`, `replicas`, `resources`, `probes`, `env`, `labels`, `workload_type`, `strategy`
- immutable/system-managed fields:
  - `id`, `created_at`, `deleted_at`

### Delete
- supported as soft delete through the handler surface

## Validation notes

- invalid UUID path or query parameters return `invalid_argument`
- missing records return `not_found`
- list endpoints support `application_id`, `environment_id`, `name`, and `include_deleted`

## Source pointers

- module: `internal/workloadconfig/module.go`
- domain: `internal/workloadconfig/domain/workload_config.go`
- service: `internal/workloadconfig/service/workload_config.go`
- handler: `internal/workloadconfig/transport/http/handler.go`
