# Release

## Ownership

- active service boundary: `release-service`
- runnable host process: `release-service`
- domain package: `internal/release/domain`
- handler package: `internal/release/transport/http`
- service package: `internal/release/service`

## Purpose

`Release` is an environment-specific deployment execution record derived from one manifest plus rollout-time environment inputs.

Its job is not to build an image. Its job is to:

- choose one `manifest`
- choose one target `environment`
- freeze the `app_config` used by this deployment
- optionally carry deferred route inputs when route flow is part of the deployment path
- choose the rollout strategy
- render Kubernetes YAML from manifest inputs plus environment inputs
- publish the rendered bundle to OCI
- create the Argo CD `Application`
- track deployment progress until completion

## Quick reader guide

Use this document when you need to answer deploy-side questions such as:

- which manifest was deployed
- which environment was targeted
- which app config and routes were frozen
- what deployment artifact was published
- what Argo CD external reference was created
- how rollout steps and status progressed

If your question is instead about:

- which source revision was built
- what image was produced
- what Tekton pipeline ran
- which workload and service snapshots were frozen for build

then the owning resource is `Manifest`, not `Release`.

## Boundary summary

`Release` is the deploy-side freeze point.

It owns:

- target environment binding
- deployment-time config freeze
- rollout strategy
- rendered deployment bundle
- deployment artifact publication metadata
- rollout execution progress and final deployment state

It does not own:

- source/build selection as the primary record
- image build execution history
- Tekton build topology
- the canonical build-side snapshots owned by `Manifest`

## Relationship with Manifest

`Manifest` and `Release` have different responsibilities and should stay separate.

### Manifest owns

- the durable build record
- source/build metadata
- image result
- `services_snapshot`
- `workload_config_snapshot`

### Release owns

- environment-specific deployment execution
- `app_config_snapshot`
- optional/deferred route inputs when route flow is enabled
- strategy selection
- Kubernetes YAML rendering
- OCI packaging for the deployment bundle
- Argo CD `Application` creation
- rollout status tracking

Conclusion:

- `Manifest` should not own release artifact packaging
- `Manifest` should not own environment-specific rendered YAML
- `Release` should consume the frozen build outputs and workload snapshots already stored on `Manifest`

## Create request contract

Current recommended create request:

