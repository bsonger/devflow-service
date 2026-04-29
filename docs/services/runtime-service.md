# Runtime Service

## Purpose

`runtime-service` owns runtime desired state, runtime revisions, live observed pod state, and direct runtime operations.

The primary external value of this service is not database CRUD.
It is runtime inspection and runtime control against live Kubernetes workloads for one `application + environment`.

## Owns

- `RuntimeSpec`
- `RuntimeSpecRevision`
- `RuntimeObservedPod`
- `RuntimeOperation`
- runtime desired state for `application + environment`
- immutable runtime revisions
- live runtime observation responsibilities previously modeled as `resource-observer`
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

- Kubernetes API
- shared backend primitives

That is the important mental model for these flows:

- list application pod status
- delete one pod
- trigger one rollout / restart

### Current implementation note

The current codebase still contains PostgreSQL-backed runtime persistence for:

- `RuntimeSpec`
- `RuntimeSpecRevision`
- `RuntimeObservedPod`
- `RuntimeOperation`

So the present implementation is not yet fully aligned with the simpler Kubernetes-first dependency model.
Documenting that gap explicitly is important so readers do not confuse target API semantics with current storage internals.

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

### 1. List pod status

Runtime service receives:

- `application_id`
- `environment_id`

Then it should:

1. resolve the target workload in Kubernetes
2. read the live pod list
3. return current pod status for that application runtime

### 2. Delete one pod

Runtime service receives:

- `application_id`
- `environment_id`
- `pod_name`

Then it should:

1. resolve the target pod in Kubernetes
2. delete that pod
3. let the owning controller recreate or rebalance it

### 3. Trigger rollout / restart

Runtime service receives:

- `application_id`
- `environment_id`

Then it should:

1. resolve the target Deployment in Kubernetes
2. patch `kubectl.kubernetes.io/restartedAt`
3. let Kubernetes perform the rolling restart

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
