# Runtime Service

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
- runtime-service no longer depends on PostgreSQL for startup or request handling
- runtime-service keeps its active runtime index in-process
- runtime-service rebuilds that in-process index through observer sync rather than by loading rows from PostgreSQL at boot

Important current nuance:

- the runtime index is not durable local storage inside `runtime-service`
- after restart, runtime state is expected to be rebuilt by the in-process observers
- release bundles now need runtime-relevant Kubernetes labels such as `devflow.application/id` and `devflow.environment/id` so the observer can reconstruct `application + environment` ownership from live workloads

Any older PostgreSQL-oriented description should be treated as historical and non-owning.

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

## Pre-production status

As of April 29, 2026:

- pre-production runtime-service has been updated to serve `GET /api/v1/runtime/workload`
- public workload overview reads are working through shared ingress
- pre-production runtime observation is owned by the in-process Kubernetes observer inside runtime-service
- pre-production runtime-service rebuilds runtime workload/pod state from observer sync instead of PostgreSQL-backed startup state
- runtime-service should be treated as PostgreSQL-independent in the active contract for startup and request handling

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
