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
internal/appservice/
internal/approute/
```

The current repo-local layout follows the monorepo policy:

```text
internal/appservice/domain
internal/appservice/service
internal/appservice/repository
internal/appservice/transport/http
internal/appservice/transport/downstream
internal/appservice/module.go
internal/approute/domain
internal/approute/service
internal/approute/repository
internal/approute/transport/http
internal/approute/module.go
```

Within the running process, these domains are registered through the `network-service` router and startup surfaces:

```text
cmd/network-service/main.go
internal/networkservice/transport/http/router.go
```

The resource contracts owned by this boundary are documented at:

- `docs/resources/service.md`
- `docs/resources/route.md`
