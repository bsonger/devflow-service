# Manifest

## Ownership

- active service boundary: `release-service`
- runnable host process: `release-service`
- domain package: `internal/manifest/domain`
- handler package: `internal/manifest/transport/http`
- service package: `internal/manifest/service`

## Purpose

`Manifest` is the build-time snapshot and image-delivery record for one application revision.
It freezes service and workload snapshots, triggers the Tekton image build, records the image result, and remains the durable traceable record after `PipelineRun` / `TaskRun` resources are garbage-collected.

## Quick reader guide

Use this document when you need to answer build-side questions such as:

- what exactly was frozen before image build started
- which source revision was actually built
- what image was produced
- how Tekton progress maps back to one durable system record

If your question is instead about:

- target environment
- app config used for deployment
- rendered deployment YAML
- Argo CD deployment state
- published OCI deployment bundle

then the owning resource is `Release`, not `Manifest`.

## Common base fields

| Field | Type | Required | Writable | Description |
|---|---|---|---|---|
| `id` | `uuid.UUID` | server-generated | no | 主键 |
| `created_at` | `time.Time` | server-generated | no | 创建时间 |
| `updated_at` | `time.Time` | server-generated | no | 更新时间 |
| `deleted_at` | `*time.Time` | optional | system-managed | 软删除时间，内部实现字段 |

## Field table

| Field | Type | Required | Writable | Description |
|---|---|---|---|---|
| `application_id` | `uuid.UUID` | required | user | 关联应用 ID |
| `git_revision` | `string` | optional | user | 构建源码选择器，可传 branch、tag 或 commit；默认 `main` |
| `repo_address` | `string` | system-managed | no | 代码仓库地址 |
| `commit_hash` | `string` | system-managed | no | 本次构建绑定的源码提交 |
| `image_ref` | `string` | system-managed | no | 最终镜像完整引用，推荐使用 digest 形式；这是 workload 真正部署时使用的镜像地址，不是内部资源 ID |
| `image_tag` | `string` | system-managed | no | 镜像标签，便于展示和排查 |
| `image_digest` | `string` | system-managed | no | 最终镜像 digest，不可变版本标识 |
| `pipeline_id` | `string` | system-managed | no | Tekton `PipelineRun` 名称/标识 |
| `trace_id` | `string` | system-managed | no | 创建/构建链路的 trace id |
| `span_id` | `string` | system-managed | no | 创建 Tekton `PipelineRun` 时关联的 parent span id |
| `steps` | `[]ManifestStep` | system-managed | no | 从 Tekton `Pipeline` 动态生成并持续回写的任务步骤快照 |
| `services_snapshot` | `[]ManifestService` | system-managed | no | 冻结服务快照 |
| `workload_config_snapshot` | `ManifestWorkloadConfig` | system-managed | no | 冻结工作负载配置快照 |
| `status` | `ManifestStatus` | system-managed | no | Manifest 状态 |

## Status values

- `Pending`
- `Running`
- `Ready`
- `Succeeded`
- `Failed`

Current code uses `Ready` as an active top-level manifest status:

- `Ready` means manifest steps have completed and the build result is in a deployable state
- `release-service` currently accepts both `ManifestReady` and `ManifestSucceeded` as deployable manifest states

## Boundary summary

`Manifest` is the build-side freeze point.

It owns:

- build identity
- build execution state
- image result
- frozen service and workload snapshots

It does not own:

- environment-specific deployment inputs
- rendered deployment bundle output
- Argo CD application state
- rollout progression

## Dependency inputs

`Manifest` is release-owned, but it freezes inputs from multiple upstream sources.

### Metadata inputs

From `meta-service`:

- application identity
- application name
- repository address

### Config inputs

From `config-service`:

- `workload_config_snapshot`

### Network inputs

From `network-service`:

- `services_snapshot`

### Build-system inputs

From Tekton and registry configuration:

- pipeline topology used to derive `steps`
- image target naming
- build execution status and final image result

## API surface

Service-internal route surface:

