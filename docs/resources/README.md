# Resources

This directory contains the current resource-contract docs for `devflow-service`.

## Standard resource doc format

Unless a resource is intentionally read-only or derived-only, each file in this directory should follow this shape:

1. `# <Resource>`
2. `## Ownership`
3. `## Purpose`
4. `## Common base fields` when the resource persists the shared CRUD columns
5. `## Field table`
6. `## API surface`
7. `## Create / update rules`
8. `## Validation notes`
9. `## Source pointers`

Some resources add extra sections such as nested types, lifecycle, sync behavior, or UI guidance, but the core headings above should stay stable.

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
- `project.md`: `Project`
- `cluster.md`: `Cluster`
- `environment.md`: `Environment`
- `appconfig.md`: `AppConfig`
- `workloadconfig.md`: `WorkloadConfig`
- `service.md`: application-owned network `Service`
- `route.md`: application-owned `Route`
- `runtime-spec.md`: `RuntimeSpec`, `RuntimeSpecRevision`, `RuntimeObservedPod`, `RuntimeOperation`
- `manifest.md`: `Manifest`
- `intent.md`: `Intent`
- `release.md`: `Release`
- `image.md`: derived image-output contract owned by the release boundary

For shared CRUD, pagination, filtering, and soft-delete rules, start with `docs/policies/resource-api.md`.
Keep service-boundary and merge-status docs under `docs/services/`.
