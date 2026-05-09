# Runtime Service

## 这个文档解决什么问题

这份文档说明 `runtime-service` 当前拥有哪些运行时职责，以及它和 `release-service` 的真相边界如何分开。

读完后，读者应该能回答：

- `runtime-service` 当前对外暴露哪些 operator-facing 能力
- 它为什么不是一个普通 CRUD 服务
- 它观察 rollout，但为什么不拥有 `Release` 真相

## Reader routing

如果你要先看完整发布链路，先读 `docs/system/flow-overview.md`。

这份文档只解释 runtime-owned 的那一半，主要是：

- stage 7：runtime observation 和 release writeback
- stage 8：runtime operator actions

要看 `Release` 真相、Argo handoff 或 writeback route ownership，请跳到：

- `docs/resources/release.md`
- `docs/services/release-service.md`
- `docs/system/release-writeback.md`

## Purpose

`runtime-service` 负责 runtime desired state、runtime revisions、runtime observed index state，以及直接的 runtime operations。

这个服务对外最重要的价值不是数据库 CRUD。
它真正提供的是：

- 对 live Kubernetes workload 的运行态查看
- 对 `application + environment` 粒度的运行态控制

## Planned note

- `telemetry-service` 不是当前 runtime 的强依赖
- 当前 tracing、metrics、profiling 仍是平台能力
- 如果图里出现 `telemetry-service`，只能按 planned 理解

## Owns

- `RuntimeSpec`
- `RuntimeSpecRevision`
- `RuntimeObservedPod`
- `RuntimeObservedWorkload`
- `RuntimeOperation`
- runtime desired state for `application + environment`
- immutable runtime revisions
- live runtime observation responsibilities
- direct K8s pod lifecycle operations

## Does Not Own

- `Project`
- `Application`
- `ApplicationEnvironment`
- `Cluster`
- `Environment`
- `AppConfig`
- `WorkloadConfig`
- `Service`
- `Route`
- `Manifest`
- `Image`
- `Release`
- `Intent`

## Dependency model

### Target dependency model

对 operator-facing runtime API，应把 `runtime-service` 理解为依赖：

- runtime observer / index
- Kubernetes API
- shared backend primitives

这是理解下面几类流程的核心心智模型：

- show one application workload overview
- list application pod status
- delete one pod
- trigger one rollout / restart

读写分离规则：

- read surfaces should prefer runtime-owned observed index data
- action surfaces should call Kubernetes only when an operator explicitly performs an action
- read surfaces must fail explicitly when observer/index truth cannot resolve exactly one runtime target

### Current implementation note

当前 active contract 是 Kubernetes-first：

- runtime reads come from runtime-owned observed state
- runtime actions mutate Kubernetes directly
- the default runtime HTTP service uses `internal/runtime/repository.RuntimeStore`, which currently defaults to the in-memory store
- the default runtime read path is rebuilt through observer sync rather than by loading runtime rows from PostgreSQL at boot

当前必须特别注意：

- the runtime index is not durable local storage inside `runtime-service`
- after restart, runtime state is expected to be rebuilt by the in-process observers
- release-owned Kubernetes metadata is the runtime identity contract: rendered workloads, pod templates, and the Argo CD `Application` handoff object must all carry `app.kubernetes.io/name`, `devflow.io/release-id`, `devflow.application/id`, and `devflow.environment/id`
- runtime-service consumes those labels as the authoritative release/application/environment lookup surface; it must not require annotations for identity recovery
- Argo CD `Application` annotations are reserved for supplementary tracing context such as trace/span correlation during handoff diagnostics
- runtime-service active/runtime-domain storage is PostgreSQL-free
- shared platform startup outside `cmd/runtime-service` may still open PostgreSQL for other services
- release rollout observation is also started by the active runtime startup path, but it consumes the same in-memory runtime observer state instead of a runtime-domain PostgreSQL store
- when release writeback wiring is present, that rollout observer is a callback sender into `release-service`; it does not become the owner of release status, release steps, or writeback route policy
- release/application/environment metadata and inspection surfaces remain compatible with both `Deployment` and `Rollout`, and the active in-tree runtime rollout observer now derives live rollout progress from the observed workload kind
- once `finalize_release` closes a release, runtime-side late callbacks must not rewrite top-level terminal truth or overwrite already-finalized callback-owned step details

不要把当前 runtime contract 误读成“全仓库已经不再使用 PostgreSQL”。
当前只是在 operator-facing 的 runtime workload / pod 路径上，默认采用 observer/index-backed、memory-backed 的方式。

## Read and action split

`runtime-service` 应当被看成两个相关但分离的 surface：

### Read surface