- `POST /api/v1/manifests`
- `GET /api/v1/manifests`
- `GET /api/v1/manifests/{id}`
- `DELETE /api/v1/manifests/{id}`

Pre-production shared ingress external surface:

- `POST /api/v1/release/manifests`
- `GET /api/v1/release/manifests`
- `GET /api/v1/release/manifests/{id}`
- `DELETE /api/v1/release/manifests/{id}`

`GET /api/v1/manifests/{id}/resources` returns a derived resource view built from frozen snapshots plus `image_ref`.
On the pre-production shared ingress, the external path is `GET /api/v1/release/manifests/{id}/resources`.
The manifest record itself does not persist rendered output payloads.

## Execution model

The intended manifest lifecycle is:

1. caller submits `POST /api/v1/manifests` with `application_id`
   - pre-production external ingress path: `POST /api/v1/release/manifests`
2. service freezes:
   - `services_snapshot`
   - `workload_config_snapshot`
3. service resolves build context and source metadata:
   - `repo_address`
   - target code revision / commit
4. service creates a Tekton `PipelineRun`
5. service persists:
   - `pipeline_id`
   - `trace_id`
   - `span_id`
   - initial `steps`
   - initial `status`
6. runtime-service watches `PipelineRun` / `TaskRun` updates and continuously writes build progress back into the manifest record
7. when Tekton reaches a terminal state, manifest remains the durable build record even if live cluster objects are later garbage-collected

The manifest record is therefore the durable source of truth for:

- frozen service/workload inputs
- source version metadata
- image build result
- build execution history summary

## Create request contract

The current recommended create request is intentionally small.

### Required request fields

| Field | Type | Required | Why it exists |
|---|---|---|---|
| `application_id` | `uuid.UUID` | yes | identifies which application this manifest/build record belongs to |

### Optional request fields

| Field | Type | Required | Default | Why it exists |
|---|---|---|---|---|
| `git_revision` | `string` | no | `main` | lets caller choose a branch, tag, or exact git commit without exposing separate branch/commit request fields |

### Recommended request shape

```json
{
  "application_id": "11111111-1111-1111-1111-111111111111",
  "git_revision": "main"
}
```

## Why create only needs `application_id` plus optional `git_revision`

At the current boundary, callers should not need to provide:

- `environment_id`
- pipeline step definitions

Reason:

- manifest is environment-agnostic
- manifest exposes build result fields directly (`image_ref`, `image_tag`, `image_digest`)
- manifest does not own release artifact packaging
- build steps come from the Tekton `Pipeline` definition, not from client input
- source/build metadata should be resolved by the service from application/build context and later watcher writeback
- older internal revision linkage should not be treated as manifest contract surface

The active manifest persistence model also does **not** retain legacy release-era columns such as:

- `environment_id`
- `image_id`
- `routes_snapshot`
- `app_config_snapshot`
- `rendered_yaml`
- `rendered_objects`
- `artifact_repository`
- `artifact_tag`
- `artifact_digest`
- `artifact_ref`

## Frozen boundary

The key contract of `Manifest` is that it freezes build-time inputs before release happens.

Frozen on manifest:

- application-scoped service topology via `services_snapshot`
- application-scoped workload runtime shape via `workload_config_snapshot`
- requested source selector via `git_revision`
- resolved immutable source identity via `commit_hash`

Not frozen on manifest:

- `environment_id`
- `app_config_snapshot`
- `routes_snapshot`
- release deployment artifact metadata
- rendered deployment YAML for one environment

Those later deployment facts belong to `Release`.

## Create-time resolved fields

The service is expected to resolve or fill these after request acceptance:

- `repo_address`
- `commit_hash`
- `image_ref`
- `image_tag`
- `image_digest`
- `pipeline_id`
- `trace_id`
- `span_id`
- `steps`
- `services_snapshot`
- `workload_config_snapshot`
- `status`

## Parameters intentionally not accepted today

### `environment_id`

Not accepted because manifest is not environment-bound.
Environment binding belongs to release.

### `branch`

