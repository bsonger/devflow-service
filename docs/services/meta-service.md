# Meta Service

## 这个文档解决什么问题

这份文档说明当前 `meta-service` 的职责边界。

读完后，读者应该能回答：

- `meta-service` 当前到底拥有哪些元数据资源
- 它和讨论中的 `application-service` 是什么关系
- 哪些下游服务依赖它
- 哪些能力不应该继续塞回 `meta-service`

## Reader routing

如果你要看字段和接口明细，继续跳到：

- `docs/resources/project.md`
- `docs/resources/application.md`
- `docs/resources/application-environment.md`
- `docs/resources/cluster.md`
- `docs/resources/environment.md`

如果你要看目标命名和资源关系，继续跳到：

- `docs/system/architecture.md`
- `docs/system/domain-model.md`

## Purpose

`meta-service` is the current active service being migrated into the root `devflow-service` layout.

It is the system metadata authority for the platform control plane.
Other services should not own duplicate truth for application, environment, cluster, or application-environment binding metadata.

## Owns

- `Project`
- `Application`
- `ApplicationEnvironment`
- `Cluster`
- `Environment`

## Does Not Own

- `AppConfig`
- `WorkloadConfig`
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
- shared backend primitives

### Downstream service consumers

- `config-service`
  - validates and resolves application context for app config and workload config ownership
- `network-service`
  - validates and resolves application context for service and route ownership
- `release-service`
  - resolves application projection during manifest creation
  - resolves application / environment / cluster deploy target during release creation and deployment
- frontend and platform orchestration layers

## What meta-service provides to other services

### To config-service

`meta-service` provides the metadata context needed to answer questions such as:

- does this application exist
- which project owns this application
- which environments are bound to this application

`config-service` still owns config truth.
`meta-service` only provides the metadata boundary.

### To network-service

`meta-service` provides the metadata context needed to answer questions such as:

- does this application exist
- which application-environment binding is valid
- which environment identifier is being referenced

`network-service` still owns network truth.
`meta-service` only provides the metadata boundary.

### To release-service

`meta-service` provides the metadata context needed to answer questions such as:

- what is the application name
- what repository is associated with the application
- which environment is being targeted
- which cluster is associated with that environment
- what namespace or deploy target should be used

`release-service` then combines that metadata with config and network truth from other services and freezes it into release-owned records.

## Dependency view

```mermaid
flowchart LR
    META[meta-service]
    PG[(PostgreSQL)]
    CS[config-service]
    NS[network-service]
    RS[release-service]
    UI[Frontend / Platform]

    META --> PG

    CS --> META
    NS --> META
    RS --> META
    UI --> META
```

### Notes

- `meta-service` is the metadata authority for application, environment, cluster, and binding truth
- `config-service`, `network-service`, and `release-service` consume metadata from `meta-service` but should not duplicate ownership
- `meta-service` does not execute builds or deployments

## What meta-service does not do

`meta-service` should not:

- own application configuration files
- own workload runtime shape
- own service ports or routes
- own build records
- own release records
- own live pod inspection or rollout operations
- render deployment bundles

That separation is important because `meta-service` is the metadata source of truth, not the deployment executor.

## Naming note

- 当前仓库没有独立可运行的 `application-service`
- 如果设计讨论提到 `application-service`，当前应理解为 `meta-service` 所承载的应用元数据边界
- 在代码和本地文档没有落地之前，不要把 `application-service` 写成当前事实

## Entrypoint

Primary runnable entrypoint: `cmd/meta-service/main.go`.

```text
cmd/meta-service/main.go
```

## Registered Domains

```text
internal/project/
internal/application/
internal/applicationenv/
internal/cluster/
internal/environment/
```

## Pre-production Shared Ingress

- `/api/v1/meta/...`

## Resource Contracts

- `docs/resources/project.md`
- `docs/resources/application.md`
- `docs/resources/application-environment.md`
- `docs/resources/cluster.md`
- `docs/resources/environment.md`

## Diagnostics

- `AGENTS.md`
- `internal/platform/...`
- `docs/system/recovery.md`
- `docs/system/architecture.md`
- `docs/system/diagrams.md`
- `docs/policies/verification.md`
- `scripts/README.md`

Runtime endpoints:

- `/healthz`
- `/readyz`
- `/internal/status`

## Verification

```sh
go test ./...
go build -o bin/meta-service ./cmd/meta-service
bash scripts/verify.sh
```
