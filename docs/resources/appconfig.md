# AppConfig

## Ownership

- active service boundary: `config-service`
- runnable host process: `config-service`
- domain package: `internal/appconfig/domain`
- handler package: `internal/appconfig/transport/http`
- service package: `internal/appconfig/service`

## Purpose

`AppConfig` stores the environment-scoped configuration snapshot for one application.
Its configuration source is synchronized from the fixed GitHub config repository.
The repository location is system-configured, while the effective repository directory is system-derived from project, application, and environment identity.
The resource tracks the mount directory plus the latest synced revision, synced files, source repository directory, and source commit.

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
| `environment_id` | `string` | required | user | 环境标识；当前实现要求传入有效环境 UUID 字符串 |
| `mount_path` | `string` | optional | user | 配置挂载目录，默认 `/etc/config` |
| `latest_revision_no` | `int` | system-managed | no | 最新修订号 |
| `latest_revision_id` | `*uuid.UUID` | system-managed | no | 最新修订记录 ID |
| `files` | `[]File` | system-managed | no | 最近一次同步得到的文件集合 |
| `source_directory` | `string` | system-managed | no | 最近一次同步对应的 GitHub 配置目录 |
| `source_commit` | `string` | system-managed | no | 最近一次同步来源 commit |

## File fields

| Field | Type | Required | Writable | Description |
|---|---|---|---|---|
| `name` | `string` | system-managed | no | 文件名 |
| `content` | `string` | system-managed | no | 文件内容 |

## API surface

Service-internal route surface:

- `POST /api/v1/app-configs`
- `GET /api/v1/app-configs?application_id=...&environment_id=...`
- `GET /api/v1/app-configs/{id}`
- `PUT /api/v1/app-configs/{id}`
- `DELETE /api/v1/app-configs/{id}`
- `POST /api/v1/app-configs/{id}/sync-from-repo`

Pre-production shared ingress external surface:

- `POST /api/v1/config/app-configs`
- `GET /api/v1/config/app-configs?application_id=...&environment_id=...`
- `GET /api/v1/config/app-configs/{id}`
- `PUT /api/v1/config/app-configs/{id}`
- `DELETE /api/v1/config/app-configs/{id}`
- `POST /api/v1/config/app-configs/{id}/sync-from-repo`

## Create / update rules

### Create
- required fields:
  - `application_id`
  - `environment_id`
- optional fields:
  - `mount_path`
- server-managed fields:
  - `id`
  - `created_at`
  - `updated_at`
  - revision and sync output fields

### Update
- mutable fields:
  - `mount_path`
- request-scoped required fields:
  - `application_id`
  - `environment_id`
- immutable/system-managed fields:
  - `id`, `created_at`, `deleted_at`, revision and sync output fields

### Delete
- supported as soft delete through the handler surface

## Sync behavior

- sync source is the fixed GitHub config repository
- repository URL is system-configured and is not stored as a user-writable field on `AppConfig`
- runtime config must provide `config_repo.root_dir` and may override `config_repo.default_ref`
- sync path is system-derived from:
  - `project_name`
  - `application_name`
  - `environment_name`
- the effective source shape is:

```text
{project_name}/{application_name}/{environment_name}
```

- all files found under that directory are synchronized into the latest revision
- sync updates:
  - `files`
  - `source_directory`
  - `source_commit`
  - `latest_revision_no`
  - `latest_revision_id`

## Validation notes

- invalid UUID path or query parameters return `invalid_argument`
- `GET /api/v1/app-configs` requires both `application_id` and `environment_id`
- `environment_id` is documented as a string because it is an identifier field, but the current implementation requires a valid environment UUID string
- `POST /api/v1/config/app-configs` on the shared ingress creates one record per unique `application_id + environment_id`; duplicate active pairs return `invalid_argument` from the current handler mapping
- `application_id + environment_id` must be unique among non-deleted records
- `mount_path` defaults to `/etc/config` when omitted
- `files`, `source_directory`, and `source_commit` are populated from the latest synced revision
- `source_directory` records the effective directory inside the GitHub config repository used by the latest sync
- rendered configmap output is not stored on `AppConfig`; release-time rendering owns that step
- live PostgreSQL cutover for this shape is tracked in `deployments/pre-production/database/appconfig-hard-cutover.sql`
- before the first successful sync, revision and sync output fields may be empty
- missing records return `not_found`
- repo sync failures map to `failed_precondition`

## Source pointers

- module: `internal/appconfig/module.go`
- domain: `internal/appconfig/domain/app_config.go`
- service: `internal/appconfig/service/app_config.go`
- handler: `internal/appconfig/transport/http/handler.go`
