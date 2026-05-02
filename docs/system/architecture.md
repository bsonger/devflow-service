# Architecture

## 这个文档解决什么问题

这份文档解释 `devflow-service` 当前的系统结构。

读完后，读者应该能回答：

- 当前仓库里有哪些真实可运行服务
- 当前服务名和目标边界名有什么差别
- 各服务分别拥有哪些边界
- 核心资源链路是怎么从 metadata 走到 release 和 runtime 的
- 哪些能力是当前事实，哪些只是 planned

## 当前仓库角色

`devflow-service` 是当前 DevFlow 后端的 Go monorepo。

当前本地事实：

- `meta-service` 仍然是当前迁移主线
- `config-service`、`network-service`、`release-service`、`runtime-service` 已经都有独立入口
- `release-service` 是当前最主要的跨服务组合边界
- `runtime-service` 是 Kubernetes-facing 的运行时读写与观察边界

## 当前服务边界

| 当前可运行服务 | 当前边界 | 说明 |
|---|---|---|
| `meta-service` | 元数据边界 | 当前拥有 `Project`、`Application`、`ApplicationEnvironment`、`Cluster`、`Environment` |
| `config-service` | 配置边界 | 当前拥有 `AppConfig`、`WorkloadConfig` |
| `network-service` | 网络边界 | 当前拥有 `Service`、`Route` |
| `release-service` | 发布边界 | 当前拥有 `Manifest`、`Release`、`Intent`，并吸收 verify writeback |
| `runtime-service` | 运行时边界 | 当前拥有 runtime read model、operator actions、observer callback |

## 目标命名与当前事实的映射

为了避免文档和设计讨论混淆，必须把下面几组词分开理解：

### `application-service`

- 这是“应用元数据边界”的目标命名或概念命名
- 当前仓库里没有独立可运行的 `application-service`
- 当前这部分事实仍然由 `meta-service` 提供

### `verify-service`

- 这不是当前独立服务
- 以前的 verify ingress / writeback 语义现在已经并入 `release-service`
- 所以任何当前 callback / writeback 说明都应优先落到 `release-service`

### `telemetry-service`

- 当前只能标记为 planned
- 可以出现在图里，不能写成当前代码强依赖
- 当前实现里的 tracing / metrics / profiling 仍然是平台能力，不是独立服务边界

## 服务之间如何协作

当前主链路可以简化成：

1. `meta-service` 提供应用、环境、集群等元数据
2. `config-service` 提供配置输入
3. `network-service` 提供服务和路由输入
4. `release-service` 读取这些输入，冻结出 `Manifest` 和 `Release`
5. `runtime-service` 从 Kubernetes 观察运行状态，并把 rollout 进度写回 `release-service`

## 资源关系

核心资源关系以 `docs/system/domain-model.md` 为准。

这里只保留最重要的架构结论：

- `Project -> Application`
- `Application -> ApplicationEnvironment`
- `Application -> WorkloadConfig`
- `Application -> Service`
- `ApplicationEnvironment -> AppConfig`
- `ApplicationEnvironment -> Route`
- `Manifest` 冻结 build-side 输入
- `Release` 冻结 deploy-side 输入

## Manifest 和 Release 的边界

### Manifest

`Manifest` 是发布前冻结的不可变快照。

它冻结：

- `git_revision`
- `commit_hash`
- `services_snapshot`
- `workload_config_snapshot`

它记录：

- Tekton build 状态
- workload image 输出

它不负责：

- 目标环境
- 环境配置
- 对外路由
- rollout 最终状态

### Release

`Release` 是环境相关的 deploy-side 快照。

它冻结：

- `manifest_id`
- `environment_id`
- `app_config_snapshot`
- `routes_snapshot`
- `strategy`

它负责：

- render deployment bundle
- publish bundle to OCI
- create Argo CD `Application`
- 记录 rollout / finalize 状态

## 环境差异如何表达

当前环境差异通过 Overlay / EnvConfig 思路表达，而不是靠业务代码分支。

在当前实现里，可以这样理解：

- `WorkloadConfig`：应用级基线
- `Service`：应用级服务拓扑
- `AppConfig`：环境级配置内容
- `Route`：环境级入口差异
- `Release` render：把这些输入叠加成目标环境的最终部署 bundle

## 当前实现现实

当前仓库虽然已经拆出多个 service entrypoint，但并不意味着所有边界都已经完全通过独立 HTTP 调用隔离。

仍需注意：

- 同一个 Go module 内仍存在同仓实现访问
- `config-service` 和 `network-service` 的部分校验仍有过渡态
- `release-service` 是当前最真实的跨服务组合边界
- `runtime-service` 的 active runtime-domain 路径是 PostgreSQL-free

这部分细节以 `docs/system/current-service-extraction-reality.md` 为准。

## 代码组织

当前代码组织目标：

- 一个根 `go.mod`
- `cmd/<service>/main.go` 只做启动和组装
- `internal/<domain>/domain|service|repository|transport`
- `internal/platform/` 放基础设施能力
- `internal/shared/` 只放小而稳定的通用 helper

当前真实入口包括：

- `cmd/meta-service/main.go`
- `cmd/config-service/main.go`
- `cmd/network-service/main.go`
- `cmd/release-service/main.go`
- `cmd/runtime-service/main.go`

不要在当前迁移阶段引入：

- `go.work`
- 每服务一个 `go.mod`
- 宽泛的 `common/`、`util/`、`base/` 目录

## 文档分层

这份文档是系统级事实层。

配套阅读：

- `docs/system/domain-model.md`
- `docs/system/flow-overview.md`
- `docs/system/current-service-extraction-reality.md`
- `docs/services/README.md`
- `docs/resources/README.md`
- `docs/system/ingress-routing.md`

## Assumptions

- 当前 workspace 中没有 `devflow-control` 的目标架构文档副本，所以这里对 `application-service` 的描述只作为本仓库内的概念映射，不宣称外部未来设计已经定稿
- `telemetry-service` 在当前文档中只可视为 planned，除非未来代码和入口实际落地
