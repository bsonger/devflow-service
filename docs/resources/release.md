# Release

## Ownership

- active service boundary: `release-service`
- domain package: `internal/release/domain`
- handler package: `internal/release/transport/http`
- service package: `internal/release/service`

## Purpose

`Release` tracks an actual deployment attempt derived from a packaged manifest artifact.
它除了记录发布执行状态，还负责冻结 rollout 阶段真正使用的 `app_config` 与 `route` 视图。

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
| `application_id` | `uuid.UUID` | system-managed | no | 关联应用 ID |
| `manifest_id` | `uuid.UUID` | required | user | 关联 manifest |
| `image_id` | `uuid.UUID` | system-managed | no | 关联 image |
| `environment_id` | `string` | required | user | 目标环境标识 |
| `routes_snapshot` | `[]ReleaseRoute` | system-managed | no | release 创建时冻结的路由快照 |
| `app_config_snapshot` | `ReleaseAppConfig` | system-managed | no | release 创建时冻结的应用配置快照 |
| `type` | `string` | optional | user | 发布动作类型 |
| `steps` | `[]ReleaseStep` | system-managed | no | 发布步骤 |
| `status` | `ReleaseStatus` | system-managed | no | 发布状态 |
| `external_ref` | `string` | system-managed | no | 外部系统引用 |

## Release status values

- `Pending`
- `Running`
- `Succeeded`
- `Failed`
- `RollingBack`
- `RolledBack`
- `Syncing`
- `SyncFailed`

## Step status values

- `Pending`
- `Running`
- `Succeeded`
- `Failed`

## Common type values

- `Install`
- `Upgrade`
- `Rollback`

## API surface

- `POST /api/v1/releases`
- `GET /api/v1/releases`
- `GET /api/v1/releases/{id}`
- `DELETE /api/v1/releases/{id}`
- `POST /api/v1/verify/argo/events`
- `POST /api/v1/verify/release/steps`

## Create / update rules

### Create
- required fields:
  - `manifest_id`
  - `environment_id`
- optional fields:
  - `type`
- server-managed fields:
  - `id`
  - `application_id`
  - `image_id`
  - `execution_intent_id`
  - `routes_snapshot`
  - `app_config_snapshot`
  - `steps`
  - `status`
  - `external_ref`

### Writeback
- public writeback routes are observer-oriented update surfaces
- Argo events update release-level status
- release step events update step progress and step status
- both routes are token-protected and do not create releases

### Delete
- supported as soft delete through the handler surface

## Validation notes

- invalid UUID path or query parameters return `invalid_argument`
- missing records return `not_found`
- create-time runtime or deploy-target readiness problems return `failed_precondition`
- release 创建时会直接读取当前 config-service / network-service 数据，并冻结为自己的 `app_config_snapshot` 与 `routes_snapshot`
- release rollout 解析目标环境时只使用 release 自身的 `environment_id`
- 当 OCI manifest artifact 不可用时，repo plugin fallback 也应以 `release-id` 为主输入，而不是继续依赖旧的 `manifest-id` 语义
- list endpoints support `application_id`, `manifest_id`, `image_id`, `status`, `type`, and `include_deleted`

## Source pointers

- module: `internal/release/module.go`
- domain: `internal/release/domain/release.go`
- types: `internal/release/domain/types.go`
- service: `internal/release/service/release.go`
- handler: `internal/release/transport/http/release_handler.go`
- writeback: `internal/release/transport/http/release_writeback.go`
