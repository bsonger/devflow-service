# Runtime Storage Model

## Purpose

This document records the current runtime-service storage behavior.
Use it when deciding whether runtime reads, actions, observers, or rollout checks are backed by memory, Kubernetes, or PostgreSQL.

## Current default store

The default runtime HTTP service is memory-backed:

- `internal/runtime/repository/repository.go` sets `RuntimeStore = NewMemoryStore()`
- `internal/runtime/service/service.go` sets `DefaultService = New(repository.RuntimeStore, nil)`
- `internal/runtime/transport/http/router.go` wires HTTP handlers to `runtimeservice.DefaultService`

Operator-facing runtime reads and actions therefore do not load runtime state from PostgreSQL by default.

## Runtime read path

The runtime API reads from the active runtime store:

- `GET /api/v1/runtime/workload`
- `GET /api/v1/runtime/pods`
- `DELETE /api/v1/runtime/pods/{pod_name}`
- `POST /api/v1/runtime/rollouts`

For the default service, workload and pod state comes from the in-process memory store.
The service does not query Kubernetes directly for every read request.

## Observer rebuild path

`internal/runtime/observer/kubernetes_runtime.go` rebuilds runtime state from Kubernetes watch/list events.
It uses `repository.RuntimeStore`, which is memory-backed by default.

The observer:

- watches Deployments and Pods in Kubernetes
- requires release labels such as `devflow.application/id` and `devflow.environment/id`
- calls `EnsureRuntimeSpecByApplicationEnv`
- writes observed workloads and pods into the runtime store

After a service restart, memory-backed runtime state starts empty until observer sync repopulates it.

## Runtime action path

Runtime actions are executed through the runtime service and Kubernetes executor:

- pod delete resolves the observed pod and deletes it through Kubernetes
- workload restart resolves the observed workload and restarts the Deployment through Kubernetes

Namespace and workload identity are resolved from stored observed runtime state.
If observer state is missing or stale, actions can fail even when the workload still exists in Kubernetes.

## PostgreSQL-backed code still exists

Do not describe runtime-service as fully database-free.
The repository still contains a PostgreSQL-backed runtime store implementation:

- `internal/runtime/repository/repository.go`
- `internal/runtime/repository.NewPostgresStore`

The release rollout observer also still uses PostgreSQL-backed support:

- `internal/runtime/observer/release_rollout.go`
- direct release-table reads through `platformdb.Postgres()`

This means the current default runtime HTTP path is memory-backed, while some runtime-adjacent support code remains PostgreSQL-backed.

## Documentation rule

When documenting runtime-service:

- say "default runtime HTTP service is memory-backed" instead of "runtime-service has no PostgreSQL code"
- say "observer sync rebuilds in-memory state" instead of "runtime reads directly from Kubernetes"
- mention the remaining PostgreSQL-backed repository and rollout observer where storage boundaries matter
- keep shared ingress paths separate from backend-local runtime routes; see `docs/system/ingress-routing.md`
