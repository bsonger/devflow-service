# Release

## Ownership

- active service boundary: `release-service`
- domain package: `internal/release/domain`
- handler package: `internal/release/transport/http`
- service package: `internal/release/service`

## Purpose

`Release` tracks an actual deployment attempt derived from a frozen manifest.
It records the application, image, target environment, release type, execution steps, current status, and any external runtime or orchestrator reference used during rollout.

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
| `env` | `string` | optional | user | 目标环境名 |
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
- optional fields:
  - `env`
  - `type`
- server-managed fields:
  - `id`
  - `application_id`
  - `image_id`
  - `execution_intent_id`
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
- list endpoints support `application_id`, `manifest_id`, `image_id`, `status`, `type`, and `include_deleted`

## Source pointers

- module: `internal/release/module.go`
- domain: `internal/release/domain/release.go`
- types: `internal/release/domain/types.go`
- service: `internal/release/service/release.go`
- handler: `internal/release/transport/http/release_handler.go`
- writeback: `internal/release/transport/http/release_writeback.go`
