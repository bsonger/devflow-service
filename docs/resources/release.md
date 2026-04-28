# Release

## Ownership

- active service boundary: `release-service`
- domain package: `internal/release/domain`
- handler package: `internal/release/transport/http`
- service package: `internal/release/service`

## Purpose

`Release` is an environment-specific deployment execution record derived from one manifest plus rollout-time environment inputs.

它的核心职责不是构建镜像，而是：

- 选择一个 `manifest`
- 选择一个目标 `environment`
- 冻结本次发布真正使用的 `app_route` / `app_config`
- 选择发布策略
- 根据 manifest + environment inputs 渲染 Kubernetes YAML
- 将渲染结果上传到 OCI
- 创建 ArgoCD Application
- 持续跟踪部署状态直到完成

## Relationship with Manifest

`Manifest` 和 `Release` 的职责应该明确分层：

### Manifest 负责

- build record
- source/build metadata
- image result
- `services_snapshot`
- `workload_config_snapshot`

### Release 负责

- environment-specific deployment
- `app_route_snapshot`
- `app_config_snapshot`
- strategy selection
- Kubernetes YAML rendering
- OCI packaging for deployment bundle
- ArgoCD Application creation
- rollout status tracking

结论：

- manifest 不应该再拥有 release artifact packaging
- manifest 不应该再拥有 environment-specific rendered YAML contract
- release 应该消费 manifest 中已经冻结好的 build outputs and workload snapshots

## Final create contract

Current target create request:

```json
{
  "manifest_id": "11111111-1111-1111-1111-111111111111",
  "environment_id": "production",
  "strategy": "blueGreen",
  "type": "Upgrade"
}
```

Rules:

- required:
  - `manifest_id`
  - `environment_id`
  - `strategy`
- optional:
  - `type`
- not accepted as user input:
  - `artifact_repository`
  - `artifact_tag`
  - `artifact_digest`
  - `artifact_ref`
  - `routes_snapshot`
  - `app_config_snapshot`
  - `steps`
  - `status`
  - `external_ref`

## Final ownership conclusion for contentious fields

### `environment_id`

- must stay on `Release`
- because release is environment-specific deployment execution
- manifest is environment-agnostic and should not carry deployment environment identity

### `image_ref`

- release consumes the built image through `manifest.image_ref`
- release itself should not introduce a separate `image_id` field into its API contract

### `artifact_ref` vs `image_ref`

- `image_ref` belongs to `Manifest`
- `artifact_ref` belongs to `Release`

They are not duplicates because they point to different things:

- `image_ref`: workload image produced by build
- `artifact_ref`: deployment bundle OCI artifact produced by release execution

## Common base fields

| Field | Type | Required | Writable | Description |
|---|---|---|---|---|
| `id` | `uuid.UUID` | server-generated | no | 主键 |
| `created_at` | `time.Time` | server-generated | no | 创建时间 |
| `updated_at` | `time.Time` | server-generated | no | 更新时间 |
| `deleted_at` | `*time.Time` | optional | system-managed | 软删除时间 |

## Field table

### Stable release contract fields

| Field | Type | Required | Writable | Description |
|---|---|---|---|---|
| `execution_intent_id` | `*uuid.UUID` | system-managed | no | 关联执行意图 |
| `application_id` | `uuid.UUID` | system-managed | no | 关联应用 ID |
| `manifest_id` | `uuid.UUID` | required | user | 关联 manifest |
| `environment_id` | `string` | required | user | 目标环境标识 |
| `routes_snapshot` | `[]ReleaseRoute` | system-managed | no | release 创建时冻结的 route 快照 |
| `app_config_snapshot` | `ReleaseAppConfig` | system-managed | no | release 创建时冻结的 app config 快照 |
| `strategy` | `string` | required | user | 本次发布选择的 rollout 策略 |
| `steps` | `[]ReleaseStep` | system-managed | no | 发布步骤，使用稳定 `code` 标识每个步骤 |
| `status` | `ReleaseStatus` | system-managed | no | 发布状态 |
| `external_ref` | `string` | system-managed | no | 外部系统引用，例如 ArgoCD Application 名称 |

### Deployment artifact fields recommended for release ownership

These fields describe the rendered deployment bundle produced by release-service and should belong to `Release`, not `Manifest`.

