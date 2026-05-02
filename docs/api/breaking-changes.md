# Breaking Changes

This file records contract-breaking differences between previously checked-in OpenAPI artifacts and the backend behavior that is already live in code.

## 2026-05-02

- OpenAPI storage is now split by real service boundary:
  - `meta-service.yaml`
  - `network-service.yaml`
  - `config-service.yaml`
  - `release-service.yaml`
  - `runtime-service.yaml`
  - `devflow.yaml` remains as an aggregate view
- Canonical OpenAPI paths now use the shared-ingress external prefixes:
  - `meta-service.yaml` uses `/api/v1/meta/...`
  - `config-service.yaml` uses `/api/v1/config/...`
  - `network-service.yaml` uses `/api/v1/network/...`
  - `release-service.yaml` uses `/api/v1/release/...`
  - `runtime-service.yaml` keeps `/api/v1/runtime/...`
- `application environment` responses were corrected to the real envelope shape. `POST /api/v1/applications/{id}/environments` and `GET /api/v1/applications/{id}/environments/{environment_id}` return `{"data": ...}` in code, while the older generated Swagger implied a bare object.
- observer-protected callback surfaces now explicitly document `401 unauthorized` for:
  - `/api/v1/verify/*`
  - `/api/v1/manifests/tekton/*`
  - `/api/v1/internal/runtime-*`
- runtime-service routes and manifest Tekton writeback routes were added to `api/openapi/devflow.yaml`. These routes already existed in code but were absent from the older generated Swagger snapshot.
- list endpoints now document the real pagination and filter query parameters where the older snapshot omitted them.
