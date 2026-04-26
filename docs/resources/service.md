# Service

## Ownership

- active service boundary: `network-service`
- runnable host process: `network-service`
- domain package: `internal/appservice/domain`
- handler package: `internal/appservice/transport/http`
- service package: `internal/appservice/service`

## Purpose

`Service` models an application-owned network service definition.
It stores the logical service name plus the exposed service ports and target ports used by route validation and release-time manifest assembly.

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
| `application_id` | `uuid.UUID` | required | user | 所属应用 ID |
| `name` | `string` | required | user | 服务名 |
| `ports` | `[]ServicePort` | optional | user | 端口定义集合 |

## Port fields

| Field | Type | Required | Writable | Description |
|---|---|---|---|---|
| `name` | `string` | optional | user | 端口名 |
| `service_port` | `int` | required | user | Service 暴露端口 |
| `target_port` | `int` | required | user | 后端目标端口 |
| `protocol` | `string` | optional | user | 传输协议 |

## API surface

- `POST /api/v1/services`
- `GET /api/v1/services`
- `PATCH /api/v1/services/{service_id}`
- `DELETE /api/v1/services/{service_id}?application_id=...`

## Create / update rules

### Create
- required fields:
  - `application_id`
  - `name`
- server-managed fields:
  - `id`
  - `created_at`
  - `updated_at`

### Update
- mutable fields:
  - `name`, `ports`
- immutable/system-managed fields:
  - `id`, `created_at`, `deleted_at`

### Delete
- supported as soft delete through the handler surface

## Validation notes

- invalid `application_id` query/body values or `service_id` path values return `invalid_argument`
- missing records return `not_found`
- list endpoints support `name` filtering and `include_deleted`

## Source pointers

- module: `internal/appservice/module.go`
- domain: `internal/appservice/domain/service.go`
- service: `internal/appservice/service/service.go`
- handler: `internal/appservice/transport/http/handler.go`