| Field | Type | Required | Writable | Description |
|---|---|---|---|---|
| `artifact_repository` | `string` | system-managed | no | 发布 YAML bundle 所在 OCI repository |
| `artifact_tag` | `string` | system-managed | no | 发布 bundle tag |
| `artifact_digest` | `string` | system-managed | no | 发布 bundle digest |
| `artifact_ref` | `string` | system-managed | no | 完整 OCI 引用 |

### Observability fields recommended for release ownership

| Field | Type | Required | Writable | Description |
|---|---|---|---|---|
| `trace_id` | `string` | system-managed | no | 贯穿渲染、上传、Argo 创建、runtime 跟踪的 trace id |
| `span_id` | `string` | system-managed | no | 创建外部部署对象时的关键 parent span id |

## Action semantics field

### `type`

Current code still has `type` values such as:

- `Install`
- `Upgrade`
- `Rollback`

This field can continue to exist if the system wants to distinguish action semantics.
But rollout strategy should not be overloaded into `type`.

Recommended split:

- `type`: install / upgrade / rollback
- `strategy`: rolling / blueGreen / canary

Final API stance:

- keep `type` in request/response for action semantics
- do not use `type` to encode rollout strategy

## Strategy values

Recommended rollout strategy values:

- `rolling`
- `blueGreen`
- `canary`

The chosen strategy determines what YAML is rendered and what runtime tracking logic is expected.

Examples:

- `rolling` -> `Deployment`
- `blueGreen` -> `Rollout` plus blue/green traffic/service switching resources
- `canary` -> `Rollout` plus canary traffic split resources

## Release status values

Current/target top-level status values:

- `Pending`
- `Running`
- `Succeeded`
- `Failed`
- `RollingBack`
- `RolledBack`

`Syncing` / `SyncFailed` can be treated as step-level or transitional semantics if the system later wants to simplify the top-level state model.

## Step status values

- `Pending`
- `Running`
- `Succeeded`
- `Failed`

## ReleaseStep structure

Recommended `ReleaseStep` shape:

| Field | Type | Required | Writable | Description |
|---|---|---|---|---|
| `code` | `string` | system-managed | no | 稳定机器标识，用于服务端/observer 精确定位步骤 |
| `name` | `string` | system-managed | no | 面向用户展示的步骤名 |
| `progress` | `int32` | system-managed | no | 进度百分比，`0-100` |
| `status` | `StepStatus` | system-managed | no | 步骤状态 |
| `message` | `string` | system-managed | no | 当前步骤说明或错误信息 |
| `start_time` | `time.Time` | system-managed | no | 开始时间 |
| `end_time` | `*time.Time` | system-managed | no | 结束时间 |

Example:

```json
{
  "code": "publish_bundle",
  "name": "Publish bundle to OCI",
  "status": "Running",
  "progress": 45,
  "message": "Uploading deployment bundle",
  "start_time": "2026-04-28T10:00:00Z",
  "end_time": null
}
```

Design rule:

- `code` is for machines and must stay stable
- `name` is for humans and may change over time
- runtime-service and release-service should update steps by `code`, not by display name
- writeback payloads should prefer `step_code`; `step_name` is migration-only compatibility input

## API surface

- `POST /api/v1/releases`
- `GET /api/v1/releases?application_id=...&environment_id=...`
- `GET /api/v1/releases/{id}`
- `GET /api/v1/releases/{id}/bundle-preview`
- `DELETE /api/v1/releases/{id}`
- `POST /api/v1/verify/argo/events`
- `POST /api/v1/verify/release/steps`
- `POST /api/v1/verify/release/artifact`

## List and read contract

Recommended stable read fields:

- `id`
- `execution_intent_id`
- `application_id`
- `manifest_id`
- `environment_id`
- `strategy`
- `type`
- `status`
- `routes_snapshot`
- `app_config_snapshot`
- `steps`
- `artifact_repository`
- `artifact_tag`
- `artifact_digest`
- `artifact_ref`
- `external_ref`
- `created_at`
- `updated_at`

Recommended list filters:

- `application_id`
- `environment_id`
- `manifest_id`
- `status`
- `type`
- `include_deleted`

Validation note:

- `GET /api/v1/releases` requires both `application_id` and `environment_id`

## Execution model

