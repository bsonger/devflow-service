# AppConfig Cutover Runbook

## Audience

This runbook is for the operator performing the AppConfig production or pre-production cutover.
It is intentionally execution-oriented.

## Goal

Move AppConfig from the legacy schema to the new hard-cutover model:

- one active AppConfig per `application_id + environment_id`
- config payload stored in `configuration_revisions.files`
- no stored `rendered_configmap`
- default config mount path `/etc/config`

## Inputs

Prepare these before starting:

- database connection string in `DATABASE_URL`
- kubectl access to the target cluster
- rollout authority for:
  - `config-service`
  - `release-service`
  - any service still mounting config from the old path

## Files used during cutover

- SQL: `deployments/pre-production/database/appconfig-hard-cutover.sql`
- schema baseline: `deployments/pre-production/database/init.sql`
- operator checklist: `docs/system/appconfig-cutover.md`
- AppConfig contract: `docs/resources/appconfig.md`

## T-30 min

1. Confirm repo verification is green:

```sh
bash scripts/verify.sh
```

2. Confirm target manifests use `/etc/config`:

```sh
rg -n "mountPath: /etc/config" deployments/pre-production
```

3. Confirm no target manifest still uses `/etc/devflow/config`:

```sh
rg -n "/etc/devflow/config" deployments/pre-production internal || true
```

4. Announce write freeze for legacy AppConfig updates.

## T-10 min

1. Take a DB backup or storage snapshot.
2. Record current rollout revisions:

```sh
kubectl get deploy -n devflow-pre-production
```

3. Record current AppConfig row count:

```sh
psql "$DATABASE_URL" -c "select count(*) as active_configurations from configurations where deleted_at is null;"
```

## T0 - Apply database cutover

Run:

```sh
psql "$DATABASE_URL" -f deployments/pre-production/database/appconfig-hard-cutover.sql
```

Immediate checks:

```sh
psql "$DATABASE_URL" -c "select count(*) from configurations where deleted_at is null;"
psql "$DATABASE_URL" -c "select application_id, env, count(*) from configurations where deleted_at is null group by 1,2 having count(*) > 1;"
psql "$DATABASE_URL" -c "select count(*) from information_schema.columns where table_name='configurations' and column_name in ('name','description','format','data','labels','files');"
psql "$DATABASE_URL" -c "select count(*) from information_schema.columns where table_name='configuration_revisions' and column_name='rendered_configmap';"
```

Expected:

- no duplicate active `(application_id, env)` rows
- removed legacy columns return count `0`
- removed `rendered_configmap` returns count `0`

## T+5 min - Roll out services

Roll out updated workloads.
At minimum:

```sh
kubectl rollout restart deploy/config-service -n devflow-pre-production
kubectl rollout restart deploy/release-service -n devflow-pre-production
kubectl rollout restart deploy/meta-service -n devflow-pre-production
kubectl rollout restart deploy/network-service -n devflow-pre-production
kubectl rollout restart deploy/runtime-service -n devflow-pre-production
```

Wait for readiness:

```sh
kubectl rollout status deploy/config-service -n devflow-pre-production
kubectl rollout status deploy/release-service -n devflow-pre-production
kubectl rollout status deploy/meta-service -n devflow-pre-production
kubectl rollout status deploy/network-service -n devflow-pre-production
kubectl rollout status deploy/runtime-service -n devflow-pre-production
```

## T+10 min - Re-sync AppConfig

For each active AppConfig ID:

```sh
curl -X POST "$CONFIG_SERVICE_BASE_URL/api/v1/app-configs/{id}/sync-from-repo"
```

Validate one sample:

```sh
curl "$CONFIG_SERVICE_BASE_URL/api/v1/app-configs/{id}"
```

Expected sample response characteristics:

- `mount_path` is `/etc/config` or an explicit override
- `files` is populated
- `source_directory` is populated
- `source_commit` is populated
- `latest_revision_no` and `latest_revision_id` are populated

## T+20 min - Release smoke test

1. create one test release
2. wait for release rendering
3. confirm mounted config path is `/etc/config`
4. confirm rendered ConfigMap data matches AppConfig `files`

Suggested checks:

```sh
kubectl get pods -n devflow-pre-production
kubectl logs deploy/release-service -n devflow-pre-production --tail=200
kubectl logs deploy/config-service -n devflow-pre-production --tail=200
```

## Success criteria

The cutover is considered successful when all are true:

- DB cutover SQL completed without manual patching
- no duplicate active AppConfigs per `application_id + environment_id`
- AppConfig sync succeeds against the fixed GitHub repo layout
- release flow no longer depends on `rendered_configmap`
- active workloads mount config from `/etc/config`

## Fast rollback

If the cutover fails after SQL or rollout:

1. stop AppConfig writes
2. roll back service deployments to the previous revision
3. restore the database backup or snapshot taken before T0
4. if needed, restore old manifests expecting `/etc/devflow/config`
5. rerun health checks on old binaries

## Notes

- This is a hard cutover, not a compatibility migration.
- Prefer restore-based rollback over manual schema repair.
