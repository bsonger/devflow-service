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
6. `## Downstream Consumers` when that section is separate
7. `## Entrypoint`
8. `## Registered Domains`
9. `## Pre-production Shared Ingress`
10. `## Resource Contracts`
11. `## Diagnostics`
12. `## Verification`

Some services also add service-specific sections such as:

- dependency detail by workflow
- dependency view diagrams
- operator flow descriptions
- pre-production delivery-path notes

The important rule is that dependency information should now live under `## Dependency model` rather than the older `## Upstream Dependencies` heading.

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
