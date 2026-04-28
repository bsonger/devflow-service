# Meta Service

## Purpose

`meta-service` is the current active service being migrated into the root `devflow-service` layout.

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

## Upstream Dependencies

- PostgreSQL
- shared backend primitives

## Downstream Consumers

- frontend and platform orchestration layers
- extracted service boundaries through downstream clients

## Entrypoint

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

Legacy alias still routed in pre-production:

- `/api/v1/platform/...`

## Resource Contracts

- `docs/resources/project.md`
- `docs/resources/application.md`
- `docs/resources/application-environment.md`
- `docs/resources/cluster.md`
- `docs/resources/environment.md`

## Diagnostics

- `AGENTS.md`
- `docs/system/recovery.md`
- `docs/system/architecture.md`
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
