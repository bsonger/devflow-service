# Service

## Ownership

- active service boundary: `network-service`
- runnable host process: `network-service`
- domain package: `internal/service/domain`
- handler package: `internal/service/transport/http`
- service package: `internal/service/service`

## Purpose

`Service` models an application-owned network service definition.
One application can own multiple services, and each service is identified by `name` within the application boundary.
The current resource models a simple Kubernetes-style internal service and is rendered as a `ClusterIP` Service by default.
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
| `name` | `string` | required | user | 服务名；在同一个 `application_id` 下必须唯一 |
| `ports` | `[]ServicePort` | required | user | 端口定义集合，至少包含一个端口 |

## Port fields

| Field | Type | Required | Writable | Description |
|---|---|---|---|---|
| `name` | `string` | conditional | user | 端口名；单端口时可选，多端口时必填且应唯一 |
| `service_port` | `int` | required | user | Service 暴露端口 |
| `target_port` | `int` | required | user | 后端目标端口 |
| `protocol` | `string` | optional | user | 传输协议，默认 `TCP` |

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
  - `ports`
- server-managed fields:
  - `id`
  - `created_at`
  - `updated_at`

### Update
- mutable fields:
  - `name`, `ports`
- request-scoped required fields:
  - `application_id`
- immutable/system-managed fields:
  - `id`, `created_at`, `deleted_at`

### Delete
- supported as soft delete through the handler surface

## Validation notes

- invalid `application_id` query/body values or `service_id` path values return `invalid_argument`
- missing records return `not_found`
- list endpoints support `name` filtering and `include_deleted`
- one application can own multiple services
- `name` must be unique within the same `application_id`
- `ports` must contain at least one entry
- `protocol` defaults to `TCP` when omitted
- current service rendering targets Kubernetes `ClusterIP`; `clusterIP` itself is system-assigned and is not a user-writable field

## Source pointers

- module: `internal/service/module.go`
- domain: `internal/service/domain/service.go`
- service: `internal/service/service/service.go`
- handler: `internal/service/transport/http/handler.go`
