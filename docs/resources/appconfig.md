# AppConfig

## Ownership

- active service boundary: `config-service`
- runnable host process: `config-service`
- domain package: `internal/appconfig/domain`
- handler package: `internal/appconfig/transport/http`
- service package: `internal/appconfig/service`

## Purpose

`AppConfig` stores application-scoped configuration material for a specific environment.
It also tracks repo-backed config sync state, rendered configmap output, and the latest synced revision.

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
| `environment_id` | `string` | required | user | 环境标识 |
| `name` | `string` | required | user | 配置名 |
| `description` | `string` | optional | user | 配置描述 |
| `format` | `string` | optional | user | 配置格式 |
| `data` | `string` | optional | user | 原始配置内容 |
| `mount_path` | `string` | optional | user | 挂载路径 |
| `labels` | `[]LabelItem` | optional | user | 扩展标签 |
| `source_path` | `string` | optional | user | 配置仓库源路径 |
| `latest_revision_no` | `int` | system-managed | no | 最新修订号 |
| `latest_revision_id` | `*uuid.UUID` | system-managed | no | 最新修订记录 ID |
| `files` | `[]File` | system-managed | no | 渲染文件集合 |
| `rendered_configmap` | `RenderedConfigMap` | system-managed | no | 渲染后的 configmap 数据 |
| `source_commit` | `string` | system-managed | no | 最近一次同步来源 commit |

## API surface

- `POST /api/v1/app-configs`
- `GET /api/v1/app-configs`
- `GET /api/v1/app-configs/{id}`
- `PUT /api/v1/app-configs/{id}`
- `DELETE /api/v1/app-configs/{id}`
- `POST /api/v1/app-configs/{id}/sync-from-repo`

## Create / update rules

### Create
- required fields:
  - `application_id`
  - `environment_id`
  - `name`
- server-managed fields:
  - `id`
  - `created_at`
  - `updated_at`
  - revision and rendered output fields

### Update
- mutable fields:
  - `application_id`, `environment_id`, `name`, `description`, `format`, `data`, `mount_path`, `labels`, `source_path`
- immutable/system-managed fields:
  - `id`, `created_at`, `deleted_at`, revision and rendered output fields

### Delete
- supported as soft delete through the handler surface

## Validation notes

- invalid UUID path or query parameters return `invalid_argument`
- missing records return `not_found`
- repo sync failures map to `failed_precondition`
- list endpoints support `application_id`, `environment_id`, `name`, and `include_deleted`
- sync output returns the created or refreshed `AppConfigRevision`

## Source pointers

- module: `internal/appconfig/module.go`
- domain: `internal/appconfig/domain/app_config.go`
- service: `internal/appconfig/service/app_config.go`
- handler: `internal/appconfig/transport/http/handler.go`
