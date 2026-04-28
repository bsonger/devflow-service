# RuntimeSpec

## Ownership

- active service boundary: `runtime-service`
- runnable host process: `runtime-service`
- domain package: `internal/runtime/domain`
- handler package: `internal/runtime/transport/http`
- service package: `internal/runtime/service`

## Purpose

This runtime surface is primarily the application runtime inspection and operation surface.
From a product and operator point of view, its main jobs are:

- list the live pod status for one `application + environment`
- delete a specific pod for that application runtime
- trigger a rollout-style restart for the application's deployment

`RuntimeSpec` is the lookup anchor for one `application + environment` target.
`RuntimeSpecRevision` stores immutable desired-state snapshots under that runtime target.
`RuntimeObservedPod` stores the live observed pod state used by the pod-status view.
`RuntimeOperation` stores the audit trail for direct runtime actions such as pod deletion and deployment restart.

So although the storage model contains `RuntimeSpec`, `RuntimeSpecRevision`, `RuntimeObservedPod`, and `RuntimeOperation`, the primary external use case is still: inspect app pods, delete pod, and restart rollout.

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
| `health_thresholds` | `string` | 健康阈值 JSON 字符串，入库列为 `health_thresholds_jsonb` |
| `resources` | `string` | 资源配置 JSON 字符串，入库列为 `resources_jsonb` |
| `autoscaling` | `string` | 自动扩缩容 JSON 字符串，入库列为 `autoscaling_jsonb` |
| `scheduling` | `string` | 调度配置 JSON 字符串，入库列为 `scheduling_jsonb` |
| `pod_envs` | `string` | Pod 环境变量 JSON 字符串，入库列为 `pod_envs_jsonb` |
| `created_by` | `string` | 创建人 |
| `created_at` | `time.Time` | 创建时间 |

### RuntimeObservedPod

| Field | Type | Description |
|---|---|---|
| `id` | `uuid.UUID` | 观测记录 ID |
| `runtime_spec_id` | `uuid.UUID` | 所属运行时规格 ID |
| `application_id` | `uuid.UUID` | 所属应用 ID |
| `environment` | `string` | 环境键 |
| `namespace` | `string` | 运行时派生 namespace，不是自由写入字段 |
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

### RuntimeObservedPodContainer

| Field | Type | Description |
|---|---|---|
| `name` | `string` | 容器名 |
| `image` | `string` | 容器镜像引用 |
| `image_id` | `string` | 容器运行时上报的镜像 ID / digest |
| `ready` | `bool` | 容器 ready 状态 |
| `restart_count` | `int` | 容器重启次数 |
| `state` | `string` | 容器状态摘要 |

### RuntimeOperation

| Field | Type | Description |
|---|---|---|
| `id` | `uuid.UUID` | 操作记录 ID |
| `runtime_spec_id` | `uuid.UUID` | 所属运行时规格 ID |
| `operation_type` | `string` | 操作类型：`pod_delete` / `deployment_restart` |
| `target_name` | `string` | 目标资源名称（Pod 或 Deployment） |
| `operator` | `string` | 操作人 |
| `created_at` | `time.Time` | 操作时间 |

## Main operator flows

### 1. List application pod status

Primary read flow:

1. resolve the target runtime by `application_id + environment`
2. read the runtime spec ID
3. call `GET /runtime-specs/{id}/pods`
4. render pod phase, ready state, restart count, node, IP, and container status

This is the main pod-status view for one application runtime.

### 2. Delete one pod

Primary action flow:

1. identify the target runtime spec
2. call `POST /runtime-specs/{id}/pods/{pod_name}/delete`
3. runtime-service deletes the pod through the Kubernetes API
4. runtime-service writes one `RuntimeOperation` record with `operation_type = pod_delete`

This action is for one concrete pod, not for the whole application rollout.

### 3. Trigger rollout / restart

Primary action flow:

1. identify the target runtime spec
2. call `POST /runtime-specs/{id}/deployments/{deployment_name}/restart`
3. runtime-service patches the Deployment with `kubectl.kubernetes.io/restartedAt`
4. Kubernetes performs the rolling restart
5. runtime-service writes one `RuntimeOperation` record with `operation_type = deployment_restart`

Current implementation note:

- the code path is a Deployment restart, not a separate rollout resource
- in product language this can be described as triggering a rollout or restart for the application workload

## API surface

Service-internal route surface:

- `POST /api/v1/runtime-specs`
- `GET /api/v1/runtime-specs`
- `GET /api/v1/runtime-specs/{id}`
- `DELETE /api/v1/runtime-specs`
- `POST /api/v1/runtime-specs/{id}/revisions`
- `GET /api/v1/runtime-specs/{id}/revisions`
- `GET /api/v1/runtime-spec-revisions/{id}`
- `GET /api/v1/runtime-specs/{id}/pods`
- `POST /api/v1/runtime-specs/{id}/pods/{pod_name}/delete`
- `POST /api/v1/runtime-specs/{id}/deployments/{deployment_name}/restart`
- `GET /api/v1/runtime-specs/{id}/operations`
- `POST /api/v1/internal/runtime-spec-pods/sync`
- `POST /api/v1/internal/runtime-spec-pods/delete`

Pre-production shared ingress external surface:

