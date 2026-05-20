# Release Service

## 这个文档解决什么问题

这份文档说明 `release-service` 当前拥有哪些发布边界，以及它和 `Manifest`、`Release`、runtime writeback 的关系。

读完后，读者应该能回答：

- 哪些阶段属于 `release-service`
- 为什么 `Manifest` 和 `Release` 都归这个服务，但职责不同
- 历史上的 `verify-service` 现在去了哪里

## Reader routing

如果你要先建立完整发布链路，先看 `docs/system/flow-overview.md`。

这份服务文档聚焦的是 `release-service` 自己拥有的阶段，主要是：

- stage 2：`Manifest` 冻结与构建派发
- stage 3：`Release` 冻结
- stage 4：release bundle render
- stage 5：bundle publish
- stage 6：Argo handoff / deployment start
- stage 7：release 侧 callback surface 与 release truth 持久化

这不是总生命周期总览页。
它是 release-owned stages 的 owner / diagnostics guide。

## Purpose

`release-service` 负责从 build 到 deploy 的交接记录和执行流程。

当前 pre-production / production 拆分下，最短准确描述是：

- pre-production `release-service` creates and owns build-side `Manifest`
- production `release-service` creates and owns production deploy-side `Release`
- production `release-service` may read a missing referenced manifest from the configured manifest source fallback
- production `runtime-service` writes rollout progress back to production `release-service`
- each control plane must set an explicit `observer.control_plane_id`; the value must match between that plane's `release-service` and `runtime-service`

它会：

- 创建 build-side 的 `Manifest`
- 创建 deploy-side 的 `Release`
- 暴露 callback / writeback surface，用来接收部署进度更新

它也是当前仓库里最重要的跨服务 orchestration boundary。

它不拥有上游资源真相，例如：

- application metadata
- app config
- workload config
- services
- routes

但它会把这些上游事实组合成两个 release-owned freeze point：

- `Manifest` for the build-side record and image-delivery trace
- `Release` for the deploy-side environment bind, bundle publication, and rollout state

## Naming note

- `verify-service` 不是当前独立可运行服务
- 当前 verify ingress / writeback contract 已并入 `release-service`
- 所以 callback 路由、鉴权和 writeback 文档都应优先归到 `release-service`

## Owns

- build-side `Manifest`
- workload `Image` result recorded on the manifest
- deploy-side `Release`
- deploy `Intent`
- build and release lifecycle records around image build, deployment bundle render, and deployment bundle publication
- verify ingress and verification writeback responsibilities previously modeled as `verify-service`

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
- `RuntimeSpec`
- `RuntimeSpecRevision`
- `RuntimeObservedPod`
- `RuntimeOperation`

## Dependency model

`release-service` 同时依赖两类输入：

- 自己持久化的 release data
- 其他服务提供的上游事实

### Control-plane and persistence dependencies

- PostgreSQL for persisted `Manifest` and `Release` records
- Tekton for build execution tied back to the build-side `Manifest`
- Argo CD for deploying the deploy-side `Release` bundle
- Kubernetes API
- OCI registry for deploy-side bundle publication in pre-production (`zot`)

历史命名说明：

- the runtime/config surface still uses the legacy `manifest_registry` key and helper names
- in the current code path that naming refers to the registry target for release deployment bundle publication, not ownership of the `Manifest` API resource

### Control-plane federation dependencies

Current control-plane-specific downstream settings:

- `downstream.manifest_source_base_url`
  - used by the owning `release-service` when it needs to read a manifest that is not present in the local store
- `downstream.release_service_base_url`
  - reserved for explicit release-service federation routes; do not infer ownership from namespace naming alone

The active production flow does not move release truth back to pre-production.
Production release records and production callback state belong to production
`release-service`.

### Upstream business dependencies

- `meta-service`
  - application metadata
  - environment metadata
  - cluster metadata
  - deploy target resolution
- `config-service`
  - workload config during manifest creation
  - app config during release creation
- `network-service`
  - service topology during manifest creation
  - route topology during release creation

## Dependency detail by workflow

### Manifest create path

创建 `Manifest` 时，`release-service` 会组合这些上游事实：

1. read application projection from `meta-service`
2. read workload config from `config-service`
3. read service list from `network-service`
4. derive image target and submit Tekton build
5. persist one frozen build-side manifest record in PostgreSQL

这说明：

- `Manifest` 是 release-owned 的 build-side record
- 但它冻结的部分输入来自其他服务

它承担的观察面包括：

- build identity
- 冻结后的 workload / service 输入
- Tekton progress writeback
- 最终 workload image 输出

### Release create path

创建 `Release` 时，`release-service` 会组合这些上游事实：

1. read frozen manifest from release-owned persistence
   - or read it from configured manifest-source fallback when this control plane owns the release but the manifest was built elsewhere
2. read app config from `config-service` when present
3. read route list from `network-service`
4. resolve application / environment / cluster deploy target from `meta-service`
5. freeze those live inputs onto the release row
6. render, publish, and deploy the release bundle

这说明：