Not accepted as a standalone field.
Branch selection should go through `git_revision`, which can uniformly represent:

- branch
- tag
- commit hash

This avoids parallel request fields that overlap semantically.

### `services_snapshot` / `workload_config_snapshot`

Not accepted from the client because they are server-frozen snapshots.
Allowing caller-supplied snapshots would break trust in manifest as a durable system record.

### `steps`

Not accepted from the client because steps are derived from Tekton pipeline topology.

## Output boundary

`Manifest` produces build-side outputs that later deployment flows consume.

Primary outputs:

- `image_ref`
- `image_tag`
- `image_digest`
- `steps`
- `status`

It does not produce the release deployment artifact.
That OCI deployment bundle is a `Release` output, not a `Manifest` output.

## Future-compatible optional parameters

The following fields are intentionally **not** part of the current recommended contract, but may be introduced later if the product needs more build selection controls:

- `build_profile`
- `pipeline_name`

If introduced later, they should be modeled as explicit source/build selection inputs rather than as deployment or artifact fields.

## Create request examples

### Minimal request

```json
{
  "application_id": "11111111-1111-1111-1111-111111111111"
}
```

## Source resolution rules

The manifest create flow should resolve source inputs using this order:

1. if request includes `git_revision`, use that value as the requested source selector
2. otherwise default requested source selector to `main`
3. resolve the requested selector against the application repository
4. persist:
   - immutable resolved source via `commit_hash`

Recommended interpretation:

- `commit_hash` should always be the final immutable git commit used for the build

This means:

- `git_revision` expresses caller intent
- `commit_hash` is the durable immutable build identity

## End-to-end flow

### 1. Client creates manifest

Client calls:

- `POST /api/v1/manifests`
- pre-production external ingress path: `POST /api/v1/release/manifests`

Request fields:

- required: `application_id`
- optional: `git_revision`
- default `git_revision`: `main`

### 2. Service resolves source

release-service:

- loads application metadata
- reads `repo_address`
- resolves `git_revision`
- computes the immutable `commit_hash` that will actually be built

Result:

- `repo_address` is persisted
- `commit_hash` is persisted

### 3. Service freezes snapshots

release-service freezes:

- `services_snapshot`
- `workload_config_snapshot`

These snapshots become the durable build-time input record for the manifest.

### 4. Service prepares pipeline execution

release-service:

- selects the Tekton pipeline
- reads the Tekton `Pipeline` definition
- derives initial `steps` from:
  - `spec.tasks`
  - `spec.finally`

No static hardcoded step list should be the source of truth for new manifests.

### 5. Service creates PipelineRun with trace context

When creating the Tekton `PipelineRun`, release-service writes trace correlation into annotations:

- `otel.devflow.io/trace-id`
- `otel.devflow.io/parent-span-id`

It also records:

- `pipeline_id`
- `trace_id`
- `span_id`

### 6. Service persists initial manifest state

Initial persisted build record includes:

- `application_id`
- `repo_address`
- `commit_hash`
- `pipeline_id`
- `trace_id`
- `span_id`
- `steps`
- `services_snapshot`
- `workload_config_snapshot`
- `status`

Initial status is typically:

- `Pending`, then
- `Running` once Tekton execution is observed

### 7. runtime-service watches Tekton

runtime-service watches:

- `PipelineRun`
- `TaskRun`

It correlates cluster events back to the manifest using:

- `pipeline_id`
- annotations carrying trace metadata
- task names matching persisted `steps[*].task_name`

### 8. runtime-service writes back progress

runtime-service continuously updates:

- top-level `status`
- `steps[*].status`
- `steps[*].task_run`
- `steps[*].start_time`
- `steps[*].end_time`
- `steps[*].message`

### 9. runtime-service writes back final image result

When the build succeeds, runtime-service persists:

- `image_ref`
- `image_tag`
- `image_digest`

If needed, it may also confirm the final `commit_hash` used by the build flow.

### 10. Manifest remains the durable record

Even after Tekton resources are deleted or garbage-collected:

- `PipelineRun`
- `TaskRun`

