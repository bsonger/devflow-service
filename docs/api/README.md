# API

## 这个文档解决什么问题

`docs/api/` 负责解释跨资源、跨服务都适用的 API 约定。

当你要回答下面这些问题时，先来这里：

- 所有接口共享的错误格式是什么
- 分页格式和请求头约定是什么
- OpenAPI 应该怎么维护
- 哪些 API 变更属于 breaking change

## 当前文档

- `contract-guide.md`：统一 API 契约说明
- `breaking-changes.md`：已经发生或需要记录的 breaking changes

## 与其他目录的边界

- 资源字段和单个接口的语义，去 `docs/resources/*`
- 服务边界和职责，去 `docs/services/*`
- OpenAPI 文件本身和生成方式，去 `api/openapi/README.md`
- 必须遵守的契约维护规则，去 `docs/policies/api-contract-policy.md`

## 推荐阅读顺序

1. `contract-guide.md`
2. `api/openapi/README.md`
3. `docs/policies/api-contract-policy.md`
4. 受影响的 `docs/resources/*.md`
