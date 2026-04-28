# Runtime Service

## Purpose

`runtime-service` owns runtime desired state, runtime revisions, live observed pod state, and direct runtime operations.

## Owns

- `RuntimeSpec`
- `RuntimeSpecRevision`
- `RuntimeObservedPod`
- `RuntimeOperation`
- runtime desired state for `application + environment`
- immutable runtime revisions
- live runtime observation responsibilities previously modeled as `resource-observer`
- direct K8s pod lifecycle operations

## Does Not Own

- `Project`
- `Application`
- `ApplicationEnvironment`
- `Cluster`
- `Environment`
- `AppConfig`
- `WorkloadConfig`
- `Service`
- `Route`
- `Manifest`
- `Image`
- `Release`
- `Intent`

## Upstream Dependencies

- PostgreSQL
- Kubernetes API
- shared backend primitives

## Downstream Consumers

- release-time consumers
- platform orchestration layers

## Entrypoint

Primary runnable entrypoint: `cmd/runtime-service/main.go`.

```text
cmd/runtime-service/main.go
```

## Registered Domains

```text
internal/runtime/domain
internal/runtime/repository
internal/runtime/service
internal/runtime/transport/http
```

## Pre-production Shared Ingress

- `/api/v1/runtime/...`

Internal observer callbacks are service-internal only and are not part of the shared-ingress external contract.

## Resource Contracts

- `docs/resources/runtime-spec.md`

## Diagnostics

- `internal/runtime/transport/http/router.go`
- `internal/runtime/service/service.go`
- `internal/runtime/observer`
- `docs/system/release-writeback.md`

Runtime endpoints:

- `/healthz`
- `/readyz`
- `/internal/status`

## Verification

```sh
go test ./internal/runtime/... 
go build -o bin/runtime-service ./cmd/runtime-service
bash scripts/verify.sh
```