Release execution should be modeled as asynchronous intent-driven work.

`POST /api/v1/releases` should:

- validate the request
- freeze release-time inputs
- create the release record
- create/schedule execution intent
- return the release record quickly

It should **not** require the HTTP request to synchronously complete:

- deployment bundle rendering
- OCI upload
- ArgoCD Application creation
- rollout completion

Reason:

- these are long-running external operations
- they need retries and resumability
- they should be tracked through release status and steps rather than request blocking

## Release creation flow

## 1. Client creates release

The caller selects:

- one `manifest`
- one target `environment`
- one rollout `strategy`

Recommended request shape:

```json
{
  "manifest_id": "11111111-1111-1111-1111-111111111111",
  "environment_id": "production",
  "strategy": "blueGreen",
  "type": "Upgrade"
}
```

## 2. Service validates release inputs

release-service should validate:

- manifest exists
- manifest is publishable
- environment exists / is deployable
- manifest belongs to the same application that the target environment can serve

Recommended manifest readiness rule:

- accept terminal successful build output, e.g. `Succeeded`
- compatibility mode may still accept older `Ready`

## 3. Service freezes rollout-time inputs

release-service freezes:

- `app_config_snapshot`
- `routes_snapshot`

It also reads from manifest:

- `image_ref`
- `services_snapshot`
- `workload_config_snapshot`

Together these form the immutable deployment input set for the release.

## 3.5 Service creates release record and execution intent

After validation and snapshot freeze, release-service should:

- persist the release record
- initialize release steps
- set initial `status`, typically `Pending`
- create/schedule execution intent

The HTTP create call should return after this phase.

## 4. Executor renders deployment bundle

An asynchronous executor should render environment-specific Kubernetes resources from:

### Inputs from Manifest

- `image_ref`
- `services_snapshot`
- `workload_config_snapshot`

### Inputs from Release

- `environment_id`
- `app_config_snapshot`
- `routes_snapshot`
- `strategy`

### Typical outputs

- `ConfigMap`
- `Service`
- `Deployment` or `Rollout`
- `VirtualService`
- `DestinationRule`
- strategy-specific routing / traffic resources

The output of this phase is a deployment bundle, not a manifest resource record.

This rendering responsibility belongs to release-service's deployment execution flow, not manifest.

## 5. Executor uploads deployment bundle to OCI

After rendering the deployment bundle:

- package the deployment YAML bundle
- push it to OCI
- persist:
  - `artifact_repository`
  - `artifact_tag`
  - `artifact_digest`
  - `artifact_ref`

This deployment bundle is the release artifact.

## 6. Executor creates ArgoCD Application

The asynchronous execution flow creates an ArgoCD Application that points to the OCI bundle.

ArgoCD should pull:

- deployment bundle YAML from OCI

not regenerate manifests from scratch, and not depend on manifest-era artifact ownership.

## 7. ArgoCD starts deployment

ArgoCD syncs the bundle into the target environment.

Depending on strategy:

- rolling deploys a standard workload update
- blueGreen manages preview/active switch
- canary performs staged traffic progression

## 8. runtime-service tracks rollout status

runtime-service continuously watches rollout execution and writes back:

- release top-level `status`
- release `steps`
- progress messages
- failure reasons
- external deployment references

Tracked objects may include:

- ArgoCD Application
- Rollout / Deployment
- ReplicaSet
- Pod
- Service
- VirtualService

## 9. Release remains the durable deployment record

Even after transient runtime objects change or are garbage-collected, the release record should preserve:

- what was deployed
- where it was deployed
- which strategy was used
- which YAML bundle was used
- what status/progress the deployment reached

## Create rules

### Required request fields

- `manifest_id`
- `environment_id`
- `strategy`

### Optional request fields

- `type`

### Server-managed fields

- `id`
- `application_id`
- `execution_intent_id`
- `routes_snapshot`
- `app_config_snapshot`
- `steps`
- `status`
- `external_ref`
- deployment artifact fields
- observability fields

### Fields that should not appear in the final create payload

- `image_ref`
- `artifact_repository`
- `artifact_tag`
- `artifact_digest`
- `artifact_ref`
- `routes_snapshot`
- `app_config_snapshot`
- `steps`
- `status`
- `external_ref`

## Create response semantics

