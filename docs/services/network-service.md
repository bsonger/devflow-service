# Network Service

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
- `meta-service` for application and environment metadata resolution
- shared backend primitives

### What network-service depends on by workflow

- service CRUD:
  - `meta-service` validates application ownership context
  - PostgreSQL persists service definitions
- route CRUD and validation:
  - `meta-service` validates application and environment context
  - PostgreSQL persists route definitions
  - route validation reads release-facing service topology from network-owned `Service` records

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
