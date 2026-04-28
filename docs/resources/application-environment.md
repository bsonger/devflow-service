# Application Environment Binding

## Ownership

- active service boundary: `meta-service`
- runnable host process: `meta-service`
- domain package: `internal/applicationenv/domain`
- handler package: `internal/applicationenv/transport/http`
- service package: `internal/applicationenv/service`

## Purpose

`Application Environment Binding` is the database-backed relation between one `Application` and one `Environment`.
It is the only migrated binding resource in the current `meta-service` contract.
It does **not** replace `AppConfig` or `WorkloadConfig` storage:

- `AppConfig` resolves by exact `application_id + environment_id`
- `WorkloadConfig` remains application-scoped

## Common base fields

| Field | Type | Required | Writable | Description |
|---|---|---|---|---|
| `id` | `uuid.UUID` | server-generated | no | 绑定记录 ID，对应数据库列 `binding_id` |
| `created_at` | `time.Time` | server-generated | no | 创建时间 |
| `updated_at` | `time.Time` | server-generated | no | 更新时间 |
| `deleted_at` | `*time.Time` | optional | system-managed | 软删除时间 |

## Field table

| Field | Type | Required | Writable | Description |
|---|---|---|---|---|
| `application_id` | `uuid.UUID` | required | path/system | 关联应用 ID |
| `environment_id` | `string` | required | user | 环境标识；当前实现要求传入有效环境 UUID |

## API surface

Service-internal route surface:

- `GET /api/v1/applications/{id}/environments`
- `POST /api/v1/applications/{id}/environments`
- `GET /api/v1/applications/{id}/environments/{environment_id}`
- `DELETE /api/v1/applications/{id}/environments/{environment_id}`

Pre-production shared ingress external surface:

- `GET /api/v1/meta/applications/{id}/environments`
- `POST /api/v1/meta/applications/{id}/environments`
- `GET /api/v1/meta/applications/{id}/environments/{environment_id}`
- `DELETE /api/v1/meta/applications/{id}/environments/{environment_id}`

## Create / update rules

### Create / attach

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

### Read

- list returns all non-deleted bindings for one application and enriches each item with resolved `Environment`
- detail returns the binding, resolved `Environment`, `app_configs`, and `workload_configs`
- config resolution order on detail:
  1. `AppConfig`: exact `application_id + environment_id`
  2. `WorkloadConfig`: list by `application_id`

### Delete

- soft-deletes the binding row
- does not delete `AppConfig` or `WorkloadConfig`

## Validation notes

- invalid application or environment UUIDs return `invalid_argument`
- missing application, environment, or binding records return `not_found`
- current storage expects a unique active pair on `application_id + environment_id`
- current backing table is `application_environment_bindings`

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

Recommended list index:

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
