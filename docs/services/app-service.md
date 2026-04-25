# App Service

This service boundary has been migrated into `devflow-service`.
Use this file as the repo-local summary for where app-owned behavior now lives in code.

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

## Current Repo Entry

`app-service` is no longer modeled as a separate runnable entrypoint in this repo.
Its owned resources now live inside the root `meta-service` runtime and route assembly.

The migrated implementation is split by domain:

```text
internal/project/
internal/application/
internal/cluster/
internal/environment/
```

Within the running process, these domains are registered through the `meta-service` router and startup surfaces:

```text
cmd/meta-service/main.go
internal/app/router.go
```

The resource contracts owned by this boundary are documented at:

- `docs/resources/project.md`
- `docs/resources/application.md`
- `docs/resources/cluster.md`
- `docs/resources/environment.md`

The current repo-local mapping is:
- `internal/project`
- `internal/application`
- `internal/cluster`
- `internal/environment`
