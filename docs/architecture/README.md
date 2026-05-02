# DevFlow Architecture Diagrams

## 这个文档解决什么问题

这个目录只提供图示材料。
如果你已经知道要看哪一块，但想先建立视觉概览，可以来这里。

它不负责定义当前实现事实。
当前事实始终以 `docs/system/*`、`docs/services/*`、`docs/resources/*` 为准。

## 怎么使用

- 先读 `docs/system/architecture.md`
- 再读这里的图，帮助建立全局图像
- 如果图和正文冲突，以正文为准，并把图同步更新

## 图示索引

| 图 | 文件 | 适合回答什么问题 |
|---|---|---|
| Overall Architecture | [overall-architecture.mmd](overall-architecture.mmd) | 用户、网关、服务、数据库、Tekton、Argo、Kubernetes 如何连接 |
| Release Flow | [release-flow.mmd](release-flow.mmd) | 一次发布从 Manifest 到 Release 再到 rollout 如何流转 |
| CI Build Flow | [ci-build-flow.mmd](ci-build-flow.mmd) | 一次构建如何从 git revision 变成镜像和 Manifest |
| Service Boundaries | [service-boundaries.mmd](service-boundaries.mmd) | 哪个服务拥有哪些资源，哪些依赖是跨服务的 |

## 特别说明

- `telemetry-service` 在图里只能当 planned 看待，不能当成当前代码强依赖
- 图里的未来命名不能覆盖当前可运行服务名
- 如果你只想确认当前事实，不要停留在本目录，直接回到 `docs/system/` 或 `docs/services/`
