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

## Upstream Dependencies

- PostgreSQL
- shared backend primitives

## Downstream Consumers

- release-time consumers
- platform orchestration layers

## Entrypoint

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