the manifest remains the durable source of truth for:

- which repository was built
- which commit was built
- which image was produced
- which pipeline executed it
- which steps succeeded or failed
- which trace to follow for debugging

### Branch selection

```json
{
  "application_id": "11111111-1111-1111-1111-111111111111",
  "git_revision": "main"
}
```

### Commit selection

```json
{
  "application_id": "11111111-1111-1111-1111-111111111111",
  "git_revision": "abcdef1234567890abcdef1234567890abcdef12"
}
```

### Example response shape after creation starts

```json
{
  "data": {
    "id": "22222222-2222-2222-2222-222222222222",
    "application_id": "11111111-1111-1111-1111-111111111111",
    "repo_address": "git@github.com:example/demo.git",
    "commit_hash": "abcdef1234567890",
    "pipeline_id": "devflow-tekton-image-build-push-only-run-abcde",
    "trace_id": "8d4c7c1f7c0b7b9b0d8f4c7c1f7c0b7b",
    "span_id": "7c0b7b9b0d8f4c7c",
    "steps": [
      {
        "task_name": "git-clone",
        "status": "Pending"
      },
      {
        "task_name": "image-build-and-push",
        "status": "Pending"
      }
    ],
    "services_snapshot": [],
    "workload_config_snapshot": {
      "replicas": 1
    },
    "status": "Pending",
    "created_at": "2026-04-27T12:00:00Z",
    "updated_at": "2026-04-27T12:00:00Z"
  }
}
```

## Create / update rules

### Create
- required fields:
  - `application_id`
- server-managed fields:
  - `id`
  - repo / commit / image metadata
  - pipeline / trace metadata
  - dynamic steps
  - snapshots
  - `status`

### Delete
- supported as soft delete through the handler surface

## Status transitions

Recommended `ManifestStatus` transitions:

```text
Pending -> Running -> Ready -> Succeeded
Pending -> Running -> Failed
Pending -> Failed
```

Operational meaning:

| Status | Meaning |
|---|---|
| `Pending` | manifest record created, snapshots frozen, build not yet confirmed running |
| `Running` | Tekton `PipelineRun` has started and at least one task is in progress |
| `Ready` | all manifest steps have completed successfully, but the final build result writeback may still be settling |
| `Succeeded` | all manifest steps completed successfully and final image metadata was written back |
| `Failed` | Tekton pipeline failed, was cancelled, or ended without producing a valid final image result |

Additional rules:

- terminal states are `Succeeded` and `Failed`
- late watcher events must not reopen a terminal manifest unless there is an explicit rebuild/retry operation
- step state is more granular than top-level status; callers should use `steps` for detailed progress and `status` for high-level summary
- writeback result data alone must not promote a manifest to `Succeeded` while any persisted step is still `Pending` or `Running`
- duplicate observer callbacks with unchanged status, step state, or build result should be treated as idempotent no-ops
- if `git_revision` points to a moving reference such as a branch, the persisted `commit_hash` is the durable audit value

## Nested snapshot types

### `ManifestStep`

`ManifestStep` is the persisted per-task execution snapshot derived from a Tekton `Pipeline`.

| Field | Type | Required | Writable | Description |
|---|---|---|---|---|
| `task_name` | `string` | system-managed | no | Tekton pipeline task name |
| `task_run` | `string` | optional | system-managed | 回写后的 `TaskRun` 名称 |
| `status` | `StepStatus` | system-managed | no | 当前步骤状态 |
| `start_time` | `*time.Time` | optional | system-managed | 步骤开始时间 |
| `end_time` | `*time.Time` | optional | system-managed | 步骤结束时间 |
| `message` | `string` | optional | system-managed | runtime-service 回写的状态说明 |

### `ManifestService`

`ManifestService` is the frozen service topology snapshot captured at manifest creation time.

| Field | Type | Required | Writable | Description |
|---|---|---|---|---|
| `id` | `string` | optional | system-managed | 源服务记录标识 |
| `name` | `string` | system-managed | no | 服务名 |
| `ports` | `[]ManifestServicePort` | system-managed | no | 服务端口快照 |

