# Resources

## Purpose

This directory contains the current resource-contract docs for `devflow-service`.
Use these docs to answer three questions quickly:

- which service owns this resource
- what the current API surface looks like
- what the current validation and write rules are

## Standard resource doc format

Most resource docs in this directory fall into one of three shapes.

### 1. CRUD-backed resource docs

Use this shape when the resource is a normal persisted contract:

1. `# <Resource>`
2. `## Ownership`
3. `## Purpose`
4. `## Common base fields` when the resource persists the shared CRUD columns
5. `## Field table`
6. `## API surface`
7. `## Create / update rules`
8. `## Validation notes`
9. `## Source pointers`

### 2. Operation-oriented resource docs

Use this shape when the external contract is primarily action/read-model oriented rather than simple CRUD, for example runtime operations:

1. `# <Resource>`
2. `## Ownership`
3. `## Purpose`
4. `## Main operator flows` or equivalent execution-oriented overview
5. `## API surface`
6. `## Request contracts` / response focus as needed
7. `## Validation notes`
8. `## Source pointers`

### 3. Derived or read-only contract docs

Use this shape when the contract is derived from another persisted resource and has no standalone CRUD surface, for example `Image`:

1. `# <Resource>`
2. `## Ownership`
3. `## Purpose`
4. `## Field table` or source-field mapping
5. `## API surface`
6. `## Create / update rules`
7. `## Validation notes`
8. `## Source pointers`

Some resources add extra sections such as nested types, lifecycle, sync behavior, relationship notes, or UI guidance, but the owning headings above should stay stable for that document type.

## Resource ownership map

- `meta-service`
  - `project.md`
  - `application.md`
  - `application-environment.md`
  - `cluster.md`
  - `environment.md`
- `config-service`
  - `appconfig.md`
  - `workloadconfig.md`
- `network-service`
  - `service.md`
  - `route.md`
- `release-service`
  - `manifest.md`
  - `release.md`
  - `intent.md`
  - `image.md`
- `runtime-service`
  - `runtime-spec.md`

`frontend-ui.md` is the UI-facing companion doc for these resource contracts.

## External API path reminder

Unless a document says otherwise, `/api/v1/...` examples in this directory are service-internal routes.
For pre-production shared ingress, use the service-prefixed external paths documented in each resource file:

- meta-owned resources: `/api/v1/meta/...`
- config-owned resources: `/api/v1/config/...`
- network-owned resources: `/api/v1/network/...`
- release-owned resources: `/api/v1/release/...`
- runtime-owned resources: `/api/v1/runtime/...`

## Index

- `application.md`: `Application`
- `application-environment.md`: `ApplicationEnvironmentBinding`
- `frontend-ui.md`: frontend information architecture and field-level UI contract
- `runtime-frontend-checklist.md`: short frontend integration checklist for runtime page reads/actions
- `project.md`: `Project`
- `cluster.md`: `Cluster`
- `environment.md`: `Environment`
- `appconfig.md`: `AppConfig`
- `workloadconfig.md`: `WorkloadConfig`
- `service.md`: application-owned network `Service`
- `route.md`: application-owned `Route`
- `runtime-spec.md`: `RuntimeSpec`, `RuntimeSpecRevision`, `RuntimeObservedWorkload`, `RuntimeObservedPod`, `RuntimeOperation`
- `manifest.md`: `Manifest`
- `intent.md`: `Intent`
- `release.md`: `Release`
- `image.md`: derived image-output contract owned by the release boundary

## Notes

- one resource belongs to exactly one active service boundary
- for shared CRUD, pagination, filtering, and soft-delete rules, start with `docs/policies/resource-api.md`
- keep service-boundary and merge-status docs under `docs/services/`
