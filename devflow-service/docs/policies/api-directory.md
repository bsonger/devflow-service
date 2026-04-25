# API Directory Policy

This document defines how `api/` is used as the contract layer for `devflow-service`.

## Scope

- `api/` holds durable service contracts: OpenAPI, protobuf, JSON Schema, and examples.
- `internal/<domain>/transport` implements the contract; it does not author it.

## Directory structure

```
api/
├── openapi/
│   └── swagger.yaml        # generated from code annotations; checked in as stable contract
└── proto/
    └── (reserved for future gRPC services)
```

## Rules

1. **Contract first** — `api/` is the source of truth for external callers. Handlers must align with the documented paths, parameters, and response shapes.
2. **OpenAPI generation** — Swagger/OpenAPI documents are generated from swag annotations in handler source code via `scripts/regen-swagger.sh`.
3. **Generated artifacts are checked in** — `api/openapi/swagger.yaml` and `api/openapi/swagger.json` are part of the repo so that consumers can read the contract without building the project.
4. **No business logic in `api/`** — `api/` defines "how to call us"; `internal/` implements "how we handle the request".
5. **All handlers must carry swag annotations** — Every exported HTTP handler must have `@Summary`, `@Tags`, `@Router`, and `@Success` (or equivalent) comments so that the contract stays complete and up to date.
6. **Proto directory is reserved** — When gRPC surfaces are introduced, their `.proto` definitions live under `api/proto/<service>/v1/`.

## Relationship to implementation

```
api/openapi/swagger.yaml     # contract surface (generated, checked in)
    ↓
internal/<domain>/transport  # HTTP/gRPC adapter implementation
    ↓
internal/<domain>/service    # use-case orchestration
    ↓
internal/<domain>/domain     # business rules
```

## Regeneration

Run from the repo root:

```bash
./scripts/regen-swagger.sh
```

This invokes `swag init` and writes the output to `api/openapi/`.

## Current notes

- Today only HTTP/REST is active; gRPC contracts are not yet defined.
- The active service name remains `meta-service`.
- Handler annotations must stay in sync with route registration in `internal/*/transport/http/`.
