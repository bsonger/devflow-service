# Image

## Ownership

- active service boundary: `release-service`
- domain package: `internal/image/domain`
- handler package: `internal/image/transport/http`
- service package: `internal/image/service`

## Purpose

`Image` tracks a buildable and deployable application image artifact.
It records the source application, optional config/runtime revision bindings, branch and repository metadata, build pipeline linkage, image digest, and step-level execution state.

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
| `execution_intent_id` | `*uuid.UUID` | system-managed | no | 关联执行意图 |
| `application_id` | `uuid.UUID` | required | user | 关联应用 ID |
| `configuration_revision_id` | `*uuid.UUID` | optional | user/system | 关联配置修订 |
| `runtime_spec_revision_id` | `*uuid.UUID` | optional | user/system | 关联运行时修订 |
| `name` | `string` | system-managed | no | 镜像名 |
| `tag` | `string` | patchable | user/system | 镜像标签 |
| `branch` | `string` | optional | user | 来源分支 |
| `repo_address` | `string` | system-managed | no | 来源仓库地址 |
| `commit_hash` | `string` | patchable | user/system | 构建提交 |
| `digest` | `string` | patchable | user/system | OCI digest |
| `pipeline_id` | `string` | system-managed | no | 外部流水线标识 |
| `steps` | `[]ImageTask` | system-managed | no | 构建步骤状态 |
| `status` | `ImageStatus` | system-managed | no | 镜像状态 |

## Status values

- `Pending`
- `Running`
- `Succeeded`
- `Failed`

## API surface

- `POST /api/v1/images`
- `GET /api/v1/images`
- `GET /api/v1/images/{id}`
- `PATCH /api/v1/images/{id}`

## Create / update rules

### Create
- required fields:
  - `application_id`
- optional fields:
  - `configuration_revision_id`
  - `runtime_spec_revision_id`
  - `branch`
- server-managed fields:
  - `id`
  - `name`
  - `repo_address`
  - `pipeline_id`
  - `steps`
  - `status`

### Patch
- mutable fields:
  - `commit_hash`
  - `digest`
  - `tag`
- immutable/system-managed fields:
  - `id`, `application_id`, revision bindings, `name`, `repo_address`, `pipeline_id`, `steps`, `status`

### Delete
- no public delete route is exposed today
- soft-deleted rows may still be visible to list callers through `include_deleted=true`

## Validation notes

- invalid UUID path or query parameters return `invalid_argument`
- missing records return `not_found`
- list endpoints support `application_id`, `pipeline_id`, `status`, `branch`, `name`, and `include_deleted`

## Source pointers

- module: `internal/image/module.go`
- domain: `internal/image/domain/image.go`
- service: `internal/image/service/image.go`
- handler: `internal/image/transport/http/image_handler.go`
- writeback: `internal/image/transport/http/image_writeback.go`
