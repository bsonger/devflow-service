# Application

## Ownership

- active service: `meta-service`
- domain package: `internal/application/domain`
- handler package: `internal/application/transport/http`
- service package: `internal/application/service`

## Purpose

`Application` is the app metadata resource in `meta-service`.
It stores the project reference, repository address, descriptive fields, labels, and the current `active_image` binding.

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
| `active_image_id` | `*uuid.UUID` | optional | user/system | 当前绑定 image |
| `labels` | `[]LabelItem` | optional | user | 扩展标签 |

## API surface

- `POST /api/v1/applications`
- `GET /api/v1/applications`
- `GET /api/v1/applications/{id}`
- `PUT /api/v1/applications/{id}`
- `DELETE /api/v1/applications/{id}`
- `PATCH /api/v1/applications/{id}/active_image`

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
  - `project_id`, `name`, `repo_address`, `description`, `active_image_id`, `labels`
- immutable/system-managed fields:
  - `id`, `created_at`, `deleted_at`

### Delete
- supported as soft delete through the handler surface

## Validation notes

- `project_id` must reference an existing `Project`
- `image_id` in the patch request must be a valid UUID
- missing or invalid UUID path parameters return `invalid_argument`
- list endpoints support `project_id`, `name`, `repo_address`, and `include_deleted`

## Source pointers

- module: `internal/application/module.go`
- domain: `internal/application/domain/application.go`
- service: `internal/application/service/service.go`
- handler: `internal/application/transport/http/handler.go`