- `POST /api/v1/runtime/runtime-specs`
- `GET /api/v1/runtime/runtime-specs`
- `GET /api/v1/runtime/runtime-specs/{id}`
- `DELETE /api/v1/runtime/runtime-specs`
- `POST /api/v1/runtime/runtime-specs/{id}/revisions`
- `GET /api/v1/runtime/runtime-specs/{id}/revisions`
- `GET /api/v1/runtime/runtime-spec-revisions/{id}`
- `GET /api/v1/runtime/runtime-specs/{id}/pods`
- `POST /api/v1/runtime/runtime-specs/{id}/pods/{pod_name}/delete`
- `POST /api/v1/runtime/runtime-specs/{id}/deployments/{deployment_name}/restart`
- `GET /api/v1/runtime/runtime-specs/{id}/operations`

Internal observer-only endpoints are not exposed through the shared ingress:

- `POST /api/v1/internal/runtime-spec-pods/sync`
- `POST /api/v1/internal/runtime-spec-pods/delete`

## Create / update rules

### Create

`POST /runtime-specs` request body:

```json
{
  "application_id": "999c0c88-1f1f-41d1-a67a-8159d07c878c",
  "environment": "staging"
}
```

Rules:

- one runtime spec is created for one `application_id + environment`
- duplicate active pairs return `conflict`
- `environment` is trimmed before persistence
- create only creates the top-level runtime spec; it does not create the first revision automatically

### Update

- there is no general-purpose top-level `PUT /runtime-specs/{id}` surface today
- desired-state mutation happens by creating a new revision
- observed pod state is written by internal observer sync/delete flows
- each new revision updates `current_revision_id` on the parent `RuntimeSpec`

`POST /runtime-specs/{id}/revisions` request body:

```json
{
  "replicas": 1,
  "health_thresholds": "{}",
  "resources": "{}",
  "autoscaling": "{}",
  "scheduling": "{}",
  "pod_envs": "[]",
  "created_by": "runtime-operator"
}
```

Current implementation note:

- revision payload fields are accepted as JSON-encoded strings, not nested JSON objects
- repository write path normalizes empty values to JSON defaults such as `{}` or `[]` before inserting JSONB columns

### Delete

- runtime spec delete is keyed by `application_id + environment` in the request body, not by runtime-spec ID path
- delete removes the top-level spec record through the current repository path
- pod delete and deployment restart are explicit runtime actions and also create `RuntimeOperation` records

Action request bodies:

```json
{
  "operator": "alice"
}
```

## Namespace derivation

The runtime namespace is system-derived from `application_id` and `environment`:

- if `environment` is empty or `production`, namespace = `{application_id}`
- otherwise, namespace = `{application_id}-{strings.ToLower(environment)}`

Examples:

- production: `999c0c88-1f1f-41d1-a67a-8159d07c878c`
- staging: `999c0c88-1f1f-41d1-a67a-8159d07c878c-staging`

Observer sync/delete requests may include `namespace`, but if provided it must match this derived value exactly.

## Observer writeback behavior

`POST /api/v1/internal/runtime-spec-pods/sync`:

- looks up the owning runtime spec by `application_id + environment`
- derives the canonical namespace from the runtime spec
- upserts one live pod record keyed by `(runtime_spec_id, namespace, pod_name)`
- if `observed_at` is omitted, runtime-service fills `time.Now().UTC()`
- current handler returns `204 no content`

`POST /api/v1/internal/runtime-spec-pods/delete`:

- looks up the owning runtime spec by `application_id + environment`
- derives the canonical namespace from the runtime spec
- soft-deletes the matching observed pod record by setting `deleted_at`
- if `observed_at` is omitted, runtime-service fills `time.Now().UTC()`
- current handler returns `204 no content`

## Read behavior

- `GET /runtime-specs` currently returns all runtime specs; there is no handler-level business filter contract yet
- the main runtime inspection read is `GET /runtime-specs/{id}/pods`
- `GET /runtime-specs/{id}/pods` returns only non-deleted observed pods, ordered by `observed_at desc, pod_name asc`
- `GET /runtime-specs/{id}/pods` is the primary backing API for the application pod-status view
- `GET /runtime-specs/{id}/revisions` returns revisions ordered by `revision desc`
- `GET /runtime-specs/{id}/operations` returns operation history ordered by `created_at desc`

## Validation notes

- invalid UUID path parameters return `invalid_argument`
- create/delete/sync flows require non-empty `application_id` and `environment`
- observed pod sync also requires non-empty `pod_name` and `phase`
- missing records return `not_found`
- internal observed-pod sync/delete endpoints require `X-Devflow-Observer-Token` or `X-Devflow-Verify-Token` when a shared token is configured
- observer payload namespace must match the runtime-service derived namespace for the target `application + environment`
- `containers[].image_id` is the container runtime reported image ID / digest, not the removed manifest/release `image_id` field
- release-time callers may use runtime lookup endpoints, but the main operator-facing read surface is the pod-status view for one application runtime
- Kubernetes operations require the runtime-service pod to have in-cluster client access
- Kubernetes `not found` maps to `not_found`; Kubernetes `forbidden` and in-cluster client init failures map to `failed_precondition`
- current revision create path does not yet enforce rich semantic validation on `replicas`, JSON schema, or `created_by`; it primarily validates parent runtime-spec existence

## Source pointers

- module: `internal/runtime/module.go`
- domain: `internal/runtime/domain/runtime_spec.go`
- repository: `internal/runtime/repository/repository.go`
- service: `internal/runtime/service/service.go`
- handler: `internal/runtime/transport/http/handler.go`
- router: `internal/runtime/transport/http/router.go`