### `ManifestServicePort`

| Field | Type | Required | Writable | Description |
|---|---|---|---|---|
| `name` | `string` | optional | system-managed | 端口名称 |
| `service_port` | `int` | system-managed | no | Service 暴露端口 |
| `target_port` | `int` | system-managed | no | 工作负载目标端口 |
| `protocol` | `string` | optional | system-managed | 协议，通常为 `TCP` |

### `ManifestWorkloadConfig`

`ManifestWorkloadConfig` is the frozen workload configuration snapshot used for image build / later release consumption.

| Field | Type | Required | Writable | Description |
|---|---|---|---|---|
| `id` | `string` | optional | system-managed | 源工作负载配置记录标识 |
| `replicas` | `int` | system-managed | no | 副本数快照 |
| `service_account_name` | `string` | optional | system-managed | 冻结的 Pod `serviceAccountName` |
| `resources` | `map[string]any` | optional | system-managed | 资源限制/请求快照 |
| `probes` | `map[string]any` | optional | system-managed | 健康检查快照 |
| `env` | `[]EnvVar` | optional | system-managed | 环境变量快照 |
| `labels` | `map[string]string` | optional | system-managed | Deployment / Pod metadata labels 快照 |
| `annotations` | `map[string]string` | optional | system-managed | Deployment / Pod metadata annotations 快照 |

## Trace correlation contract

Manifest build operations should remain traceable across:

- API request handling
- manifest persistence
- Tekton `PipelineRun` creation
- runtime-service watcher callbacks
- terminal status writeback

Recommended persistence fields on manifest:

- `trace_id`
- `span_id`
- `pipeline_id`

Recommended Tekton metadata:

- annotations
  - `otel.devflow.io/trace-id`
  - `otel.devflow.io/parent-span-id`

Rules:

- trace identifiers should be stored in annotations, not labels
- runtime-service should read these annotations from `PipelineRun` / `TaskRun` surfaces when available
- watcher logs and writeback logs should include the same `trace_id` and, where practical, the related `span_id`
- `pipeline_id` is the operational join key for cluster-side build events; `trace_id` is the observability join key for distributed tracing

## Runtime writeback contract

runtime-service is the long-running observer for Tekton execution state.

Its writeback responsibilities for manifests should include:

### Pipeline-level writeback

- bind / confirm `pipeline_id`
- transition top-level `status`
- persist terminal image metadata:
  - `image_ref`
  - `image_tag`
  - `image_digest`
  - `commit_hash` if emitted/confirmed by the pipeline flow

### Task-level writeback

For each Tekton pipeline task, write back into `steps[*]`:

- `task_name`
- `task_run`
- `status`
- `start_time`
- `end_time`
- `message`

### Dynamic step discovery

`steps` should be initialized from the referenced Tekton `Pipeline` definition, using:

- `spec.tasks`
- `spec.finally`

This means:

- the application service should not hardcode task names like `git-clone` or `image-build-and-push`
- changing the pipeline definition should naturally change the initial step list for new manifests
- older manifests retain the step topology that existed when they were created

### Result durability

Even if these resources are later deleted from the cluster:

- `PipelineRun`
- `TaskRun`

the manifest row should still preserve enough data to answer:

- which source version was built
- which image was produced
- which pipeline run executed it
- which step failed or succeeded
- which trace should be followed for end-to-end debugging

## Failure cases

Typical create/build failure cases should be handled as follows.

### Invalid request

Examples:

- missing `application_id`
- malformed UUID
- malformed `git_revision` shape if the service applies local validation rules

Recommended result:

- HTTP `400`
- error code `invalid_argument`

### Application not found

Examples:

- `application_id` does not exist
- application was deleted

Recommended result:

- HTTP `404`
- error code `not_found`

### Source resolution failure

Examples:

- repository metadata missing for the application
- repository cannot be accessed
- requested `git_revision` does not exist
- requested `git_revision` resolves ambiguously

