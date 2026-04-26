# Manifest

## Ownership

- active service boundary: `release-service`
- domain package: `internal/manifest/domain`
- handler package: `internal/manifest/transport/http`
- service package: `internal/manifest/service`

## Purpose

`Manifest` is the packaging-time deployment bundle built from image + workload/service inputs.
它负责产出渲染后的 YAML、OCI 制品上传信息，以及 rollout 需要消费的打包结果。

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
| `image_id` | `uuid.UUID` | required | user | 关联镜像 ID |
| `image_ref` | `string` | system-managed | no | 最终镜像引用 |
| `artifact_repository` | `string` | system-managed | no | 制品仓库 |
| `artifact_tag` | `string` | system-managed | no | 制品标签 |
| `artifact_ref` | `string` | system-managed | no | 制品完整引用 |
| `artifact_digest` | `string` | system-managed | no | 制品 digest |
| `artifact_media_type` | `string` | system-managed | no | 制品媒体类型 |
| `artifact_pushed_at` | `*time.Time` | system-managed | no | 制品推送时间 |
| `services_snapshot` | `[]ManifestService` | system-managed | no | 冻结服务快照 |
| `workload_config_snapshot` | `ManifestWorkloadConfig` | system-managed | no | 冻结工作负载配置快照 |
| `rendered_objects` | `[]ManifestRenderedObject` | system-managed | no | 渲染对象列表 |
| `rendered_yaml` | `string` | system-managed | no | 聚合 YAML |
| `status` | `ManifestStatus` | system-managed | no | Manifest 状态 |

## Status values

- `Pending`
- `Ready`
- `Failed`

## API surface

- `POST /api/v1/manifests`
- `GET /api/v1/manifests`
- `GET /api/v1/manifests/{id}`
- `GET /api/v1/manifests/{id}/resources`
- `DELETE /api/v1/manifests/{id}`

## Create / update rules

### Create
- required fields:
  - `application_id`
  - `image_id`
- server-managed fields:
  - `id`
  - artifact metadata
  - snapshots
  - rendered outputs
  - `status`

### Delete
- supported as soft delete through the handler surface

## Validation notes

- invalid UUID path or query parameters return `invalid_argument`
- missing records return `not_found`
- create-time dependency mismatches or missing deploy-target inputs return `failed_precondition`
- list endpoints support `application_id`, `image_id`, and `include_deleted`
- `GET /resources` returns grouped frozen resources and the rendered object view without mutating the manifest
- 当前边界上，manifest 主责是：
  - 镜像引用解析
  - `services_snapshot` / `workload_config_snapshot`
  - YAML 渲染
  - YAML 打包 OCI 并上传

## Source pointers

- module: `internal/manifest/module.go`
- domain: `internal/manifest/domain/manifest.go`
- service: `internal/manifest/service/manifest.go`
- handler: `internal/manifest/transport/http/manifest_handler.go`
