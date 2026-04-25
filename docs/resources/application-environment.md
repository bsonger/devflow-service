# Application Environment Binding

## Ownership

- active service: `meta-service`
- domain package: `internal/applicationenv/domain`
- handler package: `internal/applicationenv/transport/http`
- service package: `internal/applicationenv/service`

## Purpose

`Application Environment Binding` is the only database-backed application-environment relation migrated into `meta-service` from the old orchestrator flow.

It binds one `Application` to one `Environment`.
It does **not** create separate binding tables for `AppConfig` or `WorkloadConfig`.
Those two resources continue to use their own tables and are resolved by `application_id + environment_id`, with `environment_id = "base"` used as fallback.

## Field table

| Field | Type | Required | Writable | Description |
|---|---|---|---|---|
| `id` | `uuid.UUID` | server-generated | no | 绑定记录 ID，对应数据库列 `binding_id` |
| `application_id` | `uuid.UUID` | required | path/system | 关联应用 ID |
| `environment_id` | `string` | required | user | 环境标识，当前实现要求传入有效环境 UUID |
| `created_at` | `time.Time` | server-generated | no | 创建时间 |
| `updated_at` | `time.Time` | server-generated | no | 更新时间 |
| `deleted_at` | `*time.Time` | optional | system-managed | 软删除时间 |

## API surface

- `GET /api/v1/applications/{id}/environments`
- `POST /api/v1/applications/{id}/environments`
- `GET /api/v1/applications/{id}/environments/{environment_id}`
- `DELETE /api/v1/applications/{id}/environments/{environment_id}`

Compatibility read route kept for existing release/downstream callers:

- `GET /api/v1/platform/applications/{id}/environments/{environment_id}`

## Behavior

### Attach

- validates `Application` exists
- validates `Environment` exists
- upserts on `(application_id, environment_id)`
- if the binding was previously soft-deleted, attach restores it

Request body:

```json
{
  "environment_id": "8d3d8f55-0f0c-4d85-9ef6-9e19d31f9dbe"
}
```

### List

- returns all non-deleted bindings for one application
- enriches each item with the resolved `Environment`

### Detail

- returns the binding itself
- returns the resolved `Environment`
- returns `app_configs`
- returns `workload_configs`

Config resolution order:

1. exact match on `application_id + environment_id`
2. if empty, fallback to `application_id + "base"`

### Delete

- soft-deletes the binding row
- does not delete `AppConfig` or `WorkloadConfig`

## Storage expectation

Current code expects a table named:

```sql
application_environment_bindings
```

Minimal expected shape:

```sql
create table if not exists application_environment_bindings (
  binding_id uuid primary key,
  application_id uuid not null,
  environment_id text not null,
  created_at timestamptz not null,
  updated_at timestamptz not null,
  deleted_at timestamptz null,
  unique (application_id, environment_id)
);
```

Recommended index for list queries:

```sql
create index if not exists idx_application_environment_bindings_application_id
  on application_environment_bindings (application_id);
```

## Source pointers

- module: `internal/applicationenv/module.go`
- domain: `internal/applicationenv/domain/binding.go`
- service: `internal/applicationenv/service/service.go`
- repository: `internal/applicationenv/repository/repository.go`
- handler: `internal/applicationenv/transport/http/handler.go`
