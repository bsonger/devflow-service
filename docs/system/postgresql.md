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
- `deployments/pre-production/runtime-service.yaml`

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

## Init SQL source of truth

`deployments/pre-production/database/init.sql` started from the live `app` schema and was then corrected to match the current Go model and repository contract before being checked into the repo as the bootstrap baseline.

As of `2026-04-26`, the live schema includes:

- extension: `pgcrypto`
- 22 application tables
- corrected soft-delete support for `application_environment_bindings`
- corrected `projects` shape to match the current `Project` model and repository contract
- primary keys, unique indexes, support indexes, and foreign keys required by the current repository implementations

When repository-owned persistence changes, update the live schema and refresh `init.sql` together instead of letting bootstrap drift from production reality.
