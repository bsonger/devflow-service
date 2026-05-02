# DevFlow Service

## 这个文档解决什么问题

这份 `README` 给第一次进入仓库的工程师或 AI Agent 一个最短入口，用来快速回答 6 个问题：

- 这个仓库现在到底承载哪些服务
- 当前实现事实和目标边界应该去哪里确认
- 核心资源之间是什么关系
- 代码目录应该怎么理解
- 本地最常用的开发与验证命令是什么
- 文档应该从哪里开始读

## 当前状态

`devflow-service` 是 DevFlow 后端的 Go monorepo。

当前仓库已经有 5 个可运行入口：

- `meta-service`：元数据边界，当前迁移主线仍然围绕它
- `config-service`：配置边界，拥有 `AppConfig` 和 `WorkloadConfig`
- `network-service`：网络边界，拥有 `Service` 和 `Route`
- `release-service`：发布边界，拥有 `Manifest`、`Release`、`Intent`，并吸收了原 `verify-service` 的 writeback / callback 路由
- `runtime-service`：运行时读写边界，负责 runtime read model、operator action、observer callback

关键约束：

- 当前可运行服务名仍然是 `meta-service`，不要把未来命名当成当前事实
- `telemetry-service` 目前只能视为 planned，不是当前代码的强依赖
- 仓库目标布局是根目录 `cmd/ + internal/`，不是旧的 `modules/`
- `internal/shared/` 只能放小而稳定的跨域 helper，不能重新变成 `common/` 或 `util/`

## 核心资源关系

从业务关系上看，当前仓库里的主要资源链路是：

1. `Project` 下面有多个 `Application`
2. `Application` 通过 `ApplicationEnvironment` 绑定到多个 `Environment`
3. `Application` 维度维护应用级配置与网络基线：
   - `WorkloadConfig`
   - `Service`
4. `Application + Environment` 维度维护环境差异：
   - `AppConfig`
   - `Route`
5. `Manifest` 是发布前的 build-side freeze point：
   - 冻结 `git_revision / commit_hash`
   - 冻结 `WorkloadConfig`
   - 冻结 `Service`
   - 记录镜像构建结果
6. `Release` 是 deploy-side freeze point：
   - 绑定一个 `Manifest`
   - 绑定一个目标 `Environment`
   - 冻结 `AppConfig`
   - 冻结 `Route`
   - 渲染并发布部署 bundle

最重要的边界规则：

- `Manifest` 是发布前冻结的不可变快照
- `Release` 消费 `Manifest`，但不会回写或重定义 `Manifest`
- 环境差异通过 `AppConfig`、`Route` 和 release render 时的 Overlay / EnvConfig 表达，不应该通过业务代码里的环境分支硬编码

## 仓库结构

当前根目录结构按职责分层：

- `cmd/`：可运行进程入口
- `internal/`：业务实现
- `internal/platform/`：基础设施能力
- `internal/shared/`：少量稳定共享 helper
- `api/`：稳定契约，当前主要是 OpenAPI
- `deployments/`：部署与环境清单
- `docs/`：分层文档
- `scripts/`：验证和辅助脚本
- `test/`：集成或端到端验证面

业务代码约定为：

- `internal/<domain>/domain`
- `internal/<domain>/service`
- `internal/<domain>/repository`
- `internal/<domain>/transport`

目录、命名和依赖方向以 `docs/policies/go-monorepo-layout.md` 为准。

## 文档地图

推荐阅读顺序：

1. [AGENTS.md](AGENTS.md)
2. `docs/system/recovery.md`
3. `docs/system/architecture.md`
4. `docs/system/domain-model.md`
5. `docs/services/README.md`
6. `docs/resources/README.md`
7. `docs/api/README.md`
8. `docs/guides/README.md`
9. `docs/policies/go-monorepo-layout.md`

文档目录按用途分层：

- `docs/index/`：导航入口
- `docs/system/`：当前实现事实
- `docs/services/`：服务边界
- `docs/resources/`：资源契约
- `docs/api/`：API 统一约定与 breaking changes
- `docs/guides/`：开发者操作指南
- `docs/policies/`：长期规则
- `docs/architecture/`：图示材料，只做可视化补充，不是当前事实源
- `docs/generated/`：生成产物
- `docs/archive/`：历史材料

## 本地开发与验证

最常用命令：

```sh
make fmt-check
go test ./...
make build-all
make openapi-check
bash scripts/verify.sh
```

运行单个服务：

```sh
make run APP=meta-service
make run APP=config-service
make run APP=network-service
make run APP=release-service
make run APP=runtime-service
```

当前验证基线：

```sh
make fmt-check
go vet ./...
golangci-lint run
go test ./...
go build -o bin/meta-service ./cmd/meta-service
go build -o bin/config-service ./cmd/config-service
go build -o bin/network-service ./cmd/network-service
go build -o bin/release-service ./cmd/release-service
go build -o bin/runtime-service ./cmd/runtime-service
bash scripts/verify.sh
```

如果改了 API 相关代码，还必须运行：

```sh
make openapi-check
```

## 数据与运行时说明

当前 pre-production 基线中：

- `meta-service`、`config-service`、`network-service`、`release-service` 共享 Kubernetes 里的 PostgreSQL 18 集群
- `runtime-service` 的 active runtime-domain 路径是 PostgreSQL-free
- release bundle 通过 OCI registry + Argo CD 下发

相关当前事实文档：

- `docs/system/postgresql.md`
- `docs/system/flow-overview.md`
- `docs/services/release-service.md`
- `docs/services/runtime-service.md`

## Assumptions

- 仓库外部的 `devflow-control` 目标架构文档当前不在本地 workspace 中，所以这份 `README` 只以本仓库当前实现和本仓库现有文档为准
- 当文档提到 `application-service` 时，指的是“元数据边界的目标命名或概念边界”；当前实际可运行服务名仍然是 `meta-service`
