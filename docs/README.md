# Docs

## 这个文档解决什么问题

这份文档是 `devflow-service` 的文档入口页。
它只做导航，不替代 `AGENTS.md`、`docs/system/*` 或具体资源文档。

读完后，读者应该能立刻知道：

- 先从哪份文档开始
- 当前事实、API 契约、开发指南、长期规则分别在哪一层
- 哪些目录是当前事实，哪些目录只是图示、生成产物或历史材料

## 从哪里开始

- Agent 启动入口：[AGENTS.md](../AGENTS.md)
- 人类读者入口：`docs/index/getting-started.md`

## 文档结构

按用途区分：

- `docs/index/`：导航，不承载实现事实
- `docs/system/`：当前实现事实与系统级说明
- `docs/services/`：服务边界、依赖和诊断
- `docs/resources/`：资源字段、API surface、校验规则
- `docs/api/`：API 统一契约说明、兼容性与 breaking changes
- `docs/guides/`：本地开发、扩展模块、更新 OpenAPI 等操作指南
- `docs/policies/`：长期规则和必须遵守的约束
- `docs/architecture/`：架构图和 Mermaid 图示，只做视觉辅助
- `docs/generated/`：生成产物
- `docs/archive/`：历史资料
- `docs/superpowers/`：历史设计稿、计划稿，不作为当前事实

## 推荐阅读顺序

1. `docs/index/getting-started.md`
2. `AGENTS.md`
3. `docs/system/recovery.md`
4. `docs/system/architecture.md`
5. `docs/system/domain-model.md`
6. `docs/services/README.md`
7. `docs/resources/README.md`
8. `docs/api/README.md`
9. `docs/guides/README.md`
10. `docs/policies/README.md`

## 按主题跳转

想看当前架构和服务边界：

- `docs/system/architecture.md`
- `docs/system/current-service-extraction-reality.md`
- `docs/services/README.md`

想看核心资源关系：

- `docs/system/domain-model.md`
- `docs/resources/README.md`

想看 API 契约和 OpenAPI：

- `docs/api/README.md`
- `docs/api/contract-guide.md`
- `api/openapi/README.md`
- `docs/policies/api-contract-policy.md`

想看本地开发和如何改代码：

- `docs/guides/local-development.md`
- `docs/guides/backend-change-playbook.md`
- `docs/guides/openapi-workflow.md`

想看目录、分层和工程规则：

- `docs/policies/go-monorepo-layout.md`
- `docs/policies/verification.md`
- `docs/policies/doc-synchronization.md`

想看资源 API、service layer、repository、downstream client、worker runtime 这些通用规则：

- `docs/policies/resource-api.md`
- `docs/policies/service-layer.md`
- `docs/policies/repository-layer.md`
- `docs/policies/downstream-client.md`
- `docs/policies/worker-runtime.md`

## 需要特别注意的目录

- `docs/system/` 才是当前事实层
- `docs/architecture/` 只提供图，不单独定义事实
- `docs/generated/` 不应该手工编辑
- 根目录下的 `docs/architecture.md`、`docs/recovery.md`、`docs/docker.md`、`docs/observability.md`、`docs/constraints.md` 都只是迁移后的兼容跳转页

## Notes

- 各目录下的 `README.md` 负责回答“这个目录用来解决什么问题”
- 真正的事实仍然由对应正文文档负责，不要只看目录索引就下结论
