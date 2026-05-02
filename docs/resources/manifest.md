# Manifest

## 这个文档解决什么问题

这份文档说明 `Manifest` 这个资源为什么存在，以及它到底冻结了什么、没有冻结什么。

读完后，读者应该能回答：

- `Manifest` 为什么是 build-side freeze point
- 哪些输入会被冻结到 `Manifest`
- 为什么环境差异不属于 `Manifest`

## Ownership

- active service boundary: `release-service`
- runnable host process: `release-service`
- domain package: `internal/manifest/domain`
- handler package: `internal/manifest/transport/http`
- service package: `internal/manifest/service`

## Purpose

`Manifest` 是一个 application revision 对应的 build-time snapshot，也是 image-delivery record。

它负责：

- 冻结 service snapshot
- 冻结 workload snapshot
- 触发 Tekton image build
- 记录最终 image result

即使后续 `PipelineRun` / `TaskRun` 被回收，`Manifest` 仍然是持久可追踪的 build record。

## Quick reader guide

当你要回答下面这些 build-side 问题时，看这篇文档：

- what exactly was frozen before image build started
- which source revision was actually built
- what image was produced
- how Tekton progress maps back to one durable system record

如果你要问的是下面这些 deploy-side 问题：

- target environment
- app config used for deployment
- rendered deployment YAML
- Argo CD deployment state
- published OCI deployment bundle

那就应该去看 `Release`，而不是 `Manifest`。

For repo-wide API envelope, pagination, and compatibility rules, also see:

- `docs/api/contract-guide.md`

## Common base fields

| Field | Type | Required | Writable | Description |
|---|---|---|---|---|
| `id` | `uuid.UUID` | server-generated | no | Primary key |
| `created_at` | `time.Time` | server-generated | no | Create timestamp |
| `updated_at` | `time.Time` | server-generated | no | Update timestamp |
| `deleted_at` | `*time.Time` | optional | system-managed | Soft-delete timestamp |

## Field table

| Field | Type | Required | Writable | Description |
|---|---|---|---|---|
| `application_id` | `uuid.UUID` | required | user | Owning application ID |
| `git_revision` | `string` | optional | user | Source selector: branch, tag, or commit; defaults to `main` |
| `repo_address` | `string` | system-managed | no | Repository address resolved by the service |
| `commit_hash` | `string` | system-managed | no | Immutable source commit actually built |
| `image_ref` | `string` | system-managed | no | Final image reference used downstream |
| `image_tag` | `string` | system-managed | no | Human-readable image tag |
| `image_digest` | `string` | system-managed | no | Immutable image digest |
| `pipeline_id` | `string` | system-managed | no | Tekton `PipelineRun` identifier |
| `trace_id` | `string` | system-managed | no | Durable trace identifier for the build path |
| `span_id` | `string` | system-managed | no | Parent span correlation for pipeline creation |
| `steps` | `[]ManifestStep` | system-managed | no | Tekton-derived task snapshots |
| `services_snapshot` | `[]ManifestService` | system-managed | no | Frozen service topology snapshot |
| `workload_config_snapshot` | `ManifestWorkloadConfig` | system-managed | no | Frozen workload snapshot using the constrained workload contract |
| `status` | `ManifestStatus` | system-managed | no | Build-side manifest status |

## Status values

- `Pending`
- `Running`
- `Available`
- `Unavailable`

当前 `Manifest.status` 语义：

- `Pending`: the manifest record has been frozen and the build has been dispatched, but `release-service` has not yet received the first runtime / observer status writeback
- `Running`: runtime writeback reports that the build execution is in progress
- `Available`: runtime writeback reports that the manifest is deployable and can be consumed by release creation
- `Unavailable`: runtime writeback reports a terminal non-consumable outcome for this manifest

最重要的边界规则：

- manifest status is driven by runtime / observer writeback
- `steps[*].status` are per-task observation details only
- `release-service` does not locally derive aggregate manifest status from step completion or image-result persistence

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

`GET /api/v1/manifests/{id}/resources` 返回的是一个 derived inspection view。
它由 frozen snapshot 和 `image_ref` 组合而成。

`Manifest` 本身并不持久化 release-owned bundle payload。

## Frozen boundary

`Manifest` 的关键契约，是在 release 发生前冻结 build-time 输入。

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

这些后续 deployment facts 都属于 `Release`。

## Manifest workload snapshot

`workload_config_snapshot` 复用了 `config-service` 那套 constrained workload contract。

它不是一个宽泛兼容层，也不应该演化成第二套并行契约。

### `ManifestWorkloadConfig`

| Field | Type | Required | Writable | Description |
|---|---|---|---|---|
| `id` | `string` | optional | system-managed | Source workload-config record identifier |
| `replicas` | `int` | system-managed | no | Frozen replica count |
| `service_account_name` | `string` | optional | system-managed | Frozen Pod `serviceAccountName` |
| `resources` | `WorkloadResourceRequirements` | optional | system-managed | Frozen constrained resource contract |
| `probes` | `WorkloadProbes` | optional | system-managed | Frozen named HTTP probes contract |
| `env` | `[]EnvVar` | optional | system-managed | Frozen environment variable entries |
| `labels` | `map[string]string` | optional | system-managed | Frozen metadata labels |
| `annotations` | `map[string]string` | optional | system-managed | Frozen metadata annotations |

