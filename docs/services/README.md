# Services

## Purpose

This directory contains the current service-boundary docs for `devflow-service`.
Use these docs to answer one question first: which service owns which resource and runtime responsibility today?

## Standard service doc format

Most service docs now use this core structure:

1. `# <Service Name>`
2. `## Purpose`
3. `## Owns`
4. `## Does Not Own`
5. `## Dependency model`
6. `## Current implementation reality` when the service boundary differs from the target architecture or still uses same-repo implementation access
7. `## Downstream Consumers` when that section is separate
8. `## Entrypoint`
9. `## Registered Domains`
10. `## Pre-production Shared Ingress`
11. `## Resource Contracts`
12. `## Diagnostics`
13. `## Verification`

Some services also add service-specific sections such as:

- dependency detail by workflow
- dependency view diagrams
- operator flow descriptions
- pre-production delivery-path notes

The important rule is that dependency information should now live under `## Dependency model` rather than the older `## Upstream Dependencies` heading.

## Current fact requirements

Each service doc must make these distinctions explicit when they matter:

- target service boundary: the ownership model the repo is moving toward
- current implementation reality: what the code in this repo actually does today
- backend-local route surface: paths registered by the service router
- pre-production shared ingress surface: edge-facing paths exposed through `deployments/pre-production/istio/shared-ingress.yaml`
- not yet implemented behavior: planned or desired behavior that should not be read as current capability

Do not describe downstream HTTP validation, isolated storage, or runtime dependencies as current behavior unless the code path exists in this repo.

For shared ingress rewrite rules, use:

- `docs/system/ingress-routing.md`

## Current service docs

- `meta-service.md`
- `config-service.md`
- `network-service.md`
- `release-service.md`
- `runtime-service.md`

## Ownership rule

One resource belongs to exactly one active service boundary.
If a resource contract and a service doc disagree, fix the docs in the same change rather than leaving split ownership behind.

## Related docs

- `docs/resources/` for resource contracts, API behavior, and validation rules
- `docs/system/` for current repo-local execution truth
- `docs/policies/` for durable repo rules

## Notes

- These docs should describe the current code in this repo.
- Do not treat migrated material from sibling repos as authoritative if it conflicts with the current implementation here.