`POST /api/v1/releases` should be treated as:

- release record created
- execution scheduled

It should not be interpreted as:

- OCI artifact already published
- ArgoCD Application already created
- deployment already started
- deployment already completed

## Validation notes

- invalid UUID path or query parameters return `invalid_argument`
- missing records return `not_found`
- create-time environment readiness or deploy-target readiness problems return `failed_precondition`
- release 创建时应冻结当前 `app_config` / `app_route`
- release 渲染 deployment bundle 时应使用 manifest 中已经冻结好的 workload/service/image 信息
- release 的 environment 语义应该由 `release.environment_id` 明确表达，而不是由 build-time image metadata 反推
- ArgoCD source 应该消费 release 产出的 OCI deployment bundle
- release create 接口应快速返回，外部部署动作应由异步执行链路推进

## Responsibility split by phase

### release-service synchronous create phase

- validate request
- resolve manifest/environment binding
- freeze `app_config_snapshot`
- freeze `routes_snapshot`
- persist release
- initialize steps and status
- create execution intent

### asynchronous executor phase

- `render_deployment_bundle`
- `publish_bundle`
- `create_argocd_application`
- trigger deployment start

Current implementation direction:

- release intents are claimed by kind = `release`
- executor dispatches the release through release-service orchestration
- dispatch flow is now split into explicit phases:
  - `render_deployment_bundle`
  - `publish_bundle`
  - `create_argocd_application`
- render phase now builds a release-owned in-memory deployment bundle structure from:
  - `manifest.services_snapshot`
  - `manifest.workload_config_snapshot`
  - `manifest.image_ref`
  - `release.app_config_snapshot`
  - `release.routes_snapshot`
- publish phase now flows through a bundle publisher abstraction
- current default publisher records artifact metadata from rendered bundle content and digest
- publisher modes:
  - `metadata`: metadata-only
  - `oras`: package bundle as OCI artifact and push/tag to remote registry when manifest registry is enabled
- `publish_bundle` step message should prefer carrying both publisher mode and final artifact ref for operator diagnostics
- `create_argocd_application` step message should prefer carrying application name, target environment, and final artifact ref when available
- intent is then marked as `Running` after dispatch is accepted
- later runtime / writeback callbacks continue updating release status and steps
- when release-service runs in intent mode and worker config is enabled, it starts a background release intent worker loop

### runtime-service observer phase

- watch ArgoCD / rollout progress
- update step status by `code`
- update top-level release status
- persist rollout messages and failures
- finalize release outcome

## Step naming guidance

To avoid confusion with the `Manifest` resource, release steps should avoid names like:

- `render_manifests`

Preferred terminology:

- `render_deployment_bundle`
- `publish_bundle`

Reason:

- `Manifest` is the build record resource
- `deployment bundle` is the environment-specific rendered output owned by release

## Recommended step templates

### rolling

- `freeze_inputs`
- `render_deployment_bundle`
- `publish_bundle`
- `create_argocd_application`
- `start_deployment`
- `observe_rollout`
- `finalize_release`

Suggested display names:

- `freeze_inputs` -> `Freeze release inputs`
- `render_deployment_bundle` -> `Render deployment bundle`
- `publish_bundle` -> `Publish bundle to OCI`
- `create_argocd_application` -> `Create ArgoCD application`
- `start_deployment` -> `Start deployment`
- `observe_rollout` -> `Observe rollout`
- `finalize_release` -> `Finalize release`

### blueGreen

- `freeze_inputs`
- `render_deployment_bundle`
- `publish_bundle`
- `create_argocd_application`
- `deploy_preview`
- `observe_preview`
- `switch_traffic`
- `verify_active`
- `finalize_release`

### canary

- `freeze_inputs`
- `render_deployment_bundle`
- `publish_bundle`
- `create_argocd_application`
- `deploy_canary`
- `canary_10`
- `canary_30`
- `canary_60`
- `canary_100`
- `finalize_release`

These recommended step entries are the stable `code` values.
Human-facing text can be localized or adjusted independently via the `name` field.

## Step ownership and lifecycle

Release steps should be owned by different phases of the system:

### 1. create phase steps

These are completed during synchronous `POST /releases` handling.

Recommended:

- `freeze_inputs`

Suggested behavior:

