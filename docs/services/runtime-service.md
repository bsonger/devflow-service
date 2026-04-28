# Runtime Service

This service boundary has been migrated into `devflow-service`.
Use this file as the repo-local summary for where runtime-owned behavior now lives in code.

## Owns

- `RuntimeSpec`
- `RuntimeSpecRevision`
- `RuntimeObservedPod`
- `RuntimeOperation`
- runtime desired state for `application + environment`
- immutable runtime revisions
- live runtime observation responsibilities that were previously modeled as `resource-observer`
- direct K8s pod lifecycle operations (delete, deployment restart)

## Does Not Own

- image version
- release execution state
- rollout strategy
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
- shared backend primitives

## Downstream Consumers

- release-time consumers
- platform orchestration layers

## Current Merge Note

`devflow-observe` and earlier resource-observer-style live observation responsibilities were folded into the broader `runtime-service` ownership boundary first.

`runtime-service` now boots as a separate runnable entrypoint in this repo.
Its current repo-local entrypoint lives at `cmd/runtime-service/main.go`.

Full path reference:

```text
cmd/runtime-service/main.go
```

The extracted HTTP surface in this repo now includes:

- `POST /api/v1/runtime-specs`
- `GET /api/v1/runtime-specs`
- `GET /api/v1/runtime-specs/{id}`
- `DELETE /api/v1/runtime-specs?application_id=...&environment=...`
- `POST /api/v1/runtime-specs/{id}/revisions`
- `GET /api/v1/runtime-specs/{id}/revisions`
- `GET /api/v1/runtime-spec-revisions/{id}`
- `GET /api/v1/runtime-specs/{id}/pods`
- `POST /api/v1/runtime-specs/{id}/pods/{pod_name}/delete`
- `POST /api/v1/runtime-specs/{id}/deployments/{deployment_name}/restart`
- `GET /api/v1/runtime-specs/{id}/operations`
- `POST /api/v1/internal/runtime-spec-pods/sync`
- `POST /api/v1/internal/runtime-spec-pods/delete`

The internal observed-pod write endpoints are token-gated through:

- `X-Devflow-Observer-Token`
- `X-Devflow-Verify-Token`

K8s operation endpoints (`delete`, `restart`) run against the in-cluster K8s API and persist an audit record to `runtime_operations`.

In the current repo-local layout, the migrated implementation lives under:

```text
cmd/runtime-service/main.go
internal/runtime/domain
internal/runtime/repository
internal/runtime/service
internal/runtime/transport/http
```

Release-time callers still use the runtime-service lookup contract through:

- `docs/system/release-writeback.md`
- `internal/release/transport/runtime/client.go`
