# App Service

This file is migrated from `devflow-control` as a cross-repo reference.
Use it for ownership context only. Current repo-local implementation truth still lives in `meta-service` docs and code.

## Owns

- `Project`
- `Application`
- `Cluster`
- `Environment`
- `Application.active_image` binding

## Does Not Own

- `Service`
- `Route`
- `AppConfig`
- `WorkloadConfig`
- `Manifest`
- `Image`
- `Release`
- `Intent`

## Upstream Dependencies

- PostgreSQL
- shared backend primitives

## Downstream Consumers

- platform-facing orchestration layers
- release-time consumers that need application metadata

## Repo-Local Mapping Note

In the current `devflow-service` migration, these app-owned resource concepts live under:
- `internal/project`
- `internal/application`
- `internal/cluster`
- `internal/environment`