- workload overview
- pod list
- backed by runtime-owned observer/index state

### Action surface

- delete pod
- rollout / restart workload
- calls Kubernetes only when an operator explicitly requests a mutation

这组读写分离是 `runtime-service` 最重要的契约，比内部存储模型名称更重要。

## Downstream Consumers

- platform orchestration layers
- release-time consumers

## Entrypoint

Primary runnable entrypoint: `cmd/runtime-service/main.go`.

```text
cmd/runtime-service/main.go
```

## Registered Domains

```text
internal/runtime/domain
internal/runtime/repository
internal/runtime/service
internal/runtime/transport/http
```

## Pre-production Shared Ingress

- `/api/v1/runtime/...`

Internal observer callbacks are service-internal only and are not part of the shared-ingress external contract.

## Primary operator flows

### 1. Show workload overview

runtime service 接收：

- `application_id`
- `environment_id`

然后它应该：

1. resolve the target runtime binding from observer/index-owned identity
2. read the latest observed workload summary from runtime-owned index storage
3. return one workload overview for that `application + environment`

这是 controller-level 的 read surface。

失败约定：

- if no observer-owned runtime identity exists, return `not_found` (`ErrRuntimeIdentityMissing`)
- if namespace resolution or workload targeting cannot be resolved truthfully, return `failed_precondition`
- if more than one Deployment remains after release-owned label correlation, return `failed_precondition` (`ErrRuntimeWorkloadAmbiguous`) instead of picking one

### 2. List pod status

runtime service 接收：

- `application_id`
- `environment_id`

然后它应该：

1. resolve the target runtime binding from observer/index-owned identity
2. read the latest observed pod list from runtime-owned index storage
3. return current pod status for that application runtime

这是 instance-level 的 read surface。

失败约定：

- if no observer-owned runtime identity exists, return `not_found` (`ErrRuntimeIdentityMissing`)
- if the runtime target exists conceptually but the namespace cannot be derived truthfully, return `failed_precondition` (`ErrRuntimeNamespaceUnresolved`)
- do not convert missing observer state into a successful empty pod list

### 3. Delete one pod

runtime service 接收：

- `application_id`
- `environment_id`
- `pod_name`

然后它应该：

1. resolve the runtime namespace from runtime identity plus observer/index state
2. verify the requested pod is present in observer-backed pod state for that exact target
3. delete that pod in Kubernetes only after the target is confirmed truthfully
4. let the owning controller recreate or rebalance it

失败约定：

- if no observer-owned runtime identity exists, return `not_found` (`ErrRuntimeIdentityMissing`)
- if namespace resolution fails or the requested pod is absent from observer-backed state for the resolved target, return `failed_precondition` (`ErrRuntimeNamespaceUnresolved` or `ErrRuntimePodTargetMissing`)
- downstream Kubernetes `not_found` is reserved for resources that disappeared after the runtime target was already resolved confidently

### 4. Trigger rollout / restart

runtime service 接收：

- `application_id`
- `environment_id`

然后它应该：

1. resolve exactly one target Deployment from either an explicit `deployment_name` or the observed workload record when that record is a `Deployment`
2. fail with `failed_precondition` when no confident Deployment target can be derived from observer/index truth
3. patch `kubectl.kubernetes.io/restartedAt`
4. let Kubernetes perform the rolling restart

## External surface status

### Current external surface

- `GET /api/v1/runtime/workload`
- `GET /api/v1/runtime/pods`
- `DELETE /api/v1/runtime/pods/{pod_name}`
- `POST /api/v1/runtime/rollouts`

mutation 路由成功返回时，只表示两件事：

- Kubernetes 接受了这次 delete / restart 请求
- runtime-service 持久化了一条 operation acknowledgement

它**不会**声称 rollout observation 或 release convergence 已经完成。
canonical acknowledgement payload 会保持 `convergence_state=pending_observation`，直到 observer 和 release-owned surface 继续推进。

这里的 action failure mapping 是有意设计的：

- `404 not_found` means the requested `application + environment` has no observer-backed runtime identity yet, or Kubernetes could not find a resource after a valid target was already resolved
- `412 failed_precondition` means runtime-service refused to guess because namespace, workload, or pod targeting could not be resolved confidently from observer/index truth

### Runtime read-model surface

当前 reader-facing runtime contract 故意比完整的 release metadata contract 更窄：

- runtime readers consume release/application/environment identity from labels, not annotations
- annotations remain supplementary diagnostics only
- metadata and inspection surfaces stay compatible with both `Deployment` and `Rollout`
- the active in-tree runtime observer still derives live rollout progress from `Deployment` objects only today, so ambiguous or non-Deployment rollout targeting must fail explicitly instead of guessing

