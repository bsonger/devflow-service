# Config Service

This file is migrated from `devflow-control` as a cross-repo reference.
It is ownership context, not current implementation authority for this repo.

## Owns

- `AppConfig`
- `WorkloadConfig`
- config repo sync semantics

## Does Not Own

- `Project`
- `Application`
- `Manifest`
- `Image`
- `Release`
- `Intent`

## Upstream Dependencies

- PostgreSQL
- centralized config repo
- shared backend primitives

## Downstream Consumers

- platform orchestration layers
- release-time consumers
