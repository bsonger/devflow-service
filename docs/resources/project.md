# Project

## Ownership

- active service boundary: `meta-service`
- runnable host process: `meta-service`
- domain package: `internal/project/domain`
- handler package: `internal/project/transport/http`
- service package: `internal/project/service`

## Purpose

`Project` is the top-level grouping resource for applications.
It stores project metadata and exposes a related applications listing under the same service boundary.

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
| `name` | `string` | expected on create | user | 项目名 |
| `description` | `string` | optional | user | 项目描述 |
| `labels` | `[]LabelItem` | optional | user | 扩展标签 |

## API surface

Service-internal route surface:

- `POST /api/v1/projects`
- `GET /api/v1/projects`
- `GET /api/v1/projects/{id}`
- `PUT /api/v1/projects/{id}`
- `DELETE /api/v1/projects/{id}`
- `GET /api/v1/projects/{id}/applications`

Pre-production shared ingress external surface:

- `POST /api/v1/meta/projects`
- `GET /api/v1/meta/projects`
- `GET /api/v1/meta/projects/{id}`
- `PUT /api/v1/meta/projects/{id}`
- `DELETE /api/v1/meta/projects/{id}`
- `GET /api/v1/meta/projects/{id}/applications`

Legacy external alias still routed in pre-production:

- `/api/v1/platform/projects`

## Create / update rules

### Create
- practical required fields:
  - `name`
- server-managed fields:
  - `id`
  - `created_at`
  - `updated_at`

### Update
- mutable fields:
  - `name`, `description`, `labels`
- immutable/system-managed fields:
  - `id`, `created_at`, `deleted_at`

### Delete
- supported as soft delete through the handler surface

## Validation notes

- id path parameters must be valid UUIDs
- project lookups return `not_found` when the record does not exist
- list endpoints support `name` and `include_deleted`
- the related applications endpoint returns applications currently associated with the project

## Source pointers

- module: `internal/project/module.go`
- domain: `internal/project/domain/project.go`
- service: `internal/project/service/service.go`
- handler: `internal/project/transport/http/handler.go`
