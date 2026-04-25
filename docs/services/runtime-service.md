# Runtime Service

This file is migrated from `devflow-control` as a cross-repo reference.
It is ownership context only.

## Owns

- `RuntimeSpec`
- `RuntimeSpecRevision`
- runtime desired state for `application + environment`
- immutable runtime revisions
- live runtime observation responsibilities that were previously modeled as `resource-observer`

## Does Not Own

- image version
- release execution state
- rollout strategy

## Upstream Dependencies

- PostgreSQL
- shared backend primitives

## Downstream Consumers

- release-time consumers
- platform orchestration layers

## Current Merge Note

`resource-observer` is no longer treated as a separate service summary in this repo.
Its live observation and runtime writeback concerns are now considered part of the broader `runtime-service` ownership boundary.
