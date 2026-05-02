# Backend Change Playbook

## 这个文档解决什么问题

这份文档给工程师和 Agent 一个统一流程，用来处理这些常见改动：

- 新增一个 domain module
- 新增一个 service entrypoint
- 新增一个 handler / route / API
- 改动 API 契约

## 改动前先判断边界

先回答 4 个问题：

1. 这个改动属于哪个服务边界
2. 这个资源应该归哪个 domain 目录
3. 这是当前事实，还是 future / planned 设计
4. 是否会影响 OpenAPI、资源文档或验证脚本

如果第 1 个问题答不清，先停下来读：

- `docs/system/architecture.md`
- `docs/system/current-service-extraction-reality.md`
- `docs/services/README.md`

## 新增一个 domain module

当前推荐结构：

- `internal/<domain>/domain`
- `internal/<domain>/service`
- `internal/<domain>/repository`
- `internal/<domain>/transport`
- `internal/<domain>/module.go`

基本步骤：

1. 在 `domain` 定义实体、枚举、领域规则
2. 在 `repository` 定义存储接口和实现
3. 在 `service` 写用例编排
4. 在 `transport/http` 写 handler 和 request / response DTO
5. 在 `module.go` 组装依赖
6. 把 module 注册到对应 service router
7. 更新对应 `docs/resources/*.md` 和 `docs/services/*.md`

## 新增一个 service entrypoint

只有当边界已经明确，才应该新增 service entrypoint。

最少要同步这些面：

1. `cmd/<service>/main.go`
2. 一个 service router
3. `api/openapi/<service>.yaml`
4. `api/openapi/devflow.yaml`
5. `docs/services/<service>.md`
6. `README.md`
7. `AGENTS.md`
8. `scripts/verify.sh`

不要只加一个 `main.go` 就算完成。

## 新增或修改一个 HTTP API

流程建议：

1. 先改 handler、DTO、service 调用
2. 确认 service-internal path
3. 映射 shared ingress external path
4. 更新资源文档中的 API surface、字段表、校验规则
5. 更新对应 `*-service.yaml`
6. 更新 `devflow.yaml`
7. 如果注解受影响，再刷新 `swagger.yaml` / `swagger.json`

特别注意：

- 不要在文档里发明代码里没有注册的接口
- 不要漏掉错误包裹、分页、枚举和鉴权说明

## 改动 API 时至少要同步哪些文档

- 受影响的 `docs/resources/*.md`
- 必要时更新 `docs/services/*.md`
- 必要时更新 `docs/system/architecture.md`
- `docs/api/breaking-changes.md`，如果存在不兼容变化

## Codex / Agent 工作规则

- 不要为了贴合文档去偷偷改业务逻辑
- 不要随意引入新依赖；先证明现有能力不够
- 不确定的地方写 `Assumptions`
- 如果 API 相关代码变了，必须检查 OpenAPI 和文档
- 如果只改了文档，也要确认文档没有和代码冲突

## 完成前的验证

最少验证：

```sh
make fmt-check
go test ./...
make build-all
bash scripts/verify.sh
```

如果改了 API：

```sh
make openapi-check
```

## 相关文档

- `docs/guides/local-development.md`
- `docs/guides/openapi-workflow.md`
- `docs/policies/go-monorepo-layout.md`
- `docs/policies/doc-synchronization.md`