```json
{
  "manifest_id": "11111111-1111-1111-1111-111111111111",
  "environment_id": "b780ca97-a213-4763-bfb9-43f7e3a11ee7",
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

### `bundle_digest` vs `artifact_digest`

- `bundle_digest` belongs to the rendered release bundle fact
- `artifact_digest` belongs to the published OCI artifact result

They should not be treated as the same field:

- `bundle_digest`: digest of the rendered bundle content itself
- `artifact_digest`: digest reported by the publisher after bundle publication

## Dependency inputs

`Release` is release-owned, but it composes frozen and live inputs from multiple sources.

### Manifest-side frozen inputs

From persisted `Manifest`:

- built workload image output
- `services_snapshot`
- `workload_config_snapshot`
- source/build metadata for traceability

### Release-time live inputs

From `config-service`:

- `app_config_snapshot`

From `network-service`:

- `routes_snapshot`

From `meta-service`:

- target application, environment, cluster, and deploy-target metadata

## Common base fields

| Field | Type | Required | Writable | Description |
|---|---|---|---|---|
| `id` | `uuid.UUID` | server-generated | no | 主键 |
| `created_at` | `time.Time` | server-generated | no | 创建时间 |
| `updated_at` | `time.Time` | server-generated | no | 更新时间 |
| `deleted_at` | `*time.Time` | optional | system-managed | 软删除时间 |

## Frozen boundary

The key contract of `Release` is that it freezes deployment-time inputs for one target environment.

Frozen on release:

- `manifest_id`
- `environment_id`
- `app_config_snapshot`
- `routes_snapshot`
- chosen rollout `strategy`
- execution `steps`

Produced by release execution after freeze:

- rendered bundle facts
- deployment artifact metadata
- Argo CD external reference
- rollout status transitions

## Field table

### Stable release contract fields

| Field | Type | Required | Writable | Description |
|---|---|---|---|---|
| `execution_intent_id` | `*uuid.UUID` | system-managed | no | 关联执行意图 |
| `application_id` | `uuid.UUID` | system-managed | no | 关联应用 ID |
| `manifest_id` | `uuid.UUID` | required | user | 关联 manifest |
| `environment_id` | `string` | required | user | 目标环境标识；当前实现要求传入有效环境 UUID 字符串 |
| `routes_snapshot` | `[]ReleaseRoute` | system-managed | no | deferred/optional route snapshot；仅在 route 流程重新纳入主交付链路时使用 |
| `app_config_snapshot` | `ReleaseAppConfig` | system-managed | no | release 创建时冻结的 app config 快照 |
| `strategy` | `string` | required | user | 本次发布选择的 rollout 策略 |
| `steps` | `[]ReleaseStep` | system-managed | no | 发布步骤，使用稳定 `code` 标识每个步骤 |
| `status` | `ReleaseStatus` | system-managed | no | 发布状态 |
| `argocd_application_name` | `string` | system-managed | no | Argo CD `Application` 名称 |
| `external_ref` | `string` | system-managed | no | 外部系统引用，例如 ArgoCD Application 名称 |

### Deployment artifact fields recommended for release ownership

These fields describe the published deployment artifact associated with the rendered release bundle and should belong to `Release`, not `Manifest`.

| Field | Type | Required | Writable | Description |
|---|---|---|---|---|
| `artifact_repository` | `string` | system-managed | no | 发布 YAML bundle 所在 OCI repository |
| `artifact_tag` | `string` | system-managed | no | 发布 bundle tag |
| `artifact_digest` | `string` | system-managed | no | 发布 bundle digest |
| `artifact_ref` | `string` | system-managed | no | 完整 OCI 引用 |

## Output boundary

`Release` owns deploy-side outputs.

Primary outputs:

- rendered deployment bundle
- `artifact_repository`
- `artifact_tag`
- `artifact_digest`
- `artifact_ref`
- `argocd_application_name`
- `external_ref`
- release `status` and step progress
- derived `bundle_summary`

The rendered deployment bundle is a release-owned publishable artifact.
It is different from `Manifest` resource inspection views:

- manifest resources are derived read-side projections built only from manifest-frozen snapshots plus the workload image
- release bundles additionally include release-time config, route, strategy, and environment-targeted rendering decisions
- only the release bundle is packaged for OCI publication and deployment

It consumes `manifest.image_ref`, but it does not replace or duplicate `Manifest` as the build record.

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
- `Syncing`
- `SyncFailed`

Current code still exposes `Syncing` / `SyncFailed` as top-level release statuses, so docs should treat them as part of the active contract.

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
- callback senders and release-service should update steps by `code`, not by display name
- writeback payloads should prefer `step_code`; `step_name` is migration-only compatibility input

## Question routing

Use `Manifest` when the question starts with:

- what was built
- which commit was built
- which image came out of build
- what build-time service/workload shape was frozen

Use `Release` when the question starts with:

- what was deployed
- where it was deployed
- which config was used for deployment
- what OCI deployment artifact was published
- what happened during rollout

## Read surfaces

Main operator read surfaces:

- release list/detail
- bundle preview for rendered deployment output
- step/status tracking during rollout

The important read split is:

- `Manifest` answers build-side questions
- `Release` answers deploy-side questions

## API surface

Service-internal route surface:

- `POST /api/v1/releases`
- `GET /api/v1/releases?application_id=...&environment_id=...`
- `GET /api/v1/releases/{id}`
- `GET /api/v1/releases/{id}/bundle-preview`
- `DELETE /api/v1/releases/{id}`
- `POST /api/v1/verify/argo/events`
- `POST /api/v1/verify/release/steps`
- `POST /api/v1/verify/release/artifact`

Pre-production shared ingress external surface:

- `POST /api/v1/release/releases`
- `GET /api/v1/release/releases?application_id=...&environment_id=...`
- `GET /api/v1/release/releases/{id}`
- `GET /api/v1/release/releases/{id}/bundle-preview`
- `DELETE /api/v1/release/releases/{id}`
- `POST /api/v1/release/verify/argo/events`
- `POST /api/v1/release/verify/release/steps`
- `POST /api/v1/release/verify/release/artifact`

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
- `app_config_snapshot`
- optional/deferred `routes_snapshot` when route flow is enabled
- `steps`
- `artifact_repository`
- `artifact_tag`
- `artifact_digest`
- `artifact_ref`
- `external_ref`
- `created_at`
- `updated_at`

Recommended detail-read extension when bundle preview is available:

- `bundle_summary`

Recommended `bundle_summary` shape:

- `available`
- `namespace`
- `artifact_name`
- `bundle_digest`
- `primary_workload_kind`
- `resource_counts`
- `artifact_ref`
- `rendered_at`
- optional `published_at`

Recommended list filters:

- `application_id`
- `environment_id`
- `manifest_id`
- `status`
- `type`
- `include_deleted`

Validation note:

- `GET /api/v1/releases` requires both `application_id` and `environment_id`
- `environment_id` is documented as a string because it is an identifier field, but the current implementation requires a valid environment UUID string

## Bundle preview contract

`GET /api/v1/releases/{id}/bundle-preview` is the operator-facing read surface for one release's rendered deployment content.

Current behavior:

- the preview reads from:
  - persisted `Release`
  - persisted `Manifest`
  - persisted `release_bundle`
- the active implementation now persists one canonical `release_bundle` row per release after `render_deployment_bundle` succeeds

Target contract:

- one `Release` should map to one canonical rendered bundle fact
- the preview should answer both:
  - which frozen inputs were used
  - what YAML was finally rendered

Recommended status semantics:

- `200` when rendered bundle content is available
- `404` when the release does not exist
- `409` with error code `failed_precondition` and message `bundle not ready` when `render_deployment_bundle` has not succeeded yet

Recommended response sections:

- `release_id`
- `manifest_id`
- `application_id`
- `environment_id`
- `strategy`
- `namespace`
- `bundle_digest`
- optional published `artifact`
- `frozen_inputs`
- `rendered_bundle`
- `rendered_at`
- optional `published_at`

Recommended `bundle_summary` example on `GET /api/v1/releases/{id}`:

```json
{
  "bundle_summary": {
    "available": true,
    "namespace": "checkout",
    "artifact_name": "demo-api",
    "bundle_digest": "sha256:bundle",
    "primary_workload_kind": "Rollout",
    "resource_counts": {
      "configmaps": 1,
      "services": 2,
      "rollouts": 1,
      "virtualservices": 1,
      "total": 5
    },
    "artifact": {
      "repository": "zot.zot.svc.cluster.local:5000/devflow/releases/demo-api",
      "tag": "95cccbf1-2e15-4a08-ad39-94019f59edea",
      "digest": "sha256:artifact",
      "ref": "oci://zot.zot.svc.cluster.local:5000/devflow/releases/demo-api@sha256:artifact"
    },
    "rendered_at": "2026-04-29T10:00:00Z",
    "published_at": "2026-04-29T10:00:12Z"
  }
}
```

### `frozen_inputs`

This section should expose the input-side facts for the release, not the rendered Kubernetes objects.

Recommended contents:

- `manifest_summary`
- `services`
- `workload`
- `app_config`
- `routes`

Rules:

- `services` and `workload` come from frozen manifest snapshots
- `app_config` and `routes` come from release-owned frozen inputs
- this section should not contain derived rendered fields such as selectors, rollout strategy blocks, or namespace-injected metadata

### `rendered_bundle`

This section should expose the final rendered output for the same release.

Recommended contents:

- `resource_groups`
- `rendered_resources`
- `files`

Rules:

- `resource_groups` should only list kinds that actually exist in the rendered bundle
- `rendered_resources` should carry one final YAML string per rendered object
- `rendered_resources[].summary` should vary by `kind`
- `files` should include the combined `bundle.yaml`

### Recommended `release_bundle` persistence model

If bundle preview becomes a persisted release-owned fact, the resource should stay one-to-one with `Release`.

Recommended persisted facts:

- `release_id`
- `namespace`
- `artifact_name`
- `bundle_digest`
- `rendered_objects`
- `bundle_yaml`
- `created_at`

Recommended non-goals for this resource:

- do not duplicate `app_config_snapshot`
- do not duplicate `routes_snapshot`
- do not duplicate manifest-side `services` or `workload`
- do not move `artifact_repository`, `artifact_tag`, `artifact_digest`, or `artifact_ref` off `Release`

## Execution model

Release execution is long-running work, but the exact handoff shape depends on runtime mode.

Current repo facts:

- `POST /api/v1/releases` always validates the request, freezes release-time inputs, and creates the release record
- when intent mode is enabled, the flow may also create/schedule an execution intent
- in the default path, `release-service` may continue directly into dispatch work before the HTTP response returns

`POST /api/v1/releases` should therefore not be interpreted as proof that deployment has completed, even if create returned successfully.

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
  "environment_id": "b780ca97-a213-4763-bfb9-43f7e3a11ee7",
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

- accept manifest build output only when status is `Available`
- older observer payloads such as `Ready` / `Succeeded` are normalized onto `Available` at writeback ingress
- older observer payloads such as `Failed` are normalized onto `Unavailable` at writeback ingress
- new manifest observers should emit `Available` / `Unavailable` terminal values directly
- legacy aliases should be treated as migration compatibility only, not as the long-term manifest contract

## 3. Service freezes rollout-time inputs

release-service freezes:

- `app_config_snapshot`
- `routes_snapshot`

It also reads from manifest:

- `image_ref`
- `services_snapshot`
- `workload_config_snapshot`

Together these form the immutable deployment input set for the release. In the current mainline flow, `AppConfig` is active input; `Route` inputs remain deferred unless that workflow is explicitly re-enabled.

## 3.5 Service creates the release record

After validation and snapshot freeze, release-service should:

- persist the release record
- initialize release steps
- set initial `status`, typically `Pending`
- optionally create/schedule execution intent when intent mode is enabled

Current implementation note:

- release creation does not universally stop here
- in the default path, `release-service` continues into dispatch work during the create flow
- intent creation is mode-dependent, not a guaranteed always-on phase

## 4. Release execution renders deployment bundle

The active release execution path renders environment-specific Kubernetes resources from:

### Inputs from Manifest

- `image_ref`
- `services_snapshot`
- `workload_config_snapshot`

### Inputs from Release

- `environment_id`
- `app_config_snapshot`
- optional/deferred `routes_snapshot`
- `strategy`

### Typical outputs

- `ConfigMap`
- `Service`
- `Deployment` or `Rollout`
- optional route/traffic resources when route flow is enabled

The output of this phase is a deployment bundle, not a manifest resource record.

This rendering responsibility belongs to release-service's deployment execution flow, not manifest.

## 5. Release execution uploads deployment bundle to OCI

After rendering the deployment bundle:

- package the deployment YAML bundle
- push it to OCI
- persist:
  - `artifact_repository`
  - `artifact_tag`
  - `artifact_digest`
  - `artifact_ref`

This deployment bundle is the release artifact.

Current implementation target:

- in `oras` mode, the rendered bundle is packaged as one OCI artifact with a **single tar.gz layer**
- that layer contains the rendered bundle files, including `bundle.yaml`
- the preferred pre-production registry endpoint is the in-cluster `zot` service address, not an individual pod DNS name
- the bundle publication location is represented by:
  - `artifact_repository`
  - `artifact_tag`
  - `artifact_digest`
  - `artifact_ref`

Why single-layer packaging matters:

- ArgoCD OCI source is expected to consume one OCI artifact payload as the deployment source
- release execution should not publish one OCI layer per YAML file and then hope ArgoCD reassembles them
- the deployment bundle must be a stable artifact that ArgoCD can pull as-is

## 6. Release execution creates ArgoCD Application

The active release execution flow creates an ArgoCD Application that points to the OCI bundle.

ArgoCD should pull:

- deployment bundle YAML from OCI

not regenerate manifests from scratch, and not depend on manifest-era artifact ownership.

Operational rule:

- ArgoCD source should point to the release-owned OCI artifact by `repoURL + targetRevision`
- `repoURL` should be `oci://<registry>/<namespace>/<repository-prefix>/<application>`
- `targetRevision` should prefer the published digest when available

