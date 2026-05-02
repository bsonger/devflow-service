# Services

## 这个文档解决什么问题

`docs/services/` 用来回答一个核心问题：

- 现在到底是哪个服务拥有哪个资源和哪段运行时职责

它不是资源字段手册，也不是 API 总览。

## 当前服务清单

- `meta-service.md`
- `config-service.md`
- `network-service.md`
- `release-service.md`
- `runtime-service.md`

## 必须先搞清楚的命名映射

当前仓库里最容易让新人和 Agent 混淆的，是“当前实现名”和“目标边界名”不是一套词。

请按下面理解：

- 当前实际可运行服务仍然是 `meta-service`
- 当设计讨论提到 `application-service` 时，应理解为“应用元数据边界的目标命名或概念边界”；当前实现事实仍由 `meta-service` 文档负责
- `verify-service` 不再是当前独立可运行服务；它的 verify ingress / writeback contract 已经归入 `release-service`
- `telemetry-service` 不是当前实现服务，只能标记为 planned

## 每篇服务文档至少要回答什么

每篇服务文档都应该明确写清楚：

- 这个服务解决什么问题
- 它拥有哪些资源
- 它不拥有哪些资源
- 它依赖哪些上游事实或外部系统
- 当前实现和目标边界是否仍有差距
- 后端本地路由和 shared ingress 路由分别是什么
- 改动这个服务时应该去看哪些资源文档、系统文档和验证命令

## 使用规则

- 看当前事实，优先相信这里和 `docs/system/*`
- 看具体字段和 API，再跳到 `docs/resources/*` 或 `docs/api/*`
- 如果服务文档和资源文档冲突，必须在同一次改动里一起修

## 相关文档

- `docs/system/architecture.md`
- `docs/system/current-service-extraction-reality.md`
- `docs/resources/README.md`
- `docs/api/README.md`
