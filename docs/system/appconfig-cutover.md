# AppConfig Hard Cutover

## Purpose

This document defines the repo-local execution checklist for the AppConfig hard cutover.
Use it when migrating from the legacy AppConfig schema to the new `application_id + environment_id` model.

## Scope

This cutover includes:

- AppConfig schema hard migration
- removal of legacy per-`name` AppConfig storage
- removal of stored `rendered_configmap`
- normalization of config mount path to `/etc/config`
- rollout of binaries that read the new AppConfig shape

This cutover does not include:

- changing the fixed GitHub config repo contract
- changing release rendering ownership
- changing WorkloadConfig schema

## Preconditions

Before cutover, confirm:

1. `bash scripts/verify.sh` passes from the repo root
2. the target binaries are built from the current repo state
3. legacy AppConfig writes can be paused during the migration window
4. the target PostgreSQL database is backed up or snapshotted
5. the fixed config repo layout already matches:

```text
{project_name}/{application_name}/{environment_name}
```

## Execution checklist

### 1. Pause legacy writes

- stop UI/API flows that still write legacy AppConfig payloads
- avoid creating or editing AppConfig records during the cutover window

### 2. Take backup

- take a database backup or storage snapshot before applying SQL
- make sure the backup includes `configurations` and `configuration_revisions`

### 3. Run hard-cutover SQL

```sh
psql "$DATABASE_URL" -f deployments/pre-production/database/appconfig-hard-cutover.sql
```

Expected result:

- one active `configurations` row per `application_id + env`
- new unique index `uq_configurations_app_env_active`
- legacy inline config fields removed
- legacy `rendered_configmap` removed
- `environment_app_config_bindings` removed
- default mount path normalized to `/etc/config`

### 4. Roll out binaries

Roll out at least:

- `config-service`
- `release-service`
- any process that reads mounted runtime config from `/etc/config`

Also confirm these manifests use `/etc/config` volume mounts:

- `deployments/pre-production/config-service.yaml`
- `deployments/pre-production/meta-service.yaml`
- `deployments/pre-production/network-service.yaml`
- `deployments/pre-production/release-service.yaml`
- `deployments/pre-production/runtime-service.yaml`

### 5. Re-sync AppConfig data

For each active application/environment pair:

- trigger `POST /api/v1/app-configs/{id}/sync-from-repo`
- verify `files`, `source_directory`, `source_commit`, and `latest_revision_*` are populated

### 6. Smoke test release flow

- create or reuse one test manifest
- create one test release
- confirm release rendering succeeds
- confirm mounted config directory is `/etc/config`
- confirm rendered ConfigMap content comes from AppConfig `files`

## Post-cutover validation

Check:

1. `GET /api/v1/app-configs?application_id=...&environment_id=...` returns exactly one active AppConfig
2. `mount_path` defaults to `/etc/config` when omitted
3. release rendering does not depend on `rendered_configmap`
4. config-service sync records `source_directory` and `source_commit`
5. no deployment still expects `/etc/devflow/config`

## Rollback plan

This is a hard cutover, so rollback should prefer database restore plus binary rollback.
Do not try to manually reintroduce dropped columns in-place during an incident.

Recommended rollback order:

1. stop new AppConfig writes
2. roll back service binaries to the pre-cutover version
3. restore the database from the pre-cutover backup/snapshot
4. restore old deployment manifests only if the previous binaries still require `/etc/devflow/config`
5. verify old AppConfig APIs and release flow are healthy again

## Related operator doc

- `docs/system/appconfig-cutover-runbook.md`

## Source pointers

- SQL cutover: `deployments/pre-production/database/appconfig-hard-cutover.sql`
- bootstrap schema: `deployments/pre-production/database/init.sql`
- AppConfig contract: `docs/resources/appconfig.md`
- PostgreSQL baseline: `docs/system/postgresql.md`
