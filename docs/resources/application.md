# Application

## Ownership

- active service boundary: `meta-service`
- runnable host process: `meta-service`
- domain package: `internal/application/domain`
- handler package: `internal/application/transport/http`
- service package: `internal/application/service`

## Purpose

`Application` is the app metadata resource in `meta-service`.
It stores the project reference, repository address, descriptive fields, and labels.

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
| `project_id` | `uuid.UUID` | required | user | 关联项目 ID |
| `name` | `string` | required | user | 应用名 |
| `repo_address` | `string` | required | user | 代码仓库地址 |
| `description` | `string` | optional | user | 应用描述 |
| `labels` | `[]LabelItem` | optional | user | 扩展标签 |

## API surface

Service-internal route surface:

- `POST /api/v1/applications`
- `GET /api/v1/applications`
- `GET /api/v1/applications/{id}`
- `PUT /api/v1/applications/{id}`
- `DELETE /api/v1/applications/{id}`
- `GET /api/v1/applications/{id}/environments`
- `POST /api/v1/applications/{id}/environments`
- `GET /api/v1/applications/{id}/environments/{environment_id}`
- `DELETE /api/v1/applications/{id}/environments/{environment_id}`

Pre-production shared ingress external surface:

- `POST /api/v1/meta/applications`
- `GET /api/v1/meta/applications`
- `GET /api/v1/meta/applications/{id}`
- `PUT /api/v1/meta/applications/{id}`
- `DELETE /api/v1/meta/applications/{id}`
- `GET /api/v1/meta/applications/{id}/environments`
- `POST /api/v1/meta/applications/{id}/environments`
- `GET /api/v1/meta/applications/{id}/environments/{environment_id}`
- `DELETE /api/v1/meta/applications/{id}/environments/{environment_id}`

## Create / update rules

### Create
- required fields:
  - `project_id`
  - `name`
  - `repo_address`
- server-managed fields:
  - `id`
  - `created_at`
  - `updated_at`

### Update
- mutable fields:
  - `project_id`, `name`, `repo_address`, `description`, `labels`
- immutable/system-managed fields:
  - `id`, `created_at`, `deleted_at`

### Delete
- supported as soft delete through the handler surface

## Validation notes

- `project_id` must reference an existing `Project`
- missing or invalid UUID path parameters return `invalid_argument`
- list endpoints support `project_id`, `name`, `repo_address`, and `include_deleted`

## Source pointers

- module: `internal/application/module.go`
- domain: `internal/application/domain/application.go`
- service: `internal/application/service/service.go`
- handler: `internal/application/transport/http/handler.go`

For the application-environment binding sub-resource, see:

- `docs/resources/application-environment.md`

This file lists the binding endpoints because they hang off the `Application` route tree, but binding semantics, validation, and storage rules are owned by `application-environment.md`.
