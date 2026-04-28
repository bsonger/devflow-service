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

## Upstream Dependencies

- PostgreSQL
- centralized config repo
- shared backend primitives

## Downstream Consumers

- release-time consumers
- platform orchestration layers

## Entrypoint

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
