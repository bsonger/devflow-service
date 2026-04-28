# Release Service

## Purpose

`release-service` owns build artifact records, deployment intent, release execution, and verify/writeback callbacks.

## Owns

- `Manifest`
- `Image`
- `Release`
- `Intent`
- build and release lifecycle records around manifest OCI deployment artifacts
- verify ingress and verification writeback responsibilities previously modeled as `verify-service`

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
- `RuntimeSpec`
- `RuntimeSpecRevision`
- `RuntimeObservedPod`
- `RuntimeOperation`

## Upstream Dependencies

- PostgreSQL
- Tekton
- Argo CD
- Kubernetes API

## Downstream Consumers

- platform orchestration layers
- verify-time consumers

## Entrypoint

```text
cmd/release-service/main.go
```

## Registered Domains

```text
internal/manifest/
internal/intent/
internal/release/
```

## Pre-production Shared Ingress

- `/api/v1/release/...`

## Resource Contracts

- `docs/resources/manifest.md`
- `docs/resources/image.md`
- `docs/resources/intent.md`
- `docs/resources/release.md`

Operational callback contract:

- `docs/system/release-writeback.md`

## Diagnostics

- `internal/release/transport/http/router.go`
- `internal/manifest/transport/http`
- `internal/intent/transport/http`
- `internal/release/service`
- `docs/policies/worker-runtime.md`

Runtime endpoints:

- `/healthz`
- `/readyz`
- `/internal/status`

## Verification

```sh
go test ./internal/manifest/... ./internal/intent/... ./internal/release/...
go build -o bin/release-service ./cmd/release-service
bash scripts/verify.sh
```
