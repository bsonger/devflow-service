# Network Service

This service boundary has been migrated into `devflow-service`.
Use this file as the repo-local summary for where network-owned behavior now lives in code.

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

## Current Repo Entry

`network-service` now boots as a separate runnable entrypoint in this repo.
Its current repo-local entrypoint lives at `cmd/network-service/main.go`.

Full path reference:

```text
cmd/network-service/main.go
```

The migrated implementation is split by domain:

```text
internal/service/
internal/route/
```

The current repo-local layout follows the monorepo policy:

```text
internal/service/domain
internal/service/service
internal/service/repository
internal/service/transport/http
internal/service/transport/downstream
internal/service/module.go
internal/route/domain
internal/route/service
internal/route/repository
internal/route/transport/http
internal/route/module.go
```

Within the running process, these domains are registered through the `network-service` router and startup surfaces:

```text
cmd/network-service/main.go
internal/networkservice/transport/http/router.go
```

The resource contracts owned by this boundary are documented at:

- `docs/resources/service.md`
- `docs/resources/route.md`
