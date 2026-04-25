# Intent

## Ownership

- active service boundary: `release-service`
- domain package: `internal/intent/domain`
- handler package: `internal/intent/transport/http`
- service package: `internal/intent/service`

## Purpose

`Intent` represents an execution intent for asynchronous build or release work.
It tracks the resource being acted on, current status, claim and lease state, retry count, trace linkage, and the latest execution message or error.

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
| `kind` | `IntentKind` | system-managed | no | 意图类型 |
| `status` | `IntentStatus` | system-managed | no | 执行状态 |
| `resource_type` | `string` | system-managed | no | 关联资源类型 |
| `resource_id` | `uuid.UUID` | system-managed | no | 关联资源 ID |
| `trace_id` | `string` | system-managed | no | 链路追踪标识 |
| `message` | `string` | system-managed | no | 当前消息 |
| `last_error` | `string` | system-managed | no | 最近错误 |
| `claimed_by` | `string` | system-managed | no | 当前持有者 |
| `claimed_at` | `*time.Time` | system-managed | no | 开始持有时间 |
| `lease_expires_at` | `*time.Time` | system-managed | no | 租约过期时间 |
| `attempt_count` | `int` | system-managed | no | 重试次数 |

## Kind values

- `build`
- `release`

## Status values

- `Pending`
- `Running`
- `Succeeded`
- `Failed`

## API surface

- `GET /api/v1/intents`
- `GET /api/v1/intents/{id}`

## Read rules

- intents are currently query and inspection resources only on the public HTTP surface
- write-side claiming and status changes happen through internal execution flows, not these handlers
- delete is not exposed on the current public HTTP surface
- `include_deleted` is not currently supported on the list endpoint

## Validation notes

- invalid UUID path or query parameters return `invalid_argument`
- missing records return `not_found`
- list endpoints support `kind`, `status`, `resource_type`, `resource_id`, and `claimed_by`

## Source pointers

- module: `internal/intent/module.go`
- domain: `internal/intent/domain/intent.go`
- service: `internal/intent/service/intent.go`
- handler: `internal/intent/transport/http/intent_handler.go`
