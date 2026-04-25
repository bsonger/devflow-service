# Release Service

This file is migrated from `devflow-control` as a cross-repo reference.
It is ownership context, not current implementation authority for this repo.

## Owns

- `Manifest`
- `Image`
- `Release`
- `Intent`
- build and release lifecycle records around frozen manifest snapshots and OCI deployment artifacts
- verify ingress and verification writeback responsibilities that were previously modeled as `verify-service`

## Does Not Own

- `Project`
- `Application`
- `AppConfig`
- `WorkloadConfig`
- `Service`
- `Route`
- runtime desired-state semantics

## Upstream Dependencies

- PostgreSQL
- runtime service
- Tekton
- Argo CD
- Kubernetes API

## Downstream Consumers

- platform orchestration layers
- verify-time consumers

## Current Merge Note

`verify-service` is no longer treated as a separate service summary in this repo.
Its ingress and verification concerns are now considered part of the broader `release-service` ownership boundary.

## Current Repo Entry

In `devflow-service`, the migrated entrypoint now lives at:

```text
cmd/release-service/main.go
```

The migrated implementation is split across release-owned top-level domains plus release-specific assembly:

```text
internal/image/
internal/manifest/
internal/intent/
internal/release/
```

The current repo-local layout follows the monorepo policy:

```text
internal/image/service
internal/image/transport/http
internal/image/module.go
internal/manifest/service
internal/manifest/transport/http
internal/manifest/module.go
internal/intent/service
internal/intent/transport/http
internal/intent/module.go
internal/release/domain
internal/release/support
internal/release/service
internal/release/repository
internal/release/transport/http
internal/release/transport/argo
internal/release/transport/downstream
internal/release/transport/runtime
internal/release/transport/tekton
internal/release/module.go
```

This means `release-service` is no longer modeled as one large `internal/release/service` implementation area.
It now follows the same top-level domain split style already used elsewhere in this repository: resource-specific business code lives in `internal/image`, `internal/manifest`, and `internal/intent`, while `internal/release` keeps release-specific orchestration, persistence, runtime adapters, HTTP assembly, and cross-domain support helpers.

The same migration boundary now applies to release-time downstream readers:

```text
internal/application/transport/downstream
internal/project/transport/downstream
internal/environment/transport/downstream
internal/cluster/transport/downstream
internal/appconfig/transport/downstream
internal/network/transport/downstream
internal/shared/downstreamhttp
```

After this migration, `release` no longer owns application/project/environment/cluster/config/network downstream clients.
`internal/release/transport/downstream` is now limited to release-specific orchestrator binding access.
