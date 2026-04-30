# Release Service

## Reader routing

Start with `docs/system/flow-overview.md` when you need the authoritative stage contract for the end-to-end release lifecycle.
Use this document after that to inspect the service that owns stages 2 through 6 and the release-side callback surface for stage 7:

- stage 2 — manifest freeze and build dispatch
- stage 3 — release freeze
- stage 4 — release bundle render
- stage 5 — bundle publish
- stage 6 — release execution handoff / Argo deployment
- stage 7 — release-owned writeback surface and release truth persistence after runtime observation

This service doc is intentionally not the top-level lifecycle explainer. It is the owner/diagnostics guide for the release-owned stages.

## Purpose

`release-service` owns the build-to-deploy handoff records and the deployment execution flow: it creates the build-side `Manifest` record, creates the deploy-side `Release` record, and owns the callback surface that updates deploy progress.

It is also the main cross-service orchestration boundary for build and deploy.
It does not own upstream resource truth such as application metadata, app config, workload config, services, or routes, but it composes those facts into two different release-owned freeze points:

- `Manifest` for the build-side record and image-delivery trace
- `Release` for the deploy-side environment bind, bundle publication, and rollout state

## Owns

- build-side `Manifest`
- workload `Image` result recorded on the manifest
- deploy-side `Release`
- deploy `Intent`
- build and release lifecycle records around image build, deployment bundle render, and deployment bundle publication
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

- PostgreSQL for persisted `Manifest` and `Release` records
- Tekton for build execution tied back to the build-side `Manifest`
- Argo CD for deploying the deploy-side `Release` bundle
- Kubernetes API
- OCI registry for deploy-side bundle publication in pre-production (`zot`)

Historical naming note:

- the runtime/config surface still uses the legacy `manifest_registry` key and helper names
- in the current code path that naming refers to the registry target for release deployment bundle publication, not ownership of the `Manifest` API resource

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
5. persist one frozen build-side manifest record in PostgreSQL

This means `Manifest` is a release-owned build-side record, but some of its frozen inputs come from other services.
It is the inspection surface for build identity, frozen workload/service inputs, Tekton progress writeback, and final workload image output.

### Release create path

When creating a release, `release-service` composes these upstream facts:

1. read frozen manifest from release-owned persistence
2. read app config from `config-service`
3. read route list from `network-service`
4. resolve application / environment / cluster deploy target from `meta-service`
5. freeze those live inputs onto the release row
6. render, publish, and deploy the release bundle

This means `Release` is the release-owned deploy-side record.
It is the inspection surface for environment binding, rendered deployment bundle facts, published OCI artifact metadata, Argo CD handoff, and rollout/writeback status.

## Rollout observation boundary

`release-service` should be understood as the deployment initiator and release-truth owner, not the rollout observer.

Target boundary:

1. `release-service` creates or updates the Argo CD `Application`
2. Argo CD syncs the release-owned OCI bundle into Kubernetes
3. `runtime-service` may observe rollout state from Kubernetes and send token-gated callbacks when its clustered observer path is wired
4. `release-service` does not poll Argo CD application status during normal release detail reads
5. rollout progress writeback, when used, comes through release-owned writeback routes
6. those writeback routes are part of the release boundary, not a public runtime API surface

See also:

- `docs/system/release-writeback.md` for the callback contract
- `docs/services/runtime-service.md` for the runtime observer/read-model side of the same seam

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

Historical naming note:

- `manifest_registry` is the legacy config block name kept for compatibility
- in the active code path it configures release deployment bundle publication
- it does not mean the registry owns or stores the `Manifest` resource contract itself

Because release bundle repository paths are application-scoped under the `releases/` prefix, Argo CD should be configured with a repo-creds prefix secret rather than a single fixed repository entry.

## Verification

```sh
go test ./internal/manifest/... ./internal/intent/... ./internal/release/...
go build -o bin/release-service ./cmd/release-service
bash scripts/verify.sh
```
