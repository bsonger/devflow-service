# Release Service

This service boundary has been migrated into `devflow-service`.
Use this file as the repo-local summary for where release-owned behavior now lives in code.

## Owns

- `Manifest`
- `Release`
- `Intent`
- build and release lifecycle records around manifest OCI deployment artifacts plus release-owned rollout snapshots
- verify ingress and verification writeback responsibilities that were previously modeled as `verify-service`

## Does Not Own

- `Project`
- `Application`
- `AppConfig`
- `WorkloadConfig`
- `Service`
- `Route`

## Upstream Dependencies

- PostgreSQL
- Tekton
- Argo CD
- Kubernetes API

## Downstream Consumers

- platform orchestration layers
- verify-time consumers

## Current Merge Note

`verify-service` is no longer treated as a separate service summary in this repo.
Its ingress and verification concerns are now considered part of the broader `release-service` ownership boundary.

`runtime-service` is now a separate runnable service summary in this repo again.
Release-time runtime binding checks now read runtime lookup state through the runtime-service contract instead of treating runtime as part of the release-owned HTTP surface.

## Current Repo Entry

In `devflow-service`, the migrated entrypoint now lives at:

```text
cmd/release-service/main.go
```

The migrated implementation is split across release-owned top-level domains plus release-specific assembly:

```text
internal/manifest/
internal/intent/
internal/release/
```

The current repo-local layout follows the monorepo policy:

```text
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
It now follows the same top-level domain split style already used elsewhere in this repository: resource-specific business code lives in `internal/manifest` and `internal/intent`, while `internal/release` keeps release-specific orchestration, persistence, runtime adapters, HTTP assembly, and cross-domain support helpers.

The resource contracts owned by this boundary are documented at:

- `docs/resources/manifest.md`
- `docs/resources/intent.md`
- `docs/resources/release.md`

The non-resource operational callback contract for observer and verify writeback lives at:

- `docs/system/release-writeback.md`

The worker and runtime helper boundary for background execution, intent claiming, and generic runtime bootstrap is governed by:

- `docs/policies/worker-runtime.md`

Runtime endpoints for this boundary include:

- `/healthz`
- `/readyz`
- `/internal/status`

The same migration boundary now applies to release-time downstream readers:

```text
internal/application/transport/downstream
internal/project/transport/downstream
internal/environment/transport/downstream
internal/cluster/transport/downstream
internal/appconfig/transport/downstream
internal/appservice/transport/downstream
internal/release/transport/runtime
internal/shared/downstreamhttp
```

After this migration, `release` no longer owns application/project/environment/cluster/config/service-route/runtime lookup state directly.
`internal/release/transport/downstream` is now limited to release-specific orchestrator binding access, while runtime lookup calls go through `internal/release/transport/runtime`.