Recommended result:

- HTTP `409` or `422` depending on repo-wide convention
- error code `failed_precondition`

The error message should clearly identify whether failure happened at:

- repository lookup
- revision resolution
- commit resolution

### Snapshot freeze failure

Examples:

- service topology cannot be listed
- workload configuration cannot be resolved

Recommended result:

- HTTP `409`
- error code `failed_precondition`

### Pipeline definition failure

Examples:

- referenced Tekton `Pipeline` does not exist
- pipeline cannot be read to derive steps

Recommended result:

- HTTP `409`
- error code `failed_precondition`

### Pipeline submission failure

Examples:

- `PipelineRun` creation fails
- PVC/workspace provisioning fails
- cluster admission rejects the run

Recommended result:

- HTTP `409` or `500` depending on whether the failure is considered business-precondition vs infrastructure
- preferred error code:
  - `failed_precondition` for known missing build prerequisites
  - `internal` for unexpected cluster/client failures

### Runtime writeback failure

Examples:

- runtime-service watcher cannot update manifest row
- watcher cannot correlate `PipelineRun` back to a manifest

Recommended handling:

- the manifest row should remain queryable
- logs must include `pipeline_id` and `trace_id`
- failed writeback should not erase previously stored build state

## Migration notes

This document reflects the newer manifest contract where manifest is a build record plus frozen snapshots.

Compared with the older contract:

- manifest no longer conceptually owns release artifact packaging
- manifest no longer conceptually owns rendered yaml persistence
- manifest is no longer modeled around a separate `image_id` resource
- `steps` are pipeline-derived rather than statically defined
- trace correlation is first-class and durable

Legacy surfaces may still exist temporarily in code during migration, but they should not be treated as the target API contract.

## Final recommended field set

This section is the execution-oriented summary for implementation and review.

### Keep in the public manifest contract

- `id`
- `application_id`
- `repo_address`
- `commit_hash`
- `image_ref`
- `image_tag`
- `image_digest`
- `pipeline_id`
- `trace_id`
- `span_id`
- `steps`
- `services_snapshot`
- `workload_config_snapshot`
- `status`
- `created_at`
- `updated_at`

### Keep in the create request contract

- `application_id`
- `git_revision` (optional, default `main`)

### Internal / not part of the public contract

- `deleted_at`

### Should not be part of the target public manifest contract

- `environment_id`
- `branch` as a standalone request field
- persisted rendered output payloads

## Responsibility split

### Client / frontend

Responsible for:

- choosing `application_id`
- optionally providing `git_revision`
- polling / reading manifest state for progress and result display

Not responsible for:

- constructing snapshots
- constructing pipeline steps
- supplying image metadata
- supplying trace metadata

### release-service

Responsible for:

- validating create requests
- resolving source inputs
- freezing `services_snapshot`
- freezing `workload_config_snapshot`
- selecting and reading Tekton `Pipeline`
- deriving initial `steps`
- creating `PipelineRun`
- persisting initial manifest record

### runtime-service

Responsible for:

- watching `PipelineRun`
- watching `TaskRun`
- correlating Tekton events back to manifests
- writing step-level progress
- writing terminal build result fields
- preserving trace correlation in logs and callbacks

### Tekton

Responsible for:

- executing the actual build pipeline
- producing final image metadata
- exposing task- and pipeline-level execution status to watchers

## Implementation status

Current implementation direction should match this document:

- manifest create request stays minimal: `application_id` plus optional `git_revision`
- manifest response exposes source/build/image result fields, not `image_id`
- manifest domain model stays environment-agnostic
- manifest no longer owns release artifact packaging
- manifest no longer persists rendered deployment YAML as its active contract
- runtime tracking fields are part of the active contract:
  - `pipeline_id`
  - `trace_id`
  - `span_id`
  - `steps`

### Persistence

- manifest table schema matches the target build-record model
- repository read/write paths persist build metadata fields
- repository read/write paths persist dynamic `steps`
- repository read/write paths preserve terminal state correctness

### Source resolution

