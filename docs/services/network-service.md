# Network Service

This file is migrated from `devflow-control` as a cross-repo reference.
It is ownership context only.

## Owns

- `Service`
- `Route`
- route validation for service-to-port targets

## Does Not Own

- `Project`
- `Application`
- `AppConfig`
- `WorkloadConfig`
- `Image`
- `Manifest`
- `Release`
- `Intent`

## Upstream Dependencies

- PostgreSQL
- shared backend primitives

## Downstream Consumers

- release-time consumers
- platform orchestration layers
