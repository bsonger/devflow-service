# Services

## Purpose

This directory contains current service-boundary docs for `devflow-service`.

## What Each Service Doc Should Contain

Each service document should describe:

- the purpose of the service boundary
- which resources are owned by that service
- which resources are explicitly not owned by that service
- upstream dependencies
- downstream consumers
- the runnable entrypoint
- the registered code domains
- the pre-production shared ingress prefix
- the resource-contract docs owned by that service
- the main diagnostics and verification surfaces

## Current Service Docs

- `meta-service.md`
- `config-service.md`
- `network-service.md`
- `release-service.md`
- `runtime-service.md`

## Related Docs

- `docs/resources/` for resource contracts, API behavior, and validation rules
- `docs/system/` for current repo-local execution truth
- `docs/policies/` for durable repo rules

## Notes

- These docs should describe the current code in this repo.
- One resource should belong to exactly one service boundary.
- Do not treat migrated material from sibling repos as authoritative if it conflicts with the current implementation here.