When the OCI registry is exposed only through in-cluster HTTP, ArgoCD repository configuration must enable OCI force-http semantics for that registry prefix.

## 7. ArgoCD starts deployment

ArgoCD syncs the bundle into the target environment.

Depending on strategy:

- rolling deploys a standard workload update
- blueGreen manages preview/active switch
- canary performs staged traffic progression

## 8. rollout callbacks update release progress

Release progress after Argo CD handoff is represented on the release record through release-owned writeback routes:

- release top-level `status`
- release `steps`
- progress messages
- failure reasons
- deployment artifact metadata

Current implementation facts:

- `release-service` exposes token-gated callback routes under `/api/v1/verify/...`
- `release-service` itself advances several steps synchronously during create/dispatch
- when `runtime-service` starts with in-cluster Kubernetes wiring and release writeback configuration, it starts the in-tree rollout observer under `internal/runtime/observer/release_rollout.go`
- rollout writeback should still be described as a release-owned callback contract: `release-service` owns release truth and callback routes, while `runtime-service` is one active clustered sender rather than the owner of release state

Possible callback sources may include:

- Argo-side phase callbacks
- external executors
- `runtime-service` rollout observers in clustered startup paths with Kubernetes wiring
- future callback senders explicitly wired into startup

## 9. Release remains the durable deployment record

