# Resources

## 这个文档解决什么问题

`docs/resources/` 用来回答 3 件事：

- 这个资源归谁管
- 这个资源当前 API surface 是什么
- 这个资源有哪些字段、写规则和校验规则

## 先建立资源关系图

如果你第一次进仓库，先用下面这条链理解资源：

1. `Project`
2. `Application`
3. `ApplicationEnvironment`
4. `WorkloadConfig` / `Service`
5. `AppConfig` / `Route`
6. `Manifest`
7. `Release`

简化理解：

- `Project` 管应用分组
- `Application` 是业务主语
- `WorkloadConfig` 和 `Service` 是应用级基线
- `AppConfig` 和 `Route` 表达环境差异
- `Manifest` 冻结 build-side 输入
- `Release` 冻结 deploy-side 输入

如果你要看这条链更完整的解释，先跳到 `docs/system/domain-model.md`。

## 资源目录索引

`meta-service`：

- `project.md`
- `application.md`
- `application-environment.md`
- `cluster.md`
- `environment.md`

`config-service`：

- `appconfig.md`
- `workloadconfig.md`

`network-service`：

- `service.md`
- `route.md`

`release-service`：

- `manifest.md`
- `release.md`
- `intent.md`
- `image.md`

`runtime-service`：

- `runtime-spec.md`

补充材料：

- `frontend-ui.md`
- `runtime-frontend-checklist.md`

## 每篇资源文档应该回答什么

每篇资源文档至少要讲清楚：

- 资源归属的服务边界
- 资源解决什么问题
- 字段表
- API surface
- create / update / delete 规则
- 当前实现与理想边界是否还有差距
- 关键校验规则

## API 路径怎么读

资源文档里的路径要区分两层：

- service-internal route：代码里注册的 `/api/v1/...`
- shared ingress external route：外部调用看到的 `/api/v1/<service-prefix>/...`

前缀规则：

- meta：`/api/v1/meta/...`
- config：`/api/v1/config/...`
- network：`/api/v1/network/...`
- release：`/api/v1/release/...`
- runtime：`/api/v1/runtime/...`

共享入口重写规则以 `docs/system/ingress-routing.md` 为准。

## 使用规则

- 资源只属于一个当前有效服务边界
- 如果资源文档和服务文档冲突，要在同一次改动里修掉
- planned 行为必须明确标记，不能写成已经存在

## 相关文档

- `docs/system/domain-model.md`
- `docs/services/README.md`
- `docs/api/contract-guide.md`
- `docs/policies/resource-api.md`
