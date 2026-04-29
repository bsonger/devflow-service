# PostgreSQL

This document is the current repo-local authority for the Kubernetes PostgreSQL baseline used by `devflow-service`.

## Active cluster contract

- Kubernetes namespace: `database`
- CloudNativePG cluster name: `pg18-next`
- write service: `pg18-next-rw.database:5432`
- read service: `pg18-next-ro.database:5432`
- application database: `app`
- application owner: `app`
- PostgreSQL image baseline: `ghcr.io/cloudnative-pg/postgresql:18.3-minimal-trixie`

All current pre-production service manifests in this repo point to:

```text
postgresql://app:Sm3MQA5IVcPt1wUiOm1cpEU0w9vJBhPChjoUXcuCGY6cW4Yo9qUkqaNMf1OfrSIi@pg18-next-rw.database:5432/app
```

That DSN is currently present in:

- `deployments/pre-production/meta-service.yaml`
- `deployments/pre-production/config-service.yaml`
- `deployments/pre-production/network-service.yaml`
- `deployments/pre-production/release-service.yaml`

`runtime-service` is no longer part of the active PostgreSQL dependency contract.
The committed `deployments/pre-production/runtime-service.yaml` no longer contains PostgreSQL configuration; if a running cluster copy still does, treat that live manifest as stale drift and reconcile it back to the repo contract.

## Repo-managed install surfaces

The Kubernetes install and bootstrap artifacts for the new PostgreSQL 18 cluster now live under:

```text
deployments/pre-production/database/
```

Use these files:

- `kustomization.yaml` to generate the fixed bootstrap secret and init SQL ConfigMap
- `pg18-next-cluster.yaml` for the CloudNativePG cluster definition
- `init.sql` for the application schema bootstrap

Apply or reconcile the database stack with:

```sh
kubectl apply -k deployments/pre-production/database
```

## Reinstall flow

The repo now supports a deterministic install path for a new parallel PostgreSQL 18 cluster:

1. Keep the existing `pg18` cluster in place.
2. Apply the repo-managed Kubernetes manifests from `deployments/pre-production/database/`.
3. Wait for the `pg18-next-rw`, `pg18-next-r`, and `pg18-next-ro` services to become ready.
4. Reconcile or restart the application deployments so they reconnect to `pg18-next-rw.database:5432`.

The bootstrap flow uses `bootstrap.initdb.postInitApplicationSQLRefs`, so `init.sql` is executed only when the cluster is created from scratch.
Applying the manifest to an already-running cluster will not replay schema creation.

For an existing legacy database, run the checked-in hard-cutover SQL before rolling out the new AppConfig binaries:

```sh
psql "$DATABASE_URL" -f deployments/pre-production/database/appconfig-hard-cutover.sql
```

That cutover script consolidates legacy per-`name` AppConfig rows into one active record per `application_id + environment_id`, rewrites the latest revision payload to the new `files` model, drops obsolete `rendered_configmap` storage, removes the legacy `environment_app_config_bindings` table, and normalizes the default mount path to `/etc/config`.

Detailed execution and rollback steps live in `docs/system/appconfig-cutover.md`.

Recommended rollout order:

1. stop or pause writes to legacy AppConfig APIs
2. run `deployments/pre-production/database/appconfig-hard-cutover.sql` against the target database
3. roll out `config-service`, `release-service`, and any process reading config files from `/etc/config`
4. trigger AppConfig repo sync again for active application/environment pairs
5. create one release smoke test and verify rendered workload mounts `/etc/config`

## Init SQL source of truth

`deployments/pre-production/database/init.sql` started from the live `app` schema and was then corrected to match the current Go model and repository contract before being checked into the repo as the bootstrap baseline.

As of `2026-04-28`, the live schema includes:

- extension: `pgcrypto`
- application tables aligned to the current extracted service boundaries
- corrected soft-delete support for `application_environment_bindings`
- corrected `projects` shape to match the current `Project` model and repository contract
- `manifests` reduced to the current build-record model only:
  - kept: `application_id`, `git_revision`, `repo_address`, `commit_hash`, `image_ref`, `image_tag`, `image_digest`, `pipeline_id`, `trace_id`, `span_id`, `steps`, `services_snapshot`, `workload_config_snapshot`, `status`
  - removed legacy release-era columns such as `environment_id`, `image_id`, `routes_snapshot`, `app_config_snapshot`, `rendered_yaml`, `rendered_objects`, `artifact_*`, and `tag`
- `releases` aligned to the current release-service model:
  - kept `env`, `manifest_id`, `routes_snapshot`, `app_config_snapshot`, `strategy`, `artifact_*`, `argocd_application_name`, `external_ref`, `steps`, `status`
  - removed legacy `image_id`
- `configurations` aligned to the new AppConfig model:
  - one active record per `application_id + env`
  - `mount_path` default normalized to `/etc/config`
  - payload stored through `configuration_revisions.files` plus `source_commit` / `source_digest`
  - removed legacy inline `name`, `description`, `format`, `data`, `labels`, and `rendered_configmap` storage
- obsolete `environment_app_config_bindings` removed from the active schema
- primary keys, unique indexes, support indexes, and foreign keys required by the current repository implementations

When repository-owned persistence changes, update the live schema and refresh `init.sql` together instead of letting bootstrap drift from production reality.
