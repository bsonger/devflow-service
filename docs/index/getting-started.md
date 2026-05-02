# Getting Started

## 这个文档解决什么问题

这份文档给第一次进入 `devflow-service` 的读者一条最短路径。
目标不是“看全”，而是先建立正确心智模型。

## 最短阅读路径

1. `AGENTS.md`
2. `docs/system/recovery.md`
3. `README.md`
4. `docs/system/architecture.md`
5. `docs/system/domain-model.md`
6. `docs/services/README.md`
7. `docs/resources/README.md`
8. `docs/api/README.md`
9. `docs/guides/README.md`

## 按任务继续深入

如果你要理解系统边界：

- `docs/system/architecture.md`
- `docs/system/current-service-extraction-reality.md`
- `docs/services/README.md`

如果你要理解发布链路：

- `docs/system/flow-overview.md`
- `docs/services/release-service.md`
- `docs/services/runtime-service.md`

如果你要看资源字段和接口：

- `docs/resources/README.md`
- `docs/api/contract-guide.md`
- `api/openapi/README.md`

如果你要本地开发或改代码：

- `docs/guides/local-development.md`
- `docs/guides/backend-change-playbook.md`
- `docs/guides/openapi-workflow.md`

如果你要判断长期工程规则：

- `docs/policies/go-monorepo-layout.md`
- `docs/policies/verification.md`
- `docs/policies/api-contract-policy.md`

## 一句话分层

- `docs/system/`：当前事实
- `docs/services/`：服务边界
- `docs/resources/`：资源契约
- `docs/api/`：API 统一约定
- `docs/guides/`：怎么改
- `docs/policies/`：必须遵守的规则