Even after transient runtime objects change or are garbage-collected, the release record should preserve:

- what was deployed
- where it was deployed
- which strategy was used
- which YAML bundle was used
- what status/progress the deployment reached

## Create / update rules

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
- `app_config_snapshot`
- optional/deferred `routes_snapshot`
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
- `app_config_snapshot`
- optional/deferred `routes_snapshot`
- `steps`
- `status`
- `external_ref`

## Create response semantics

`POST /api/v1/releases` should be treated as:

- release record created
- dispatch may already have started before the response returns, depending on runtime mode

It should not be interpreted as:

- OCI artifact already published
- ArgoCD Application already created
- deployment already started
- deployment already completed

## Validation notes

- invalid UUID path or query parameters return `invalid_argument`
- missing records return `not_found`
- create-time environment readiness or deploy-target readiness problems return `failed_precondition`
- release 创建时应冻结当前 `app_config`
- route inputs are currently deferred in the mainline release workflow; if re-enabled later, freeze them as optional release-owned deployment inputs
- release 渲染 deployment bundle 时应使用 manifest 中已经冻结好的 workload/service/image 信息
- release 的 environment 语义应该由 `release.environment_id` 明确表达，而不是由 build-time image metadata 反推
- ArgoCD source 应该消费 release 产出的 OCI deployment bundle
- release create 是否快速返回取决于当前运行模式；默认实现会在 create 流程内继续做 dispatch

