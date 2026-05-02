# Network Service

## 这个文档解决什么问题

这份文档说明 `network-service` 当前拥有哪些网络资源，以及它在发布链路里提供哪些冻结前输入。

读完后，读者应该能回答：

- `Service` 和 `Route` 的归属边界是什么
- 当前实现中哪些校验已经落地，哪些还没有通过下游 HTTP 调用隔离
- `release-service` 会从这里读取哪些输入

## Reader routing

如果你要看资源字段和 API 细节，继续跳到：

- `docs/resources/service.md`
- `docs/resources/route.md`

## Purpose

`network-service` owns application-facing network definitions and route validation.

## Owns

- `Service`
- `Route`
- route validation for service-to-port targets

## Does Not Own

- `Project`
- `Application`
- `ApplicationEnvironment`
- `Cluster`
- `Environment`
- `AppConfig`
- `WorkloadConfig`
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
- shared backend primitives

### Current implementation reality

`network-service` should be understood as the owner of network-domain resources, but the current implementation does **not** yet enforce all of that boundary through downstream calls to `meta-service`.

Current code reality:

- `Service` CRUD currently validates payload shape, port structure, and protocol values
- `Service` CRUD does **not** currently prove `application_id` existence through a downstream `meta-service` HTTP call
- `Route` CRUD and `/routes:validate` currently validate route payload shape plus target `service_name` / `service_port` consistency against network-owned `Service` records
- `Route` flows do **not** currently prove `application_id` / `environment_id` existence through downstream `meta-service` HTTP calls

Do not read this doc as proof that metadata ownership validation has already been extracted into runtime inter-service calls.

### What network-service depends on by workflow

- service CRUD:
  - PostgreSQL persists service definitions
  - current implementation validates network payload shape and port semantics locally
- route CRUD and validation:
  - PostgreSQL persists route definitions
  - route validation reads network-owned `Service` records to confirm target `service_name` / `service_port`

## Downstream Consumers

- `release-service`
  - reads services during manifest creation
  - reads routes during release creation
- platform orchestration layers

## Entrypoint

Primary runnable entrypoint: `cmd/network-service/main.go`.

```text
cmd/network-service/main.go
```

## Registered Domains

```text
internal/service/
internal/route/
```

## Pre-production Shared Ingress

- `/api/v1/network/...`

## Resource Contracts

- `docs/resources/service.md`
- `docs/resources/route.md`

## Diagnostics

- `internal/networkservice/transport/http/router.go`
- `internal/service/transport/http`
- `internal/route/transport/http`

Runtime endpoints:

- `/healthz`
- `/readyz`
- `/internal/status`

## Verification

```sh
go test ./internal/service/... ./internal/route/... ./internal/networkservice/...
go build -o bin/network-service ./cmd/network-service
bash scripts/verify.sh
```
