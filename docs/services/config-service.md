# Config Service

## 这个文档解决什么问题

这份文档说明 `config-service` 当前拥有哪些配置资源，以及它和 `meta-service`、`release-service` 的边界如何配合。

读完后，读者应该能回答：

- `AppConfig` 和 `WorkloadConfig` 分别解决什么问题
- 哪些配置是应用级，哪些是环境级
- 当前实现还有哪些同仓过渡态

## Reader routing

如果你要看资源字段、API surface 和写规则，继续跳到：

- `docs/resources/appconfig.md`
- `docs/resources/workloadconfig.md`

如果你要看环境差异与 release freeze 的关系，继续跳到：

- `docs/system/domain-model.md`
- `docs/resources/manifest.md`
- `docs/resources/release.md`

## Purpose

`config-service` owns application configuration state and workload runtime shape.

## Owns

- `AppConfig`
- `WorkloadConfig`
- config repo sync semantics

## Does Not Own

- `Project`
- `Application`
- `ApplicationEnvironment`
- `Cluster`
- `Environment`
- `Service`
- `Route`
- `Manifest`
- `Image`
- `Release`
- `Intent`
- `RuntimeSpec`
- `RuntimeSpecRevision`
- `RuntimeObservedPod`
- `RuntimeOperation`

## Dependency model

### Upstream dependencies

- PostgreSQL
- centralized config repo
- shared backend primitives

### Current implementation reality

`config-service` should be understood as owning config-domain resources, but it is **not** currently a fully isolated downstream-HTTP consumer of `meta-service`.

Current code reality:

- `AppConfig` logic resolves application context through in-repo support/service code
- `AppConfig` environment-name resolution currently uses local service/database access inside this repo
- `WorkloadConfig` CRUD is currently repository-backed and does **not** call `meta-service` over HTTP
- this means the service boundary is documented and routed separately, but some ownership checks still rely on same-repo implementation access rather than extracted inter-service calls

Do not read this doc as proof that all config flows already validate ownership through runtime HTTP calls to `meta-service`.

### What config-service depends on by workflow

- app config CRUD and sync:
  - application projection and environment naming are resolved through current same-repo implementation paths
  - GitHub config repo provides file source-of-truth
  - PostgreSQL persists config state and revision history
- workload config CRUD:
  - PostgreSQL persists workload runtime shape
  - current implementation validates workload payload shape and uniqueness, but does **not** currently prove application existence through a downstream `meta-service` HTTP call

## Downstream Consumers

- `release-service`
  - reads workload config during manifest creation
  - reads app config during release creation
- platform orchestration layers

## Entrypoint

Primary runnable entrypoint: `cmd/config-service/main.go`.

```text
cmd/config-service/main.go
```

## Registered Domains

```text
internal/appconfig/
internal/workloadconfig/
```

## Pre-production Shared Ingress

- `/api/v1/config/...`

## Resource Contracts

- `docs/resources/appconfig.md`
- `docs/resources/workloadconfig.md`

## Diagnostics

- `docs/system/appconfig-cutover.md`
- `docs/system/appconfig-cutover-runbook.md`
- `internal/platform/config/config.go`
- `internal/configservice/transport/http/router.go`

Runtime notes:

- `config_repo.root_dir` and `config_repo.default_ref` must be initialized at startup
- the pre-production deployment mounts writable `/tmp` for config repo checkout

## Verification

```sh
go test ./internal/appconfig/... ./internal/workloadconfig/... ./internal/configservice/...
go build -o bin/config-service ./cmd/config-service
bash scripts/verify.sh
```
