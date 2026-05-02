# Local Development

## 这个文档解决什么问题

这份文档告诉新人和 Agent：

- 本地开发最常用的命令是什么
- 什么时候该跑哪些检查
- 如何运行单个服务
- API 改动后要补哪些验证

## 先做什么

第一次进入仓库，先执行：

```sh
make fmt-check
go test ./...
make build-all
bash scripts/verify.sh
```

这样可以先确认：

- Go 环境可用
- 当前代码可测试
- 五个服务入口都能构建
- 文档、脚本、目录结构没有明显漂移

## 常用命令

格式检查：

```sh
make fmt-check
```

格式化：

```sh
make fmt
```

测试：

```sh
go test ./...
```

构建全部服务：

```sh
make build-all
```

运行仓库总验证：

```sh
bash scripts/verify.sh
```

## 运行单个服务

当前支持：

```sh
make run APP=meta-service
make run APP=config-service
make run APP=network-service
make run APP=release-service
make run APP=runtime-service
```

说明：

- `meta-service`、`config-service`、`network-service`、`release-service` 当前都依赖 PostgreSQL 相关配置
- `runtime-service` 的 active runtime-domain 路径是 PostgreSQL-free，但它仍可能依赖 Kubernetes 或 observer 相关配置来获得完整行为

## 改动后怎么选验证

只改文档：

```sh
bash scripts/verify.sh
```

改 Go 代码：

```sh
make fmt-check
go test ./...
make build-all
bash scripts/verify.sh
```

改 API 相关代码：

```sh
make openapi-check
go test ./...
bash scripts/verify.sh
```

改 release / runtime writeback 相关代码：

优先跑 `scripts/README.md` 里列出的 focused seam tests，再跑全量验证。

## 什么时候去看部署清单

如果你需要确认服务启动参数、镜像选择、shared ingress 或 pre-production 环境差异，去看：

- `deployments/pre-production/`
- `deployments/tekton/`

如果你只是在做 repo-local 重构，不要把部署清单当作业务逻辑来源。

## 常见原则

- 不要为了让文档成立去改业务逻辑
- 不要随意引入新依赖；先复用现有 `internal/platform`、`internal/shared` 或已有库
- 不确定的地方写进 `Assumptions`
- 改 API 代码时，OpenAPI 和资源文档必须同一轮更新

## 相关文档

- `docs/guides/backend-change-playbook.md`
- `docs/guides/openapi-workflow.md`
- `docs/policies/verification.md`
- `scripts/README.md`
