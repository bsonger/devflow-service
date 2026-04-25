# Route

## Ownership

- active service boundary: `network-service`
- runnable host process: `network-service`
- domain package: `internal/approute/domain`
- handler package: `internal/approute/transport/http`
- service package: `internal/approute/service`

## Purpose

`Route` models an application-owned ingress route.
It stores host/path matching plus the target service name and port, and it exposes an explicit validation endpoint used to check route-to-service consistency.

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
| `application_id` | `uuid.UUID` | path-bound | no | 所属应用 ID |
| `name` | `string` | required | user | 路由名 |
| `host` | `string` | required | user | 主机名匹配 |
| `path` | `string` | required | user | 路径匹配 |
| `service_name` | `string` | required | user | 目标服务名 |
| `service_port` | `int` | required | user | 目标服务端口 |

## API surface

- `POST /api/v1/applications/{application_id}/routes`
- `GET /api/v1/applications/{application_id}/routes`
- `PATCH /api/v1/applications/{application_id}/routes/{route_id}`
- `DELETE /api/v1/applications/{application_id}/routes/{route_id}`
- `POST /api/v1/applications/{application_id}/routes:validate`

## Create / update rules

### Create
- required fields:
  - `application_id` in path
  - `name`
  - `host`
  - `path`
  - `service_name`
  - `service_port`
- server-managed fields:
  - `id`
  - `created_at`
  - `updated_at`

### Update
- mutable fields:
  - `name`, `host`, `path`, `service_name`, `service_port`
- immutable/system-managed fields:
  - `id`, `application_id`, `created_at`, `deleted_at`

### Delete
- supported as soft delete through the handler surface

## Validation notes

- invalid `application_id` or `route_id` path values return `invalid_argument`
- missing records return `not_found`
- list endpoints support `name` filtering and `include_deleted`
- `POST /routes:validate` returns `valid` plus validation errors without persisting the route

## Source pointers

- module: `internal/approute/module.go`
- domain: `internal/approute/domain/route.go`
- service: `internal/approute/service/route.go`
- handler: `internal/approute/transport/http/handler.go`
