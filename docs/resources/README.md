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
7. `## Current implementation reality` when the current code differs from the target model or still has transitional behavior
8. `## Create / update rules`
9. `## Validation notes`
10. `## Source pointers`

### 2. Operation-oriented resource docs

Use this shape when the external contract is primarily action/read-model oriented rather than simple CRUD, for example runtime operations:

1. `# <Resource>`
2. `## Ownership`
3. `## Purpose`
4. `## Main operator flows` or equivalent execution-oriented overview
5. `## API surface`
6. `## Current implementation reality` when storage, observer, or runtime behavior is transitional
7. `## Request contracts` / response focus as needed
8. `## Validation notes`
9. `## Source pointers`

### 3. Derived or read-only contract docs

Use this shape when the contract is derived from another persisted resource and has no standalone CRUD surface, for example `Image`:

1. `# <Resource>`
2. `## Ownership`
3. `## Purpose`
4. `## Field table` or source-field mapping
5. `## API surface`
6. `## Current implementation reality` when the derived contract is not backed by a standalone route or table
7. `## Create / update rules`
8. `## Validation notes`
9. `## Source pointers`

Some resources add extra sections such as nested types, lifecycle, sync behavior, relationship notes, or UI guidance, but the owning headings above should stay stable for that document type.

## Current fact requirements

Each resource doc must separate current facts from target or planned behavior.

The `## API surface` section should distinguish:

- service-internal route surface: the backend-local `/api/v1/...` route registered in code
- pre-production shared ingress external surface: the service-prefixed edge path, when exposed through shared ingress

The `## Current implementation reality` section should be added when any of these are true:

- the code still uses same-repo access where the target model says downstream service calls
- the resource is derived and has no standalone table or CRUD route
- the default runtime storage differs from older persistence-oriented docs
- a field, route, validation rule, or dependency is planned but not implemented

Planned or desired behavior must be labeled as not yet implemented instead of being written into create/update rules or validation notes as current capability.

For shared ingress rewrite rules, use:

- `docs/system/ingress-routing.md`

Runtime-specific note:

- `runtime-spec.md` is currently an operation-oriented runtime surface doc, not a normal CRUD resource doc
- for current work, read it as a runtime inspection / action / observer-read-model contract
- internal names such as `RuntimeSpec` and `RuntimeSpecRevision` are still useful for code navigation, but they are not the main public API story

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
- `runtime-spec.md`: runtime inspection and action surface backed by `RuntimeObservedWorkload`, `RuntimeObservedPod`, and `RuntimeOperation`
- `manifest.md`: `Manifest`
- `intent.md`: `Intent`
- `release.md`: `Release`
- `image.md`: derived image-output contract owned by the release boundary

## Notes

- one resource belongs to exactly one active service boundary
- for shared CRUD, pagination, filtering, and soft-delete rules, start with `docs/policies/resource-api.md`
- keep service-boundary and merge-status docs under `docs/services/`
