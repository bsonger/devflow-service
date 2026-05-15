# API Contract Guide

## 这个文档解决什么问题

这份文档统一说明 `devflow-service` 的 API 契约风格。

读完后，读者应该能回答：

- 成功响应和失败响应的包裹长什么样
- 分页怎么传、怎么返回
- 哪些请求头是稳定约定
- 枚举值和版本兼容要遵守什么规则
- 哪些路径应该进入 canonical OpenAPI

## 适用范围

这份文档覆盖 repo 级 API 约定。
具体资源字段、单个接口的业务规则，仍以对应 `docs/resources/*.md` 为准。

## 路径分层

当前仓库要区分两层路径：

- service-internal route：代码里的 `/api/v1/...`
- shared ingress external route：对外暴露的 `/api/v1/<service-prefix>/...`

canonical OpenAPI 描述的是 shared ingress external route。

## 成功响应包裹

当前统一 success envelope 来自 `internal/platform/httpx`。

单对象响应：

```json
{
  "data": {
    "id": "..."
  }
}
```

列表响应：

```json
{
  "data": [],
  "pagination": {
    "page": 1,
    "page_size": 10,
    "total": 100
  }
}
```

## 错误响应包裹

当前统一 error envelope 为：

```json
{
  "error": {
    "code": "invalid_argument",
    "message": "invalid page",
    "details": {
      "field": "page"
    }
  }
}
```

当前稳定错误码集合：

- `invalid_argument`
- `not_found`
- `conflict`
- `failed_precondition`
- `unauthorized`
- `internal`

选择规则以 `docs/policies/error-handling.md` 为准。

## 分页约定

当前分页查询参数：

- `page`
- `page_size`

当前分页规则：

- 列表接口默认启用分页
- 未显式传参时，默认按 `page=1`、`page_size=10`
- `page` 必须从 `1` 开始
- 默认 `page_size` 是 `10`
- 最大 `page_size` 是 `100`

列表响应中的 `pagination` 字段会返回：

- `page`
- `page_size`
- `total`

## 常见请求头

当前稳定头部约定：

- `X-Request-Id`
- `X-Request-ID`
- `X-Trace-Id`

说明：

- 请求里没带 `X-Request-Id` / `X-Request-ID` 时，middleware 会生成一个
- 响应里会回显 `X-Request-Id` 和 `X-Request-ID`
- 当 trace context 有效时，会额外返回 `X-Trace-Id`

## Observer / Verify 保护规则

当前 token 保护的 callback 路径需要在契约里显式体现 `401 unauthorized`。

当前约定：

- observer / verify writeback 路径可以接受 `X-Devflow-Observer-Token`
- 为兼容历史命名，也接受 `X-Devflow-Verify-Token`

## 枚举规则

枚举是兼容性边界。
没有代码支持时，不要在文档或 OpenAPI 里发明新值。

当前通用规则：

- 错误码使用稳定小词表，保持小写下划线风格
- 资源状态值、步骤状态值、发布策略值保持代码中的既有大小写
- 兼容旧别名时，必须明确写出“当前代码会归一化”

当前已知例子：

- `Release.strategy` 的 canonical 值包括 `rolling`、`blueGreen`、`canary`
- 当前代码仍会把历史输入 `blue-green` 归一化成 `blueGreen`
- `ManifestStatus`、`ReleaseStatus`、`StepStatus` 这些状态值当前是区分大小写的

## 版本兼容规则

当前公开 HTTP 路径仍统一在 `/api/v1/...`。

兼容性要求：

- 不要静默删除已有路径
- 不要静默删除已有响应字段
- 不要静默修改已有字段类型
- 不要静默更改公开枚举值
- 如果确实发生不兼容变更，要同步更新 OpenAPI、资源文档和 `docs/api/breaking-changes.md`

## 什么应该进入 canonical OpenAPI

应该进入：

- 对外暴露的 shared ingress 路径
- 对外文档需要承诺的 callback / writeback 路径

不应该默认进入：

- 只供内部 observer 使用的 `/api/v1/internal/...` 路径
- 代码里不存在的“未来接口”

## 相关文档

- `api/openapi/README.md`
- `docs/policies/api-contract-policy.md`
- `docs/policies/api-compatibility.md`
- `docs/policies/error-handling.md`
- `docs/policies/http-handler.md`

## Assumptions

- 这份文档只总结当前 repo 级共性约定，不替代资源级文档
- 如果将来引入 `v2` 或新的外部协议层，应新增明确文档，而不是在这里隐式扩展