## Responsibility split by phase

### release-service synchronous create phase

- validate request
- resolve manifest/environment binding
- freeze `app_config_snapshot`
- freeze optional/deferred `routes_snapshot` only when route flow is enabled
- persist release
- initialize steps and status
- optionally create execution intent when intent mode is enabled

### release execution phase

- `render_deployment_bundle`
- `publish_bundle`
- `create_argocd_application`
- trigger deployment start

Current implementation facts:

- when intent mode is enabled, release intents are claimed by kind = `release`
- the normal dispatch path runs through release-service orchestration
- the dispatch flow is split into explicit phases:
  - `render_deployment_bundle`
  - `publish_bundle`
  - `create_argocd_application`
- render phase now builds a release-owned in-memory deployment bundle structure from:
  - `manifest.services_snapshot`
  - `manifest.workload_config_snapshot`
  - `manifest.image_ref`
  - `release.app_config_snapshot`
  - optional/deferred `release.routes_snapshot`
- render phase is the point where one canonical release-owned bundle fact is fixed for that release
- publish phase now flows through a bundle publisher abstraction
- publish phase should consume the already-rendered bundle fact instead of introducing a different rendering source of truth
- current default publisher records artifact metadata from rendered bundle content and digest
- publisher modes:
  - `metadata`: metadata-only
  - `oras`: package the rendered bundle as a single OCI tar.gz layer and push/tag it to the remote registry when manifest registry is enabled
