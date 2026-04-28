# Cluster

## Ownership

- active service: `meta-service`
- domain package: `internal/cluster/domain`
- handler package: `internal/cluster/transport/http`
- service package: `internal/cluster/service`

## Purpose

`Cluster` stores Kubernetes target metadata and onboarding status used by the current `meta-service` runtime.
It includes server address, kubeconfig material, Argo CD naming data, and onboarding readiness fields.

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
| `name` | `string` | required | user | 集群显示名 |
| `server` | `string` | required | user | Kubernetes API server 地址 |
| `kubeconfig` | `string` | required | user/secret | 集群连接配置；敏感字段 |
| `argocd_cluster_name` | `string` | optional | user | Argo CD cluster 标识 |
| `description` | `string` | optional | user | 集群描述 |
| `labels` | `[]LabelItem` | optional | user | 扩展标签 |
| `onboarding_ready` | `bool` | system-managed | no | onboarding 是否完成 |
| `onboarding_error` | `string` | system-managed | no | 最近一次 onboarding 错误 |
| `onboarding_checked_at` | `*time.Time` | system-managed | no | 最近一次 onboarding 检查时间 |

## API surface

Service-internal route surface:

- `POST /api/v1/clusters`
- `GET /api/v1/clusters`
- `GET /api/v1/clusters/{id}`
- `PUT /api/v1/clusters/{id}`
- `DELETE /api/v1/clusters/{id}`

Pre-production shared ingress external surface:

- `POST /api/v1/meta/clusters`
- `GET /api/v1/meta/clusters`
- `GET /api/v1/meta/clusters/{id}`
- `PUT /api/v1/meta/clusters/{id}`
- `DELETE /api/v1/meta/clusters/{id}`

## Create / update rules

### Create
- required fields:
  - `name`
  - `server`
  - `kubeconfig`
- server-managed fields:
  - `id`
  - `created_at`
  - `updated_at`
  - onboarding status fields

### Update
- mutable fields:
  - `name`, `server`, `kubeconfig`, `argocd_cluster_name`, `description`, `labels`
- immutable/system-managed fields:
  - `id`, `created_at`, `deleted_at`, onboarding status fields

### Delete
- supported as soft delete through the handler surface

## Validation notes

- `name`, `server`, and `kubeconfig` must not be empty
- duplicate clusters return `conflict`
- onboarding timeout maps to `deadline_exceeded`
- onboarding malformed input maps to `invalid_argument`
- list endpoints support `name` and `include_deleted`

## Source pointers

- module: `internal/cluster/module.go`
- domain: `internal/cluster/domain/cluster.go`
- service: `internal/cluster/service/service.go`
- handler: `internal/cluster/transport/http/handler.go`
