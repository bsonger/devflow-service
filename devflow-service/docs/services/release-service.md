# Release Service

This file is migrated from `devflow-control` as a cross-repo reference.
It is ownership context, not current implementation authority for this repo.

## Owns

- `Manifest`
- `Image`
- `Release`
- `Intent`
- build and release lifecycle records around frozen manifest snapshots and OCI deployment artifacts
- verify ingress and verification writeback responsibilities that were previously modeled as `verify-service`

## Does Not Own

- `Project`
- `Application`
- `AppConfig`
- `WorkloadConfig`
- `Service`
- `Route`
- runtime desired-state semantics

## Upstream Dependencies

- PostgreSQL
- runtime service
- Tekton
- Argo CD
- Kubernetes API

## Downstream Consumers

- platform orchestration layers
- verify-time consumers

## Current Merge Note

`verify-service` is no longer treated as a separate service summary in this repo.
Its ingress and verification concerns are now considered part of the broader `release-service` ownership boundary.
