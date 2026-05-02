# OpenAPI

## 这个文档解决什么问题

这份文档解释 `api/openapi/` 里每个文件的职责，以及它们和代码、shared ingress、生成产物之间的关系。

如果你要回答下面这些问题，从这里开始：

- 哪个 OpenAPI 文件才是对外契约
- `devflow.yaml` 和 `*-service.yaml` 分别是什么
- 什么时候要改 OpenAPI
- 为什么 `swagger.yaml` 不能直接当作完整路由清单

## 当前文件

- `meta-service.yaml`
- `config-service.yaml`
- `network-service.yaml`
- `release-service.yaml`
- `runtime-service.yaml`
- `devflow.yaml`
- `swagger.yaml`
- `swagger.json`
- `docs.go`

## 契约分层

按优先级理解：

1. 路由注册和 handler 代码决定后端真实能提供什么
2. `docs/system/ingress-routing.md` 决定 shared ingress 的前缀和 rewrite
3. `*-service.yaml` 是按服务划分的对外 OpenAPI 契约
4. `devflow.yaml` 是聚合后的对外 OpenAPI 契约
5. `swagger.yaml` / `swagger.json` 是从注解生成的后端本地快照

## 外部路径约定

对外契约描述的是 shared ingress 路径，不是服务内部路径。

因此：

- `meta-service.yaml` 使用 `/api/v1/meta/...`
- `config-service.yaml` 使用 `/api/v1/config/...`
- `network-service.yaml` 使用 `/api/v1/network/...`
- `release-service.yaml` 使用 `/api/v1/release/...`
- `runtime-service.yaml` 使用 `/api/v1/runtime/...`

注意：

- `runtime-service` 的对外路径和内部路径相同
- 其他服务的内部路径通常仍是 `/api/v1/...`

## 当前范围

当前 canonical OpenAPI 只覆盖对外公开或 shared-ingress 暴露的 API。

这意味着：

- `/api/v1/release/verify/...` 和 `/api/v1/release/manifests/tekton/...` 这类受 token 保护的对外 callback 路径应该出现在契约里
- `/api/v1/internal/runtime-pods/...` 和 `/api/v1/internal/runtime-workloads/...` 这类内部 observer 路径不是当前 canonical OpenAPI 的主体

## 与代码的一致性规则

当前仓库要求：

- 改了 route registration、handler、DTO、错误格式、分页格式、枚举、认证或 request-id 行为，就要检查 OpenAPI
- 先修 `*-service.yaml`
- 再同步 `devflow.yaml`
- 如果注解快照也受影响，再更新 `swagger.yaml` / `swagger.json`

## 生成与检查

从仓库根目录执行：

```sh
bash scripts/regen-swagger.sh
make openapi-check
```

说明：

- `regen-swagger.sh` 依赖 `swag` CLI；没安装时会跳过生成
- `make openapi-check` 会跑 regenerate、YAML 解析和契约测试

## 容易混淆的地方

- `swagger.yaml` 不是完整路由真相，因为它只覆盖有 Swagger 注解的 handler
- `devflow.yaml` 不是手工发明接口的地方，它只能聚合已经在代码里存在、并且应该对外暴露的路径
- 资源行为、校验细节和错误语义不能只看 OpenAPI，还要看 `docs/resources/*` 和 `docs/api/contract-guide.md`

## 相关文档

- `docs/api/README.md`
- `docs/api/contract-guide.md`
- `docs/policies/api-contract-policy.md`
- `docs/system/ingress-routing.md`
