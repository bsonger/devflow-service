# OpenAPI Workflow

## 这个文档解决什么问题

这份文档解释在 `devflow-service` 里什么时候该改 OpenAPI，以及按什么顺序改。

## 什么时候必须检查 OpenAPI

只要改动涉及下面任何一个面，就必须检查 OpenAPI：

- route registration
- HTTP handler
- request DTO
- response DTO
- domain enum
- 错误包裹或错误码
- 分页格式
- auth middleware
- request-id middleware

## 推荐顺序

1. 先看代码里的真实 route 和 DTO
2. 再看 shared ingress 的路径映射
3. 更新对应 `api/openapi/<service>.yaml`
4. 同步更新 `api/openapi/devflow.yaml`
5. 必要时刷新 `swagger.yaml` / `swagger.json`
6. 更新相关 `docs/resources/*.md` 和 `docs/api/*.md`
7. 运行 `make openapi-check`

## 当前文件职责

- `api/openapi/<service>.yaml`：按服务划分的 canonical 契约
- `api/openapi/devflow.yaml`：聚合视图
- `api/openapi/swagger.yaml` / `swagger.json`：注解生成快照

## 不能做的事

- 不要把代码里没有的接口先写进 OpenAPI
- 不要只改 `devflow.yaml` 而漏掉对应 service 文件
- 不要把内部 observer 路由默认写成对外契约
- 不要在不兼容变化发生时假装没有 breaking change

## 验证命令

```sh
make openapi-check
```

必要时还要补：

```sh
go test ./...
bash scripts/verify.sh
```

## 相关文档

- `api/openapi/README.md`
- `docs/api/contract-guide.md`
- `docs/policies/api-contract-policy.md`
- `docs/policies/api-compatibility.md`