- `Release` 是 release-owned 的 deploy-side record
- the control plane that owns the deploy-side target owns the durable `Release` row

它承担的观察面包括：

- environment binding
- rendered deployment bundle facts
- published OCI artifact metadata
- Argo CD handoff
- rollout / writeback status

## Rollout observation boundary

`release-service` 应该被理解为 deployment initiator 和 release-truth owner，而不是 rollout observer。

目标边界应当这样理解：

1. `release-service` creates or updates the Argo CD `Application`
2. Argo CD syncs the release-owned OCI bundle into Kubernetes
3. `runtime-service` may observe rollout state from Kubernetes and send token-gated callbacks when its clustered observer path is wired
4. `release-service` does not poll Argo CD application status during normal release detail reads
5. rollout progress writeback, when used, comes through release-owned writeback routes
6. those writeback routes are part of the release boundary, not a public runtime API surface
7. production rollout writeback targets production `release-service` directly

从系统生命周期文档延续下来的关键提醒：

- `start_deployment` remains the release-service-owned handoff step for rolling releases.
- `observe_rollout` and `finalize_release` remain callback-owned follow-up steps after that handoff.
- when a release reaches a terminal workload state and drops out of the runtime running set, runtime bootstrap/reconcile may still replay that release key once so release-service gets one last callback compensation chance for `observe_rollout` / `finalize_release`
- rolling, blue-green, and canary releases now all use repo-local timeout convergence for stalled running/finalizing phases; when a runtime-affecting strategy stage times out, release truth should converge to `Failed` and record rollback remediation when appropriate
- release/application/environment identity must continue to ride on labels; annotations stay supplementary diagnostics only.
- `devflow.control-plane/id` is part of the runtime ownership contract; observers use it to keep pre-production and production truth separated.
- `devflow.control-plane/id` is sourced from `observer.control_plane_id`; do not derive it from namespace names or ingress hostnames.
- the release metadata and inspection contract stays compatible with both `Deployment` and `Rollout` primary workloads, and the active in-tree runtime observer now writes back rolling, blue-green, and canary rollout progression from the observed workload kind.
- once `finalize_release` closes a release, late callbacks must not rewrite top-level terminal truth or overwrite already-finalized callback-owned step details.

继续深入时，优先看：

- `docs/system/release-writeback.md` for the callback contract
- `docs/system/release-code-map.md` for the current lifecycle code ownership map
- `docs/services/runtime-service.md` for the runtime observer/read-model side of the same seam

## Downstream Consumers

- platform orchestration layers
- verify-time consumers

## Entrypoint

Primary runnable entrypoint: `cmd/release-service/main.go`.

```text
cmd/release-service/main.go
```

## Registered Domains

```text
internal/manifest/
internal/intent/
internal/release/
```

## Pre-production Shared Ingress

- `/api/v1/release/...`

## Resource Contracts

- `docs/resources/manifest.md`
- `docs/resources/image.md`
- `docs/resources/intent.md`
- `docs/resources/release.md`

Operational callback contract:

- `docs/system/release-writeback.md`
- `docs/system/release-steps.md`

## Diagnostics

- `internal/release/transport/http/router.go`
- `internal/manifest/transport/http`
- `internal/intent/transport/http`
- `internal/release/service`
- `internal/release/support`
- `docs/policies/worker-runtime.md`

Runtime endpoints:

- `/healthz`
- `/readyz`
- `/internal/status`

## Pre-production OCI deployment bundle flow

当前 pre-production 的 release execution 路径是：

1. `release-service` renders one canonical deployment bundle for the release.
2. `publish_bundle` packages that bundle as a single OCI tar.gz layer and pushes it to the configured OCI registry.
3. The pre-production committed registry target is the in-cluster `zot` service, not the `zot-0` pod name.
4. `create_argocd_application` creates an Argo CD `Application` whose source points at the published OCI artifact.
5. Argo CD pulls the OCI artifact and syncs it into the target namespace.

当前 release-service 在触发 Argo sync 时会显式要求：

- `prune=true`
- `Replace=true`
- `apply.force=true`

这样做的目的，是避免历史 `apply` 残留的旧字段继续留在 live workload 上，例如过期的 volume、volumeMount、port 或 metadata。

当前提交到仓库里的 pre-production 配置要求：

- `manifest_registry.registry = zot.zot.svc.cluster.local:5000`
- `manifest_registry.namespace = devflow`
- `manifest_registry.repository = releases`
- `manifest_registry.plain_http = true`
- `manifest_registry.mode = oras`

历史命名说明：

- `manifest_registry` is the legacy config block name kept for compatibility
- in the active code path it configures release deployment bundle publication
- it does not mean the registry owns or stores the `Manifest` resource contract itself

由于 release bundle repository path 是 application-scoped、并且挂在 `releases/` 前缀下，Argo CD 更适合使用 repo-creds prefix secret，而不是单条固定 repository 配置。

## Verification

```sh
go test ./internal/manifest/... ./internal/intent/... ./internal/release/...
go build -o bin/release-service ./cmd/release-service
bash scripts/verify.sh
```
