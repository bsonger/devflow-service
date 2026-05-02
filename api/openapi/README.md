# OpenAPI

This directory contains edge-facing OpenAPI contracts plus generated backend-local Swagger annotation snapshots for HTTP handlers.

## Current artifacts

- `meta-service.yaml`
- `network-service.yaml`
- `config-service.yaml`
- `release-service.yaml`
- `runtime-service.yaml`
- `devflow.yaml`
- `docs.go`
- `swagger.json`
- `swagger.yaml`

The `*-service.yaml` files are the canonical shared-ingress service-scoped contracts.
`devflow.yaml` is the aggregate shared-ingress HTTP contract across services.
`swagger.yaml` and `swagger.json` remain generated annotation snapshots so API consumers can inspect the annotation-backed view without rebuilding the project.
They are not a complete list of every registered HTTP route unless every handler has current Swagger annotations.

## Frontend boundary

The canonical OpenAPI files in this directory describe shared-ingress external routes.

That means:

- `basePath` stays `/`
- `meta-service.yaml` uses `/api/v1/meta/...`
- `config-service.yaml` uses `/api/v1/config/...`
- `network-service.yaml` uses `/api/v1/network/...`
- `release-service.yaml` uses `/api/v1/release/...`
- `runtime-service.yaml` uses `/api/v1/runtime/...`
- the files do not pin a deployment `host` or environment-specific base URL

Frontend callers may treat these files as the direct edge-routing contract for the shared ingress path layer.

Backend-local routes still exist in code and are reflected by the generated `swagger.yaml` / `swagger.json` snapshot, but those generated artifacts are not the primary frontend contract.

## Generation

Regenerate from the repo root with:

```sh
bash scripts/regen-swagger.sh
```

The script uses `swag init` when the `swag` CLI is installed.
If `swag` is missing, the script exits successfully after printing a skip message.

Current generator entrypoint:

```text
cmd/meta-service/main.go
```

Important nuance:

- the generator scans internal handler annotations across the repo
- generated artifacts only include routes that have Swagger annotations
- runtime-service routes are registered in code but are not currently present in `swagger.json` / `swagger.yaml`
- generated paths are backend-local service routes such as `/api/v1/projects`, `/api/v1/app-configs`, and `/api/v1/releases`
- the canonical `*-service.yaml` and `devflow.yaml` files rewrite those paths into shared-ingress external paths such as `/api/v1/meta/projects` or `/api/v1/config/app-configs`

For shared ingress route rewriting, read:

```text
docs/system/ingress-routing.md
```

## Truth order

Use these sources together:

1. handler code and route registration own what the backend can actually serve
2. `docs/system/ingress-routing.md` owns the shared-ingress prefix and rewrite rules
3. `docs/resources/*.md` own resource behavior, validation notes, and shared-ingress examples
4. `api/openapi/meta-service.yaml`, `network-service.yaml`, `config-service.yaml`, `release-service.yaml`, and `runtime-service.yaml` are the canonical shared-ingress service-scoped contracts
5. `api/openapi/devflow.yaml` is the aggregate shared-ingress contract view
6. `api/openapi/swagger.yaml` and `api/openapi/swagger.json` are generated backend-local annotation snapshots

When a service OpenAPI file and handler code disagree, fix the affected service file to match the current ingress-mapped contract implied by code plus ingress routing.
When `devflow.yaml` and the service files disagree, fix the aggregate file in the same change.
When the generated Swagger snapshot and handler code disagree, fix the annotations and regenerate the generated artifacts.
When contract docs and code disagree on behavior, inspect the code and update both surfaces in the same change.
