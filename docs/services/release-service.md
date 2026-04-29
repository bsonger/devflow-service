# Release Service

## Purpose

`release-service` owns build artifact records, deployment intent, release execution, and verify/writeback callbacks.

It is also the main cross-service orchestration boundary for build and deploy.
It does not own upstream resource truth such as application metadata, app config, workload config, services, or routes, but it composes those facts into frozen manifest and release records.

## Owns

- `Manifest`
- `Image`
- `Release`
- `Intent`
- build and release lifecycle records around manifest OCI deployment artifacts
- verify ingress and verification writeback responsibilities previously modeled as `verify-service`

## Does Not Own

- `Project`
- `Application`
- `ApplicationEnvironment`
- `Cluster`
- `Environment`
- `AppConfig`
- `WorkloadConfig`
- `Service`
- `Route`
- `RuntimeSpec`
- `RuntimeSpecRevision`
- `RuntimeObservedPod`
- `RuntimeOperation`

## Dependency model

`release-service` depends on both persisted release data and upstream service truth.

### Control-plane and persistence dependencies

- PostgreSQL
- Tekton
- Argo CD
- Kubernetes API
- OCI registry for deployment bundle publication in pre-production (`zot`)

### Upstream business dependencies

- `meta-service`
  - application metadata
  - environment metadata
  - cluster metadata
  - deploy target resolution
- `config-service`
  - workload config during manifest creation
  - app config during release creation
- `network-service`
  - service topology during manifest creation
  - route topology during release creation

## Dependency detail by workflow

### Manifest create path

When creating a manifest, `release-service` composes these upstream facts:

1. read application projection from `meta-service`
2. read workload config from `config-service`
3. read service list from `network-service`
4. derive image target and submit Tekton build
5. persist one frozen manifest record in PostgreSQL

This means `Manifest` is a release-owned record, but some of its frozen inputs come from other services.

### Release create path

When creating a release, `release-service` composes these upstream facts:

1. read frozen manifest from release-owned persistence
2. read app config from `config-service`
3. read route list from `network-service`
4. resolve application / environment / cluster deploy target from `meta-service`
5. freeze those live inputs onto the release row
6. render, publish, and deploy the release bundle

This means `Release` is also release-owned, but it is intentionally assembled from cross-service inputs at freeze time.

## Rollout observation boundary

`release-service` should be understood as the deployment initiator, not the rollout observer.

Target boundary:

1. `release-service` creates or updates the Argo CD `Application`
2. Argo CD syncs the release-owned OCI bundle into Kubernetes
3. `release-service` does not poll Argo CD application status during normal release detail reads
4. rollout progress writeback, when used, should come through token-gated release writeback routes
5. those writeback routes are part of the release boundary, not a public runtime API surface

## Downstream Consumers

- platform orchestration layers
- verify-time consumers

## Entrypoint

Primary runnable entrypoint: `cmd/release-service/main.go`.

```text
cmd/release-service/main.go
```

## Registered Domains

```text
internal/manifest/
internal/intent/
internal/release/
```

## Pre-production Shared Ingress

- `/api/v1/release/...`

## Resource Contracts

- `docs/resources/manifest.md`
- `docs/resources/image.md`
- `docs/resources/intent.md`
- `docs/resources/release.md`

Operational callback contract:

- `docs/system/release-writeback.md`
- `docs/system/release-steps.md`

## Diagnostics

- `internal/release/transport/http/router.go`
- `internal/manifest/transport/http`
- `internal/intent/transport/http`
- `internal/release/service`
- `internal/release/support`
- `docs/policies/worker-runtime.md`

Runtime endpoints:

- `/healthz`
- `/readyz`
- `/internal/status`

## Pre-production OCI deployment bundle flow

The current pre-production deployment path for release execution is:

1. `release-service` renders one canonical deployment bundle for the release.
2. `publish_bundle` packages that bundle as a single OCI tar.gz layer and pushes it to the configured OCI registry.
3. The pre-production committed registry target is the in-cluster `zot` service, not the `zot-0` pod name.
4. `create_argocd_application` creates an Argo CD `Application` whose source points at the published OCI artifact.
5. Argo CD pulls the OCI artifact and syncs it into the target namespace.

The committed pre-production config now expects:

- `manifest_registry.registry = zot.zot.svc.cluster.local:5000`
- `manifest_registry.namespace = devflow`
- `manifest_registry.repository = releases`
- `manifest_registry.plain_http = true`
- `manifest_registry.mode = oras`

Because release bundle repository paths are application-scoped under the `releases/` prefix, Argo CD should be configured with a repo-creds prefix secret rather than a single fixed repository entry.

## Verification

```sh
go test ./internal/manifest/... ./internal/intent/... ./internal/release/...
go build -o bin/release-service ./cmd/release-service
bash scripts/verify.sh
```