- `publish_bundle` step message should prefer carrying both publisher mode and final artifact ref for operator diagnostics
- `create_argocd_application` step message should prefer carrying application name, target environment, and final artifact ref when available
- when intent mode is enabled, the intent is marked as `Running` after dispatch is accepted
- later writeback callbacks continue updating release status and steps
- when release-service runs in intent mode and worker config is enabled, it starts a background release intent worker loop

### Pre-production OCI wiring note

Committed pre-production release deployment currently expects:

- `release-service` publishes bundle artifacts to `zot.zot.svc.cluster.local:5000`
- `manifest_registry.mode = oras`
- Argo CD uses a repo-creds prefix secret for `oci://zot.zot.svc.cluster.local:5000/devflow/releases`

This prefix-based ArgoCD credential contract is required because release artifact repositories are application-scoped beneath the `releases/` prefix.

### rollout callback phase

- report ArgoCD / rollout progress through release-owned callback routes when callback senders are wired
- update step status by `code`
- update top-level release status
- persist rollout messages and failures
- finalize release outcome

Current implementation note:

- the repository contains rollout observer code, and the clustered `runtime-service` startup path now starts it when in-cluster config and release writeback wiring are available
- treat this as a callback contract owned by `release-service`: runtime-side observers are active senders in supported clustered paths, but callback availability still depends on environment wiring rather than being guaranteed in every runtime mode

## Step naming guidance

Detailed operator-facing step semantics now live in:

- `docs/system/release-steps.md`

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
- `ensure_namespace`
- `ensure_pull_secret`
- `ensure_appproject_destination`
- `render_deployment_bundle`
- `publish_bundle`
- `create_argocd_application`
- `start_deployment`
- `observe_rollout`
- `finalize_release`

Suggested display names:

- `freeze_inputs` -> `Freeze release inputs`
- `ensure_namespace` -> `Ensure namespace`
- `ensure_pull_secret` -> `Ensure pull secret`
- `ensure_appproject_destination` -> `Ensure AppProject destination`
- `render_deployment_bundle` -> `Render deployment bundle`
- `publish_bundle` -> `Publish bundle to OCI`
- `create_argocd_application` -> `Create ArgoCD application`
- `start_deployment` -> `Start deployment`
- `observe_rollout` -> `Observe rollout`
- `finalize_release` -> `Finalize release`

### blueGreen

- `freeze_inputs`
- `ensure_namespace`
- `ensure_pull_secret`
- `ensure_appproject_destination`
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
- `ensure_namespace`
- `ensure_pull_secret`
- `ensure_appproject_destination`
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

### 2. release execution steps

These are advanced by the active release execution path before or during deployment handoff.

Common release-execution-owned steps:

- `ensure_namespace`
- `ensure_pull_secret`
- `ensure_appproject_destination`
- `render_deployment_bundle`
- `publish_bundle`
- `create_argocd_application`