- initialize the full step list at release creation time
- mark all steps as `Pending`
- once snapshots are frozen successfully, mark `freeze_inputs` as `Succeeded`

### 2. asynchronous executor steps

These are advanced by the asynchronous deployment executor.

Common executor-owned steps:

- `render_deployment_bundle`
- `publish_bundle`
- `create_argocd_application`

Rolling deployments may also include:

- `start_deployment`

### 3. runtime-service observer steps

These are advanced by runtime-service based on real cluster rollout state.

Examples:

- `observe_rollout`
- `deploy_preview`
- `observe_preview`
- `switch_traffic`
- `verify_active`
- `deploy_canary`
- `canary_10`
- `canary_30`
- `canary_60`
- `canary_100`
- `finalize_release`

## Step ownership table

| Step code | Owner | Meaning |
|---|---|---|
| `freeze_inputs` | release-service create phase | 冻结 release-time snapshots |
| `render_deployment_bundle` | async executor | 渲染 deployment bundle |
| `publish_bundle` | async executor | 上传 deployment bundle 到 OCI |
| `create_argocd_application` | async executor | 创建 ArgoCD Application |
| `start_deployment` | async executor | 触发 rolling deployment 开始 |
| `observe_rollout` | runtime-service | 跟踪 rolling rollout |
| `deploy_preview` | runtime-service | blueGreen preview 部署阶段 |
| `observe_preview` | runtime-service | 观察 blueGreen preview 健康状态 |
| `switch_traffic` | runtime-service | blueGreen 切流 |
| `verify_active` | runtime-service | blueGreen active 验证 |
| `deploy_canary` | runtime-service | canary 初始部署 |
| `canary_10` | runtime-service | canary 10% 流量阶段 |
| `canary_30` | runtime-service | canary 30% 流量阶段 |
| `canary_60` | runtime-service | canary 60% 流量阶段 |
| `canary_100` | runtime-service | canary 100% 流量阶段 |
| `finalize_release` | runtime-service | 收口 release 成功/失败终态 |

## Step initialization rules

When a release record is first created:

- initialize the complete strategy-specific step list immediately
- assign stable `code` values to every step
- initialize all step `status` values as `Pending`
- initialize all step `progress` values as `0`

Then:

- release-service marks `freeze_inputs` as `Succeeded` once snapshot freeze completes
- async executor advances executor-owned steps
- runtime-service advances rollout-observer steps

## Strategy-specific lifecycle templates

### rolling lifecycle

#### create phase

- `freeze_inputs` -> `Succeeded`

#### async executor phase

- `render_deployment_bundle`
- `publish_bundle`
- `create_argocd_application`
- `start_deployment`

#### runtime-service phase

- `observe_rollout`
- `finalize_release`

### blueGreen lifecycle

#### create phase

- `freeze_inputs` -> `Succeeded`

#### async executor phase

- `render_deployment_bundle`
- `publish_bundle`
- `create_argocd_application`

#### runtime-service phase

- `deploy_preview`
- `observe_preview`
- `switch_traffic`
- `verify_active`
- `finalize_release`

### canary lifecycle

#### create phase

- `freeze_inputs` -> `Succeeded`

#### async executor phase

- `render_deployment_bundle`
- `publish_bundle`
- `create_argocd_application`

#### runtime-service phase

- `deploy_canary`
- `canary_10`
- `canary_30`
- `canary_60`
- `canary_100`
- `finalize_release`

## Top-level status progression

Recommended top-level status semantics:

- `Pending` -> release record created, execution not yet completed
- `Running` -> async executor or rollout observer has started advancing deployment
- `Succeeded` -> all required steps completed successfully
- `Failed` -> one of the required steps failed terminally
- `RollingBack` / `RolledBack` -> explicit rollback flow

## Finalization rule

`finalize_release` should not be treated as an empty placeholder step.

It should mean:

- required rollout conditions were satisfied
- the deployment reached its intended terminal state
- the release record can be safely marked as final success or final failure

## Source pointers

- module: `internal/release/module.go`
- domain: `internal/release/domain/release.go`
- types: `internal/release/domain/types.go`
- service: `internal/release/service/release.go`
- handler: `internal/release/transport/http/release_handler.go`
- writeback: `internal/release/transport/http/release_writeback.go`
