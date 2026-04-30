# Ingress Routing

## Purpose

This document is the repo-local routing reference for the committed pre-production shared ingress.
Use it when deciding whether an API path is a backend service route or an edge-facing route.

## Source of truth

The committed pre-production edge artifact is:

```text
deployments/pre-production/istio/shared-ingress.yaml
```

That file owns the current Istio `Gateway` and `VirtualService` behavior for the shared host:

```text
devflow-pre-production.bei.com
```

## Backend-local routes

Backend services register their own HTTP routes under `/api/v1`.
Examples:

- `meta-service` registers routes such as `/api/v1/projects` and `/api/v1/applications`
- `config-service` registers routes such as `/api/v1/app-configs` and `/api/v1/workload-configs`
- `network-service` registers routes such as `/api/v1/services` and `/api/v1/routes`
- `release-service` registers routes such as `/api/v1/manifests`, `/api/v1/releases`, and `/api/v1/intents`
- `runtime-service` registers routes such as `/api/v1/runtime/workload` and `/api/v1/runtime/pods`

These are service-local paths. They are what the Gin routers register in code.

## Shared ingress routes

The pre-production shared ingress exposes service-prefixed paths:

| Edge path | Backend service | Backend path behavior |
|---|---|---|
| `/api/v1/meta/...` | `meta-service` | rewritten to `/api/v1/...` |
| `/api/v1/config/...` | `config-service` | rewritten to `/api/v1/...` |
| `/api/v1/network/...` | `network-service` | rewritten to `/api/v1/...` |
| `/api/v1/release/...` | `release-service` | rewritten to `/api/v1/...` |
| `/api/v1/runtime/...` | `runtime-service` | routed without rewrite |
| `/api/v1/platform/...` | `meta-service` | legacy compatibility prefix, rewritten to `/api/v1/...` |

The runtime prefix is different because runtime-service already owns `/api/v1/runtime/...` as its backend-local route tree.

## How to document routes

Resource docs should distinguish both surfaces:

- service-internal route surface: the path registered by the backend router
- pre-production shared ingress external surface: the edge-facing path after shared-service prefixing

For example, `config-service` registers:

```text
GET /api/v1/app-configs
```

The shared ingress exposes the same operation as:

```text
GET /api/v1/config/app-configs
```

## Boundary rule

Ingress routing is an edge concern.
It should not be used to move ownership between services.

Service ownership remains defined by:

- `docs/services/*.md`
- `docs/resources/*.md`
- the owning `internal/<domain>/...` packages

## Canonical pre-production operator proof route

For the representative operator-facing pre-production proof path, use the committed shared ingress exactly as rendered in `deployments/pre-production/istio/shared-ingress.yaml`.
Do not invent alternate prefixes and do not assume a rewrite for runtime routes.

Proof assumptions to keep explicit:

- shared host: `devflow-pre-production.bei.com`
- `runtime-service` owns `/api/v1/runtime/...` at both the backend and edge layers
- `/api/v1/runtime/...` is routed without rewrite
- `release-service` remains a separate backend reached through `/api/v1/release/...`, which *is* rewritten to `/api/v1/...` before it reaches the service

Canonical external read/action paths for this proof surface:

- `GET https://devflow-pre-production.bei.com/api/v1/runtime/workload?...`
- `GET https://devflow-pre-production.bei.com/api/v1/runtime/pods?...`
- `DELETE https://devflow-pre-production.bei.com/api/v1/runtime/pods/{pod_name}`
- `POST https://devflow-pre-production.bei.com/api/v1/runtime/rollouts`

If this route fails, diagnose in order:

1. confirm `kubectl apply -f deployments/pre-production/runtime-service.yaml`, `kubectl apply -f deployments/pre-production/release-service.yaml`, and `kubectl apply -f deployments/pre-production/istio/shared-ingress.yaml` were applied from the committed manifests
2. confirm the caller used `/api/v1/runtime/...` directly rather than `/api/v1/runtime-service/...` or another guessed prefix
3. confirm runtime failures are localized at the runtime boundary first before assuming release-service writeback or ingress drift

## Source pointers

- edge artifact: `deployments/pre-production/istio/shared-ingress.yaml`
- edge directory guide: `gateway/README.md`
- service routers:
  - `internal/app/router.go`
  - `internal/configservice/transport/http/router.go`
  - `internal/networkservice/transport/http/router.go`
  - `internal/release/transport/http/router.go`
  - `internal/runtime/transport/http/router.go`
