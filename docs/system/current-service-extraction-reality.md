# Current Service Extraction Reality

## Purpose

This document records what is true in the current repo implementation while service boundaries are being extracted.
Use it to avoid treating the target service architecture as if every runtime dependency has already become an isolated downstream HTTP call.

## Current boundary summary

The repo currently has five runnable service entrypoints:

- `cmd/meta-service`
- `cmd/config-service`
- `cmd/network-service`
- `cmd/release-service`
- `cmd/runtime-service`

Those entrypoints and shared-ingress routes are real.
Some implementation paths are still transitional because the code lives in one Go module and can still share repository code, service code, and database access.

## Implementation reality matrix

| Service | Current boundary state | Important current reality |
|---|---|---|
| `meta-service` | Runnable entrypoint and owner of metadata resources | Owns `Project`, `Application`, `ApplicationEnvironment`, `Cluster`, and `Environment` routes in the root layout |
| `config-service` | Runnable entrypoint and owner of config resources | `AppConfig` still resolves some metadata through same-repo support/service code; `WorkloadConfig` does not currently prove application existence through a downstream `meta-service` HTTP call |
| `network-service` | Runnable entrypoint and owner of network resources | `Service` and `Route` use network-owned persistence and validation; they do not currently prove application/environment existence through downstream `meta-service` HTTP calls |
| `release-service` | Runnable entrypoint and release composer | Uses downstream adapters for config/network/meta-facing reads in release and manifest flows, while still owning release persistence locally through Postgres stores |
| `runtime-service` | Runnable entrypoint for runtime read/action APIs | Default runtime service uses in-memory `RuntimeStore`; active runtime-domain storage is PostgreSQL-free; release rollout observer startup is active and consumes observer state plus Kubernetes labels |

## What is already real

- Service entrypoints exist under `cmd/`.
- Service-specific routers exist under `internal/*service/transport/http` or the service assembly package.
- Shared ingress routes are committed under `deployments/pre-production/istio/shared-ingress.yaml`.
- Resource ownership docs under `docs/services/` and `docs/resources/` describe the intended active ownership boundary.
- `release-service` has real downstream HTTP adapter usage for several upstream reads.

## What is still transitional

- A separate runnable service does not always mean every dependency is already a runtime HTTP dependency.
- Several domains still share a root Go module, shared platform code, and direct repository stores.
- Some validation that should logically belong across service boundaries is still local or absent.
- Runtime default read/action state is memory-backed, while Postgres-backed runtime code remains in the repository.

## Rules for future docs

When documenting current behavior:

- say "current implementation" only for behavior that exists in code
- label target behavior as target behavior
- label planned validation or downstream calls as not yet implemented unless the code path exists
- include source pointers when describing a transitional dependency

When changing service extraction boundaries, update:

- `docs/services/*.md`
- `docs/resources/*.md`
- `docs/system/architecture.md`
- this document
- relevant verification checks under `scripts/verify.sh`

## Source pointers

- `cmd/*/main.go`
- `internal/configservice/transport/http/router.go`
- `internal/networkservice/transport/http/router.go`
- `internal/release/transport/http/router.go`
- `internal/runtime/transport/http/router.go`
- `internal/appconfig/service/app_config.go`
- `internal/workloadconfig/service/workload_config.go`
- `internal/service/service/service.go`
- `internal/route/service/route.go`
- `internal/manifest/service/manifest.go`
- `internal/release/service/release.go`
- `internal/runtime/repository/repository.go`
- `internal/runtime/repository/memory.go`
- `internal/runtime/observer/release_rollout.go`
