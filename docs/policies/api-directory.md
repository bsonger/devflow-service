# API Directory Policy

This document defines how `api/` is used as the contract layer for `devflow-service`.

## Scope

- `api/` holds durable service contracts: OpenAPI, protobuf, JSON Schema, and examples.
- `internal/<domain>/transport` implements the contract; it does not author it.

## Directory structure

```
api/
├── openapi/
│   └── swagger.yaml        # generated from code annotations; checked in as an annotation-backed contract snapshot
└── proto/
    └── (reserved for future gRPC services)
```

## Rules

1. **Handler code and route registration stay authoritative** — `api/` is a contract layer for external callers, but the backend can only serve what the registered handlers actually expose.
2. **OpenAPI generation** — Swagger/OpenAPI documents are generated from swag annotations in handler source code via `scripts/regen-swagger.sh`.
3. **Generated artifacts are checked in** — `api/openapi/swagger.yaml` and `api/openapi/swagger.json` are part of the repo so that consumers can inspect the annotated contract snapshot without building the project.
4. **No business logic in `api/`** — `api/` defines "how to call us"; `internal/` implements "how we handle the request".
5. **Swagger coverage may be partial during migration** — Missing annotations mean a registered route can exist in code without appearing in `api/openapi/swagger.yaml` or `swagger.json`. Do not treat generated OpenAPI as a complete route inventory unless coverage has been verified.
6. **Proto directory is reserved** — When gRPC surfaces are introduced, their `.proto` definitions live under `api/proto/<service>/v1/`.

## Relationship to implementation

```
api/openapi/swagger.yaml     # annotation-backed contract snapshot (generated, checked in)
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
- Handler annotations should stay in sync with route registration in `internal/*/transport/http/`.
- Current generated OpenAPI includes annotated release writeback routes such as `/api/v1/verify/...`, but it does not currently include every registered runtime route.
- For the detailed OpenAPI truth order and ingress caveats, also read `api/openapi/README.md`.