Runtime workload overview now uses:

- `GET /api/v1/runtime/workload?application_id=...&environment_id=...`

这个 endpoint 应该：

- 从和 runtime pod display 相同的 observer/index model 返回 workload overview
- 不要在每次页面加载时都直接查询 Kubernetes
- 当 runtime target 不能从 observer/index state 和 release-owned labels 中被真实解析出来时，明确失败，而不是乐观返回成功

响应重点建议：

- workload identity: `workload_kind`, `workload_name`, `namespace`
- replica status: `desired_replicas`, `ready_replicas`, `updated_replicas`, `available_replicas`, `unavailable_replicas`
- rollout health: `summary_status`, `conditions[]`, `observed_generation`
- deployment content summary: `images[]`
- timestamps: `observed_at`, optional `restart_at`

推荐的 UI 视角切分是：

- `runtime/workload` for controller-level summary
- `runtime/pods` for pod-level details
- runtime actions for explicit Kubernetes mutations

## Internal observer surface

Observer-side internal routes now include:

- `POST /api/v1/internal/runtime-workloads/sync`
- `POST /api/v1/internal/runtime-workloads/delete`
- `POST /api/v1/internal/runtime-pods/sync`
- `POST /api/v1/internal/runtime-pods/delete`

这些路由只用于 observer/index writeback。
它们不是 user-facing API route。

鉴权说明：

- these internal routes are protected by `X-Devflow-Observer-Token` when `observer.shared_token` is configured
- when `observer.shared_token` is empty, the middleware allows the request through

## Current storage reality

当前代码应分两层理解：

- operator-facing runtime read/action endpoints use the runtime service default store, which currently points at the in-memory `RuntimeStore`
- runtime-service active/runtime-domain storage is PostgreSQL-free, even though shared platform startup outside `cmd/runtime-service` may still open PostgreSQL for other services
- release rollout observer startup is active in `internal/runtime/config/config.go`, and that observer consumes runtime observer state plus Kubernetes labels rather than a runtime-domain PostgreSQL store

更细的存储模型说明，请看 `docs/system/runtime-storage-model.md`。

## Resource Contracts

- `docs/resources/runtime-spec.md`

## Diagnostics

- `internal/runtime/transport/http/router.go`
- `internal/runtime/transport/http/handler.go`
- `internal/runtime/service/service.go`
- `internal/runtime/repository/repository.go`
- `internal/runtime/observer`
- `docs/system/release-writeback.md`

Runtime endpoints:

- `/healthz`
- `/readyz`
- `/internal/status`

## Canonical pre-production operator proof route

把这里当作当前 pre-production contract 的单一 operator-facing proof walk。
它的作用是把原本分散的诊断入口收敛成一条更清晰的验证路径。

Assumptions to keep explicit:

- `runtime-service` owns `/api/v1/runtime/...` without ingress rewrite
- runtime observation runs in-process inside `runtime-service`
- accepted runtime mutations report acknowledgement first and keep `convergence_state=pending_observation` until observer and release truth advance

Committed deployment anchors:

1. `kubectl apply -f deployments/pre-production/release-service.yaml`
2. `kubectl apply -f deployments/pre-production/runtime-service.yaml`
3. `kubectl apply -f deployments/pre-production/istio/shared-ingress.yaml`

Canonical host and routes:

- host: `https://devflow-pre-production.bei.com`
- reads: `GET /api/v1/runtime/workload`, `GET /api/v1/runtime/pods`
- actions: `POST /api/v1/runtime/rollouts`, `DELETE /api/v1/runtime/pods/{pod_name}`

Canonical inspection order after an accepted action:

1. verify the runtime acknowledgement payload and target metadata
2. confirm the action stayed at `convergence_state=pending_observation` until rollout observation advances
3. inspect runtime observer progress before blaming release convergence
4. then inspect release-service normalization of `observe_rollout` and `finalize_release`
5. only then treat release-status convergence as the remaining layer

## Verification

```sh
go test ./internal/runtime/service ./internal/runtime/transport/http ./internal/runtime/observer
go build -o bin/runtime-service ./cmd/runtime-service
bash scripts/verify.sh
```

Lookup-contract proof lives in:

- `internal/runtime/service/service_test.go` — service-layer truthy lookup failures for missing observer identity, unresolved namespace, ambiguous workload selection, and observer-backed pod-target enforcement
- `internal/runtime/transport/http/handler_test.go` — HTTP mapping proof for `404 not_found` versus `412 failed_precondition` on read and action routes
- `internal/runtime/observer/release_rollout_test.go` — observer-side release-owned label correlation and ambiguity handling