- [ ] if `git_revision` is empty, service defaults to `main`
- [ ] if `git_revision` is provided, service accepts:
  - [ ] branch
  - [ ] tag
  - [ ] commit hash
- [ ] service persists final immutable `commit_hash`

### Tekton integration

- [ ] `PipelineRun` annotations include:
  - [ ] `otel.devflow.io/trace-id`
  - [ ] `otel.devflow.io/parent-span-id`
- [ ] initial `steps` are derived from Tekton `Pipeline` structure
- [ ] no hardcoded task list is used as the source of truth

### Tekton observer writeback

- [ ] watcher can correlate `PipelineRun` to manifest
- [ ] watcher updates top-level `status`
- [ ] watcher updates step-level fields:
  - [ ] `task_run`
  - [ ] `status`
  - [ ] `start_time`
  - [ ] `end_time`
  - [ ] `message`
- [ ] watcher writes terminal image fields:
  - [ ] `image_ref`
  - [ ] `image_tag`
  - [ ] `image_digest`

### Docs / migration cleanup

- [ ] release docs and runtime docs reference manifest as the durable build record
- [ ] old image/manifest overlap is removed or explicitly documented during migration

## Open implementation questions

These decisions should be made explicitly during implementation if they are still unresolved:

- should `git_revision` itself be persisted on the manifest row, or remain request-only context?
- how should the system resolve `commit_hash` for branch/tag input:
  - directly before `PipelineRun` creation
  - or from Tekton-emitted results after clone/build starts?
- should a failed source-resolution attempt create no manifest row at all, or create a failed manifest row?
- should terminal cancellation map to `Failed`, or introduce a future `Cancelled` status?

## Validation notes

- invalid UUID path or query parameters return `invalid_argument`
- missing records return `not_found`
- create-time dependency mismatches, missing build inputs, or pipeline submission failures return `failed_precondition`
- list endpoints support `application_id` and `include_deleted`
- `services_snapshot` and `workload_config_snapshot` are immutable snapshots captured at manifest creation time
- `repo_address`, `commit_hash`, `image_ref`, `image_tag`, and `image_digest` are build record fields; callers should treat them as system-populated outputs even if the underlying values originate from source control or Tekton results
- `git_revision` may be omitted; omission means the service should build from `main`
- `git_revision` may be a branch, tag, or commit hash
- callers should use `commit_hash` rather than `git_revision` when they need an immutable audit value after creation
- `steps` must not be hardcoded in application logic; they should be derived from the referenced Tekton `Pipeline` definition (`spec.tasks` + `spec.finally`)
- runtime-service is expected to watch Tekton `PipelineRun` / `TaskRun` updates and continuously write back:
  - `status`
  - `steps[*].status`
  - `steps[*].task_run`
  - `steps[*].start_time`
  - `steps[*].end_time`
  - `steps[*].message`
- Tekton `PipelineRun` should carry trace correlation through annotations, not labels:
  - `otel.devflow.io/trace-id`
  - `otel.devflow.io/parent-span-id`
- `trace_id` / `span_id` are stored on the manifest so the build record remains traceable after Tekton resources are garbage-collected
- `pipeline_id` should be stable enough to correlate with cluster-side logs, events, and retained audit records even after the live `PipelineRun` object is deleted
- 当前边界上，manifest 主责是：
  - 冻结 `services_snapshot` / `workload_config_snapshot`
  - 触发 Tekton 镜像构建
  - 保存代码版本与镜像版本结果
  - 保存 Tekton 步骤状态与链路追踪信息

## Explicit non-goals

The current manifest resource does **not** own:

- environment binding
- release artifact packaging
- rendered kubernetes yaml persistence
- rendered object persistence
- an external `image` resource contract

Those concerns belong to `release-service` or to internal implementation details, not to the manifest API contract.

## Source pointers

- module: `internal/manifest/module.go`
- domain: `internal/manifest/domain/manifest.go`
- service: `internal/manifest/service/manifest.go`
- handler: `internal/manifest/transport/http/manifest_handler.go`