### Snapshot meaning

这个 snapshot 是 build-side 的 durable evidence，表示 `release-service` 在创建 `Manifest` 时到底从 `config-service` 读到了什么。

It is intentionally:

- **typed**, not free-form
- **frozen**, not live-linked back to mutable workload state
- **application-scoped**, not environment-specific
- **input-oriented**, not already rendered into Kubernetes YAML

### Snapshot resource shape

`resources` uses the same object shape as `WorkloadConfig`:

```json
{
  "size_class": "medium",
  "requests": {
    "cpu": "250m",
    "memory": "256Mi"
  },
  "limits": {
    "cpu": "1",
    "memory": "1Gi"
  }
}
```

canonical size-class mapping table 定义在 `internal/workloadconfig/domain.WorkloadSizeClassResources`，并记录在 `docs/resources/workloadconfig.md`。

`Manifest` snapshot 必须保留这套共享契约，不能擅自翻译成 manifest-local 默认表。

### Snapshot probe shape

`probes` uses named slots instead of a wide arbitrary map:

```json
{
  "liveness": {
    "path": "/healthz",
    "port": "http",
    "initial_delay_seconds": 10,
    "period_seconds": 10,
    "timeout_seconds": 5,
    "failure_threshold": 3
  },
  "readiness": {
    "path": "/readyz",
    "port": "http",
    "initial_delay_seconds": 5,
    "period_seconds": 10,
    "timeout_seconds": 5,
    "failure_threshold": 3
  },
  "startup": {
    "path": "/startupz",
    "port": "http",
    "period_seconds": 5,
    "failure_threshold": 12
  }
}
```

### Render-time translation boundary

这个 snapshot **不表示** `Manifest` 持久化了 Kubernetes-native 字段，例如：

- `livenessProbe`
- `readinessProbe`
- `startupProbe`
- container `resources`

真正的边界是：

- `Manifest` stores the frozen constrained workload snapshot
- manifest/release renderers later translate that snapshot into Kubernetes-shaped output for inspection views and release bundles

这是一个很重要的 anti-drift 规则：

- freeze once in the constrained contract
- translate later at the rendering seam
- do not widen the frozen snapshot back into legacy free-form maps

## Create request contract

当前推荐的 create request 故意保持很小。

### Required request fields

| Field | Type | Required | Why it exists |
|---|---|---|---|
| `application_id` | `uuid.UUID` | yes | Identifies which application this manifest/build record belongs to |

### Optional request fields

| Field | Type | Required | Default | Why it exists |
|---|---|---|---|---|
| `git_revision` | `string` | no | `main` | Lets callers choose a branch, tag, or exact git commit without exposing parallel branch/commit fields |

### Recommended request shape

```json
{
  "application_id": "11111111-1111-1111-1111-111111111111",
  "git_revision": "main"
}
```

## Execution model

The intended manifest lifecycle is:

1. caller submits `POST /api/v1/manifests` with `application_id`
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

## Validation and migration notes

Current contract expectations:

- invalid UUID path or query parameters return `invalid_argument`
- missing records return `not_found`
- create-time dependency mismatches, missing build inputs, or pipeline submission failures return `failed_precondition`
- list endpoints support `application_id` and `include_deleted`
- `services_snapshot` and `workload_config_snapshot` are immutable snapshots captured at manifest creation time
- callers should treat `repo_address`, `commit_hash`, `image_ref`, `image_tag`, and `image_digest` as system-populated outputs

For workload snapshots specifically:

- manifest snapshots should preserve the constrained workload shape from `WorkloadConfig`
- later slices may still deepen validation or migration behavior around size-class alignment and legacy cleanup
- manifest docs should describe the frozen shape honestly without claiming that manifest itself performs all config-service validation duties

## Explicit non-goals

The current manifest resource does **not** own:

- environment binding
- release artifact packaging
- rendered Kubernetes YAML persistence as a durable manifest field
- a separate external `image` resource contract
- a second, manifest-local workload schema that drifts from `WorkloadConfig`

## Observability and drift checks

The drift signals for this contract are:

- canonical release-service OpenAPI in `api/openapi/release-service.yaml`
- aggregate OpenAPI in `api/openapi/devflow.yaml`
- generated Swagger snapshot in `api/openapi/swagger.yaml`
- manifest/release/workload contract tests in `internal/manifest/...`, `internal/release/...`, and `internal/workloadconfig/...`
- final repo verification via `bash scripts/verify.sh`

When the frozen workload snapshot shape changes, update code, OpenAPI contracts, generated artifacts, and this document together.

## Source pointers

- module: `internal/manifest/module.go`
- domain: `internal/manifest/domain/manifest.go`
- service: `internal/manifest/service/manifest.go`
- handler: `internal/manifest/transport/http/manifest_handler.go`
