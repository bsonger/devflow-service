# RuntimeSpec

## Ownership

- active service boundary: `runtime-service`
- runnable host process: `runtime-service`
- domain package: `internal/runtime/domain`
- handler package: `internal/runtime/transport/http`
- service package: `internal/runtime/service`

## Purpose

`RuntimeSpec` is the runtime desired-state record for one `application + environment` target.
`RuntimeSpecRevision` is the immutable runtime snapshot stored under a `RuntimeSpec`.
`RuntimeObservedPod` is the live observed runtime pod snapshot keyed back to the owning spec.

This extracted runtime-service surface now covers both release-time runtime lookup and observer-fed live runtime visibility.

## Current API surface

- `POST /api/v1/runtime-specs`
- `GET /api/v1/runtime-specs`
- `GET /api/v1/runtime-specs/{id}`
- `DELETE /api/v1/runtime-specs?application_id=...&environment=...`
- `POST /api/v1/runtime-specs/{id}/revisions`
- `GET /api/v1/runtime-specs/{id}/revisions`
- `GET /api/v1/runtime-spec-revisions/{id}`
- `GET /api/v1/runtime-specs/{id}/pods`
- `POST /api/v1/internal/runtime-spec-pods/sync`
- `POST /api/v1/internal/runtime-spec-pods/delete`

## Field table

### RuntimeSpec

| Field | Type | Description |
|---|---|---|
| `id` | `uuid.UUID` | 运行时规格 ID |
| `application_id` | `uuid.UUID` | 所属应用 ID |
| `environment` | `string` | 目标环境键 |
| `current_revision_id` | `*uuid.UUID` | 当前生效修订 |
| `created_at` | `time.Time` | 创建时间 |
| `updated_at` | `time.Time` | 更新时间 |

### RuntimeSpecRevision

| Field | Type | Description |
|---|---|---|
| `id` | `uuid.UUID` | 运行时规格修订 ID |
| `runtime_spec_id` | `uuid.UUID` | 所属运行时规格 ID |
| `revision` | `int` | 自增修订号 |
| `replicas` | `int` | 期望副本数 |
| `health_thresholds` | `string` | 健康阈值 JSON |
| `resources` | `string` | 资源配置 JSON |
| `autoscaling` | `string` | 自动扩缩容 JSON |
| `scheduling` | `string` | 调度配置 JSON |
| `pod_envs` | `string` | Pod 环境变量 JSON |
| `created_by` | `string` | 创建人 |
| `created_at` | `time.Time` | 创建时间 |

### RuntimeObservedPod

| Field | Type | Description |
|---|---|---|
| `id` | `uuid.UUID` | 观测记录 ID |
| `runtime_spec_id` | `uuid.UUID` | 所属运行时规格 ID |
| `application_id` | `uuid.UUID` | 所属应用 ID |
| `environment` | `string` | 环境键 |
| `namespace` | `string` | 派生出的运行时命名空间 |
| `pod_name` | `string` | Pod 名称 |
| `phase` | `string` | Pod phase |
| `ready` | `bool` | Pod ready 状态 |
| `restarts` | `int` | 重启次数 |
| `node_name` | `string` | 节点名 |
| `pod_ip` | `string` | Pod IP |
| `host_ip` | `string` | Host IP |
| `owner_kind` | `string` | 所属控制器类型 |
| `owner_name` | `string` | 所属控制器名称 |
| `labels` | `map[string]string` | Pod labels |
| `containers` | `[]RuntimeObservedPodContainer` | 容器快照 |
| `observed_at` | `time.Time` | 观测时间 |
| `deleted_at` | `*time.Time` | 软删除时间 |

## Validation notes

- invalid UUID path parameters return `invalid_argument`
- missing records return `not_found`
- duplicate runtime spec creation returns `conflict`
- internal observed-pod sync/delete endpoints require `X-Devflow-Observer-Token` or `X-Devflow-Verify-Token` when a shared token is configured
- observer payload namespace must match the runtime-service derived namespace for the target `application + environment`
- release-time callers use the runtime lookup endpoints to validate `Image.runtime_spec_revision_id`

## Source pointers

- domain: `internal/runtime/domain/runtime_spec.go`
- repository: `internal/runtime/repository/repository.go`
- service: `internal/runtime/service/service.go`
- handler: `internal/runtime/transport/http/handler.go`
- router: `internal/runtime/transport/http/router.go`
