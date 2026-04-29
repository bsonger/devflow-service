# OpenAPI

This directory contains generated Swagger/OpenAPI artifacts for repo-owned HTTP handlers.

## Current artifacts

- `docs.go`
- `swagger.json`
- `swagger.yaml`

These files are generated and checked in so API consumers can inspect the annotated route snapshot without rebuilding the project.
They are not a complete list of every registered HTTP route unless every handler has current Swagger annotations.

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
- generated paths are not rewritten shared-ingress paths such as `/api/v1/meta/projects` or `/api/v1/config/app-configs`

For shared ingress route rewriting, read:

```text
docs/system/ingress-routing.md
```

## Truth order

Use these sources together:

1. handler code and route registration own what the backend can actually serve
2. `docs/resources/*.md` own resource behavior, validation notes, and shared-ingress examples
3. `api/openapi/swagger.yaml` and `api/openapi/swagger.json` are generated backend-local annotation snapshots
4. `docs/system/ingress-routing.md` owns the difference between backend-local paths and pre-production edge paths

When the generated OpenAPI and handler code disagree, fix the handler annotations and regenerate the OpenAPI artifacts.
When generated OpenAPI and resource docs disagree on behavior, inspect the code and update both surfaces in the same change.
