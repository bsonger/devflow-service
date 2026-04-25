# Environment

## Ownership

- active service: `meta-service`
- domain package: `internal/environment/domain`
- handler package: `internal/environment/transport/http`
- service package: `internal/environment/service`

## Purpose

`Environment` stores deployment-environment metadata and binds that environment to a target cluster.
It does not accept a user-managed namespace field in the current implementation.

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
| `name` | `string` | required | user | 环境名 |
| `cluster_id` | `uuid.UUID` | required | user | 目标集群 ID |
| `description` | `string` | optional | user | 环境描述 |
| `labels` | `[]LabelItem` | optional | user | 扩展标签 |

## API surface

- `POST /api/v1/environments`
- `GET /api/v1/environments`
- `GET /api/v1/environments/{id}`
- `PUT /api/v1/environments/{id}`
- `DELETE /api/v1/environments/{id}`

## Create / update rules

### Create
- required fields:
  - `name`
  - `cluster_id`
- server-managed fields:
  - `id`
  - `created_at`
  - `updated_at`

### Update
- mutable fields:
  - `name`, `cluster_id`, `description`, `labels`
- immutable/system-managed fields:
  - `id`, `created_at`, `deleted_at`

### Delete
- supported as soft delete through the handler surface

## Validation notes

- `name` and `cluster_id` must not be empty
- `cluster_id` must reference an existing `Cluster`
- invalid `cluster_id` query or path values return `invalid_argument`
- duplicate environments return `conflict`
- list endpoints support `cluster_id`, `name`, and `include_deleted`

## Source pointers

- module: `internal/environment/module.go`
- domain: `internal/environment/domain/environment.go`
- service: `internal/environment/service/service.go`
- handler: `internal/environment/transport/http/handler.go`
