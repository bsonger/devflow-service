# Route

## Ownership

- active service boundary: `network-service`
- runnable host process: `network-service`
- domain package: `internal/route/domain`
- handler package: `internal/route/transport/http`
- service package: `internal/route/service`

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
| `application_id` | `uuid.UUID` | required | user | 所属应用 ID |
| `environment_id` | `string` | required | user | 环境标识；当前实现要求传入有效环境 UUID 字符串 |
| `name` | `string` | required | user | 路由名 |
| `host` | `string` | required | user | 主机名匹配 |
| `path` | `string` | required | user | 路径匹配 |
| `service_name` | `string` | required | user | 目标服务名 |
| `service_port` | `int` | required | user | 目标服务端口 |

## API surface

Service-internal route surface:

- `POST /api/v1/routes`
- `GET /api/v1/routes?application_id=...&environment_id=...`
- `PATCH /api/v1/routes/{route_id}`
- `DELETE /api/v1/routes/{route_id}?application_id=...`
- `POST /api/v1/routes:validate`

Pre-production shared ingress external surface:

- `POST /api/v1/network/routes`
- `GET /api/v1/network/routes?application_id=...&environment_id=...`
- `PATCH /api/v1/network/routes/{route_id}`
- `DELETE /api/v1/network/routes/{route_id}?application_id=...`
- `POST /api/v1/network/routes:validate`

## Create / update rules

### Create
- required fields:
  - `application_id`
  - `environment_id`
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
  - `environment_id`, `name`, `host`, `path`, `service_name`, `service_port`
- immutable/system-managed fields:
  - `id`, `created_at`, `deleted_at`

### Delete
- supported as soft delete through the handler surface

## Validation notes

- invalid `application_id` query/body values or `route_id` path values return `invalid_argument`
- `GET /api/v1/routes` requires both `application_id` and `environment_id`
- `environment_id` is documented as a string because it is an identifier field, but the current implementation requires a valid environment UUID string
- missing records return `not_found`
- list endpoints support `name` filtering and `include_deleted`
- `POST /routes:validate` returns `valid` plus validation errors without persisting the route

## Source pointers

- module: `internal/route/module.go`
- domain: `internal/route/domain/route.go`
- service: `internal/route/service/route.go`
- handler: `internal/route/transport/http/handler.go`
