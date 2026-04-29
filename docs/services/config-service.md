# Config Service

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
- `meta-service` for application and environment metadata resolution
- shared backend primitives

### What config-service depends on by workflow

- app config CRUD and sync:
  - `meta-service` validates application and environment context
  - GitHub config repo provides file source-of-truth
  - PostgreSQL persists config state and revision history
- workload config CRUD:
  - `meta-service` validates application ownership context
  - PostgreSQL persists workload runtime shape

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
