# Config Service

This service boundary has been migrated into `devflow-service`.
Use this file as the repo-local summary for where config-owned behavior now lives in code.

## Owns

- `AppConfig`
- `WorkloadConfig`
- config repo sync semantics

These are the canonical owners after the config boundary extraction.
`meta-service` no longer serves `AppConfig` or `WorkloadConfig` after the hard cutover; these routes now belong only to `config-service`.

## Does Not Own

- `Project`
- `Application`
- `ApplicationEnvironment`
- `Cluster`
- `Environment`
- `Service`
- `Route`
- `Manifest`
- `Image`
- `Release`
- `Intent`
- `RuntimeSpec`

## Upstream Dependencies

- PostgreSQL
- centralized config repo
- shared backend primitives

## Downstream Consumers

- platform orchestration layers
- release-time consumers

## Current Repo Entry

`config-service` now boots as a separate runnable entrypoint in this repo.
Its current repo-local entrypoint lives at `cmd/config-service/main.go`.

Full path reference:

```text
cmd/config-service/main.go
```

The migrated implementation is split by domain:

```text
internal/appconfig/
internal/workloadconfig/
```

The current repo-local layout follows the monorepo policy:

```text
internal/appconfig/domain
internal/appconfig/service
internal/appconfig/repository
internal/appconfig/transport/http
internal/appconfig/transport/downstream
internal/appconfig/module.go
internal/workloadconfig/domain
internal/workloadconfig/service
internal/workloadconfig/repository
internal/workloadconfig/transport/http
internal/workloadconfig/module.go
```

Within the running process, these domains are registered through the `config-service` router and startup surfaces:

```text
cmd/config-service/main.go
internal/configservice/transport/http/router.go
```

The resource contracts owned by this boundary are documented at:

- `docs/resources/appconfig.md`
- `docs/resources/workloadconfig.md`

## Current AppConfig contract notes

- AppConfig is now unique by `application_id + environment_id`
- config sync source is the fixed GitHub config repository
- effective sync directory is derived as `{project_name}/{application_name}/{environment_name}`
- config-service must initialize `config_repo.root_dir` and `config_repo.default_ref` at runtime before sync requests are served
- the pre-production deployment mounts a writable `/tmp` volume and checks out the fixed repo under `/tmp/devflow-config-repo`
- `mount_path` defaults to `/etc/config`
- release-time rendering owns ConfigMap materialization; `AppConfig` no longer stores `rendered_configmap`
