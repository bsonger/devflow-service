# Runtime Service

## Reader routing

Start with `docs/system/flow-overview.md` when you need the authoritative stage contract for the full release lifecycle.
Use this document for the runtime-owned half of that contract:

- stage 7 — runtime observation and release writeback, where `runtime-service` observes live Kubernetes state and may send callbacks, but does not own release truth
- stage 8 — runtime operator actions, where `runtime-service` owns workload/pod read surfaces and explicit runtime mutations

This service doc is intentionally not the deploy-side source of truth for `Release`, Argo handoff, or writeback route ownership. For those, route to `docs/resources/release.md`, `docs/services/release-service.md`, and `docs/system/release-writeback.md`.

## Purpose

`runtime-service` owns runtime desired state, runtime revisions, runtime observed index state, and direct runtime operations.

The primary external value of this service is not database CRUD.
It is runtime inspection and runtime control against live Kubernetes workloads for one `application + environment`.

## Owns

- `RuntimeSpec`
- `RuntimeSpecRevision`
- `RuntimeObservedPod`
- `RuntimeObservedWorkload`
- `RuntimeOperation`
- runtime desired state for `application + environment`
- immutable runtime revisions
- live runtime observation responsibilities
- direct K8s pod lifecycle operations

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
- `Manifest`
- `Image`
- `Release`
- `Intent`

## Dependency model

### Target dependency model

For the operator-facing runtime API, `runtime-service` should be understood as depending on:

- runtime observer / index
- Kubernetes API
- shared backend primitives

That is the important mental model for these flows:

- show one application workload overview
- list application pod status
- delete one pod
- trigger one rollout / restart

Read vs write split:

- read surfaces should prefer runtime-owned observed index data
- action surfaces should call Kubernetes only when an operator explicitly performs an action

### Current implementation note

The active contract is Kubernetes-first:

- runtime reads come from runtime-owned observed state
- runtime actions mutate Kubernetes directly
- the default runtime HTTP service uses `internal/runtime/repository.RuntimeStore`, which currently defaults to the in-memory store
- the default runtime read path is rebuilt through observer sync rather than by loading runtime rows from PostgreSQL at boot

Important current nuance:

- the runtime index is not durable local storage inside `runtime-service`
- after restart, runtime state is expected to be rebuilt by the in-process observers
- release bundles now need runtime-relevant Kubernetes labels such as `devflow.application/id` and `devflow.environment/id` so the observer can reconstruct `application + environment` ownership from live workloads
- runtime-service active/runtime-domain storage is PostgreSQL-free
- shared platform startup outside `cmd/runtime-service` may still open PostgreSQL for other services
- release rollout observation is also started by the active runtime startup path, but it consumes the same in-memory runtime observer state instead of a runtime-domain PostgreSQL store
- when release writeback wiring is present, that rollout observer is a callback sender into `release-service`; it does not become the owner of release status, release steps, or writeback route policy

Do not read the current runtime contract as a repo-wide PostgreSQL removal.
For operator-facing workload and pod reads, the active path should be treated as observer/index-backed and memory-backed by default.

## Read and action split

Runtime-service should be read as two related surfaces:

### Read surface

- workload overview
- pod list
- backed by runtime-owned observer/index state

### Action surface

- delete pod
- rollout / restart workload
- calls Kubernetes only when an operator explicitly requests a mutation

This split is the main runtime-service contract and is more important than the internal storage model names.

## Downstream Consumers

- platform orchestration layers
- release-time consumers

## Entrypoint

Primary runnable entrypoint: `cmd/runtime-service/main.go`.

```text
cmd/runtime-service/main.go
```

## Registered Domains

```text
internal/runtime/domain
internal/runtime/repository
internal/runtime/service
internal/runtime/transport/http
```

## Pre-production Shared Ingress

- `/api/v1/runtime/...`

Internal observer callbacks are service-internal only and are not part of the shared-ingress external contract.

## Primary operator flows

### 1. Show workload overview

Runtime service receives:

- `application_id`
- `environment_id`

Then it should:

1. resolve the target runtime binding
2. read the latest observed workload summary from runtime-owned index storage
3. return one workload overview for that `application + environment`

This is the controller-level read surface.

### 2. List pod status

Runtime service receives:

- `application_id`
- `environment_id`

Then it should:

1. resolve the target runtime binding
2. read the latest observed pod list from runtime-owned index storage
3. return current pod status for that application runtime

This is the instance-level read surface.

### 3. Delete one pod

Runtime service receives:

- `application_id`
- `environment_id`
- `pod_name`

Then it should:

1. resolve the target pod in Kubernetes
2. delete that pod
3. let the owning controller recreate or rebalance it

### 4. Trigger rollout / restart

Runtime service receives:

- `application_id`
- `environment_id`

Then it should:

1. resolve the target Deployment in Kubernetes
2. patch `kubectl.kubernetes.io/restartedAt`
3. let Kubernetes perform the rolling restart

## External surface status

### Current external surface

- `GET /api/v1/runtime/workload`
- `GET /api/v1/runtime/pods`
- `DELETE /api/v1/runtime/pods/{pod_name}`
- `POST /api/v1/runtime/rollouts`

### Current read-model surface

Runtime workload overview now uses:

- `GET /api/v1/runtime/workload?application_id=...&environment_id=...`

This endpoint should return one workload overview from the same observer/index model already used for runtime pod display.
It should not directly query Kubernetes on every page load.

Suggested response emphasis:

- workload identity: `workload_kind`, `workload_name`, `namespace`
- replica status: `desired_replicas`, `ready_replicas`, `updated_replicas`, `available_replicas`, `unavailable_replicas`
- rollout health: `summary_status`, `conditions[]`, `observed_generation`
- deployment content summary: `images[]`
- timestamps: `observed_at`, optional `restart_at`

The intended UI split is:

- `runtime/workload` for controller-level summary
- `runtime/pods` for pod-level details
- runtime actions for explicit Kubernetes mutations

## Internal observer surface

Observer-side internal routes now include:

- `POST /api/v1/internal/runtime-workloads/sync`
- `POST /api/v1/internal/runtime-workloads/delete`
- `POST /api/v1/internal/runtime-pods/sync`
- `POST /api/v1/internal/runtime-pods/delete`

These routes are intended for observer/index writeback only.
They are not user-facing API routes.

Authentication note:

- these internal routes are protected by `X-Devflow-Observer-Token` when `observer.shared_token` is configured
- when `observer.shared_token` is empty, the middleware allows the request through

## Current storage reality

Current code should be read in two layers:

- operator-facing runtime read/action endpoints use the runtime service default store, which currently points at the in-memory `RuntimeStore`
- runtime-service active/runtime-domain storage is PostgreSQL-free, even though shared platform startup outside `cmd/runtime-service` may still open PostgreSQL for other services
- release rollout observer startup is active in `internal/runtime/config/config.go`, and that observer consumes runtime observer state plus Kubernetes labels rather than a runtime-domain PostgreSQL store

For the detailed storage model, see `docs/system/runtime-storage-model.md`.

## Resource Contracts

- `docs/resources/runtime-spec.md`

## Diagnostics

- `internal/runtime/transport/http/router.go`
- `internal/runtime/transport/http/handler.go`
- `internal/runtime/service/service.go`
- `internal/runtime/repository/repository.go`
- `internal/runtime/observer`
- `docs/system/release-writeback.md`

Runtime endpoints:

- `/healthz`
- `/readyz`
- `/internal/status`

## Verification

```sh
go test ./internal/runtime/...
go build -o bin/runtime-service ./cmd/runtime-service
bash scripts/verify.sh
```