Rolling deployments may also include:

- `start_deployment`

### 3. rollout callback steps

These steps are intended to be advanced through release-owned callback routes after deployment has been handed to external systems.

Current implementation note:

- the step schema exists now
- `release-service` accepts callback updates now
- the clustered `runtime-service` startup path can now auto-start the in-tree release rollout observer when Kubernetes config and release writeback wiring are available, so runtime-side observers are one active callback sender rather than a disabled code path

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
| `ensure_namespace` | release execution path | 确保目标 namespace 已存在 |
| `ensure_pull_secret` | release execution path | 确保目标 namespace 已具备拉镜像 secret |
| `ensure_appproject_destination` | release execution path | 确保 ArgoCD AppProject 放通目标集群与 namespace |
| `render_deployment_bundle` | release-service dispatch | 渲染 deployment bundle |
| `publish_bundle` | release-service dispatch / artifact callback | 上传 deployment bundle 到 OCI |
| `create_argocd_application` | release-service dispatch | 创建 ArgoCD Application |
| `start_deployment` | release-service dispatch / rollout callback | 触发 rolling deployment 开始 |
| `observe_rollout` | rollout callback | 跟踪 rolling rollout |
| `deploy_preview` | rollout callback | blueGreen preview 部署阶段 |
| `observe_preview` | rollout callback | 观察 blueGreen preview 健康状态 |
| `switch_traffic` | rollout callback | blueGreen 切流 |
| `verify_active` | rollout callback | blueGreen active 验证 |
| `deploy_canary` | rollout callback | canary 初始部署 |
| `canary_10` | rollout callback | canary 10% 流量阶段 |
| `canary_30` | rollout callback | canary 30% 流量阶段 |
| `canary_60` | rollout callback | canary 60% 流量阶段 |
| `canary_100` | rollout callback | canary 100% 流量阶段 |
| `finalize_release` | rollout callback | 收口 release 成功/失败终态 |

## Step initialization rules

When a release record is first created:

- initialize the complete strategy-specific step list immediately
- assign stable `code` values to every step
- initialize all step `status` values as `Pending`
- initialize all step `progress` values as `0`

Then:

- release-service marks `freeze_inputs` as `Succeeded` once snapshot freeze completes
- release-service dispatch currently advances render/publish/Argo creation steps during normal create flow
- callback senders advance later rollout steps through release-owned writeback routes

## Strategy-specific lifecycle templates

### rolling lifecycle

#### create phase

- `freeze_inputs` -> `Succeeded`

#### release execution phase

- `ensure_namespace`
- `ensure_pull_secret`
- `ensure_appproject_destination`
- `render_deployment_bundle`
- `publish_bundle`
- `create_argocd_application`
- `start_deployment`

#### rollout callback phase

- `observe_rollout`
- `finalize_release`

### blueGreen lifecycle

#### create phase

- `freeze_inputs` -> `Succeeded`

#### release execution phase

- `ensure_namespace`
- `ensure_pull_secret`
- `ensure_appproject_destination`
- `render_deployment_bundle`
- `publish_bundle`
- `create_argocd_application`

#### rollout callback phase

- `deploy_preview`
- `observe_preview`
- `switch_traffic`
- `verify_active`
- `finalize_release`

### canary lifecycle

#### create phase

- `freeze_inputs` -> `Succeeded`

#### release execution phase

- `ensure_namespace`
- `ensure_pull_secret`
- `ensure_appproject_destination`
- `render_deployment_bundle`
- `publish_bundle`
- `create_argocd_application`

#### rollout callback phase

- `deploy_canary`
- `canary_10`
- `canary_30`
- `canary_60`
- `canary_100`
- `finalize_release`

## Top-level status progression

Recommended top-level status semantics:

- `Pending` -> release record created, execution not yet completed
- `Running` -> release dispatch or rollout callback has started advancing deployment
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
