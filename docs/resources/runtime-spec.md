# Runtime API

## Ownership

- active service boundary: `runtime-service`
- runnable host process: `runtime-service`
- primary external surface: runtime inspection and runtime operations
- current implementation packages:
  - `internal/runtime/transport/http`
  - `internal/runtime/service`
  - `internal/runtime/repository`
  - `internal/runtime/domain`

## Purpose

This runtime surface is primarily the application runtime inspection and operation API.
Start with `docs/system/flow-overview.md` when you need the full release-lifecycle owner map, then use this document for the runtime-owned read/action surface that consumes release-owned metadata.
Its core jobs are:

- show the current workload overview for one `application + environment`
- list the live pod status for one `application + environment`
- delete a specific pod for that application runtime
- trigger a rollout-style restart for that application workload

## Quick reader guide

Use this document when you need to answer runtime-side questions such as:

- what workload is currently running for one `application + environment`
- what pods are currently observed
- which runtime actions are supported
- which routes read observer/index data versus which routes mutate Kubernetes

If your question is instead about:

- which image was built
- which build pipeline ran
- which deployment bundle was published
- how Argo CD rollout progressed

then the owning resource is `Manifest` or `Release`, not the runtime surface.

From the external API point of view, this service should be understood as a runtime operations surface first.
The internal storage model may still use `RuntimeSpec`, `RuntimeSpecRevision`, `RuntimeObservedPod`, and `RuntimeOperation`, but those are supporting implementation details rather than the primary API story.
For operator understanding, the preferred model is:

- display reads from runtime-owned observed index data
- explicit actions call Kubernetes

The current observed index now includes:

- `RuntimeObservedWorkload`
- `RuntimeObservedPod`
- `RuntimeOperation`

Current implementation note:

- the default runtime HTTP service uses an in-memory `RuntimeStore`
- the active runtime index is kept in-process
- observer sync rebuilds that in-process state after restart
- runtime-service active/runtime-domain storage is PostgreSQL-free
- release rollout observation is also started by the active runtime startup path and consumes runtime observer state plus Kubernetes labels
- shared platform startup outside `cmd/runtime-service` may still open PostgreSQL for other services
- release-generated workloads should carry this stable release-owned label set so runtime-service can recover workload ownership and rollout correlation from live Kubernetes resources:
  - `app.kubernetes.io/name`
  - `devflow.io/release-id`
  - `devflow.application/id`
  - `devflow.environment/id`
- workload and pod correlation require matching application/environment labels plus a non-empty release ID; `app.kubernetes.io/name` alone is not sufficient to establish runtime identity
- annotations are supplementary only and must not be required for release, application, or environment identity recovery
- runtime-service may send rollout callbacks into `release-service`, but it does not own release truth

For the full storage boundary, see `docs/system/runtime-storage-model.md`.

## Boundary summary

`Runtime API` is the runtime-side read/action boundary.

It owns:

- workload overview read model
- pod list read model
- pod delete action
- rollout / restart action
- observer/index-backed runtime state

It does not own:

- source/build records
- deployment bundle publication
- Argo CD application orchestration
- build-time or deploy-time freeze records

## Main operator flows

### 1. Show application workload overview

Primary read flow:

1. caller provides `application_id` and `environment_id`
2. runtime-service resolves the target runtime binding from observer/index-owned identity
3. runtime-service reads the latest observed workload summary from runtime-owned index data
4. runtime-service returns one workload overview

This is the controller-level runtime read surface.

Failure contract:

- `not_found` when observer/index identity for the requested `application + environment` does not exist yet
- `failed_precondition` when namespace resolution fails or workload correlation remains ambiguous after applying the release-owned label contract
- callers must not treat these failures as permission to bypass the observer/index model with direct Kubernetes reads

### 2. List application pod status

Primary read flow:

1. caller provides `application_id` and `environment_id`
2. runtime-service resolves the target runtime binding from observer/index-owned identity
3. runtime-service reads the latest observed pod list from runtime-owned index data
4. runtime-service returns the current pod list and pod status snapshot

This is the instance-level runtime read surface.

Failure contract:

- `not_found` when observer/index identity or observed pod state is absent for the requested target
- `failed_precondition` when namespace resolution fails before the runtime target can be addressed truthfully
- successful empty reads are not a substitute for missing observer state

### 3. Delete one pod

Primary action flow:

1. caller chooses one pod under one `application + environment`
2. runtime-service resolves the runtime namespace and verifies the requested pod is present in observer-owned pod state for that exact target
3. runtime-service deletes the pod in Kubernetes only after that observer-backed identity check succeeds
4. Kubernetes recreates or rebalances it according to the owning controller

This action is for one concrete pod, not for the whole application rollout.

Failure contract:

- `not_found` when observer-owned runtime identity for the requested `application + environment` does not exist yet
- `failed_precondition` when namespace resolution fails or the requested pod is not present in observer-backed state for the resolved target
- downstream Kubernetes `not_found` should only surface after the target was already resolved truthfully at the runtime boundary

### 4. Trigger rollout / restart

Primary action flow:

1. caller provides `application_id` and `environment_id`
2. runtime-service resolves the target workload Deployment
3. runtime-service patches the Deployment with `kubectl.kubernetes.io/restartedAt`
4. Kubernetes performs the rolling restart

Current implementation note:

- the runtime action is implemented as a Deployment restart
- in product language, this can be described as triggering a rollout or restart for the application workload
- a successful runtime action response acknowledges Kubernetes mutation acceptance plus persisted runtime operation metadata; it does not claim that rollout observation or release convergence has already completed

## Question routing

Use the runtime surface when the question starts with:

- what is running now
- what pods are unhealthy now
- restart this workload
- delete this pod

Use `Release` when the question starts with:

- what was deployed
- what config was frozen for deployment
- what deployment artifact was published
- what happened during rollout

Use `Manifest` when the question starts with:

- what was built
- which commit was built
- what image came out of build

## API surface

Current service route surface:

- `GET /api/v1/runtime/workload?application_id=...&environment_id=...`
- `GET /api/v1/runtime/pods?application_id=...&environment_id=...`
- `DELETE /api/v1/runtime/pods/{pod_name}`
- `POST /api/v1/runtime/rollouts`

Current pre-production shared ingress external surface:

- `GET /api/v1/runtime/workload?application_id=...&environment_id=...`
- `GET /api/v1/runtime/pods?application_id=...&environment_id=...`
- `DELETE /api/v1/runtime/pods/{pod_name}`
- `POST /api/v1/runtime/rollouts`

Selector placement follows the repo-wide policy:

- `GET` uses query filters
- `DELETE` uses request body when the path alone is not enough
- `POST` uses request body

## Request contracts

### Show workload overview

```http
GET /api/v1/runtime/workload?application_id=999c0c88-1f1f-41d1-a67a-8159d07c878c&environment_id=b780ca97-a213-4763-bfb9-43f7e3a11ee7
```

Required query parameters:

- `application_id`
- `environment_id`

Implementation status:

- this endpoint is part of the current preferred runtime read surface
- it should read runtime-owned observed workload index data
- pre-production shared ingress has been verified to return workload overview data

### List pod status

```http
GET /api/v1/runtime/pods?application_id=999c0c88-1f1f-41d1-a67a-8159d07c878c&environment_id=b780ca97-a213-4763-bfb9-43f7e3a11ee7
```

Required query parameters:

- `application_id`
- `environment_id`

### Delete pod

```http
DELETE /api/v1/runtime/pods/demo-api-7c8d9f5c6b-abcde
```

Request body:

```json
{
  "application_id": "999c0c88-1f1f-41d1-a67a-8159d07c878c",
  "environment_id": "b780ca97-a213-4763-bfb9-43f7e3a11ee7",
  "operator": "songbei"
}
```

Required fields:

- `application_id`
- `environment_id`

Optional fields:

- `operator`

### Trigger rollout / restart

```http
POST /api/v1/runtime/rollouts
```

Request body:

```json
{
  "application_id": "999c0c88-1f1f-41d1-a67a-8159d07c878c",
  "environment_id": "b780ca97-a213-4763-bfb9-43f7e3a11ee7",
  "operator": "songbei"
}
```

Required fields:

- `application_id`
- `environment_id`

Optional fields:

- `deployment_name`
- `operator`

Resolution note:

- runtime-service now accepts only two rollout target sources:
  1. explicit `deployment_name`
  2. current observed workload name when the observed workload kind is `Deployment`
- if neither source yields one confident Deployment target, the action fails with `failed_precondition`
- callers may still send `deployment_name` explicitly when they want deterministic targeting

## Internal observer sync surface

These routes are service-internal observer callbacks and are not part of the shared external ingress contract:

- `POST /api/v1/internal/runtime-workloads/sync`
- `POST /api/v1/internal/runtime-workloads/delete`
- `POST /api/v1/internal/runtime-pods/sync`
- `POST /api/v1/internal/runtime-pods/delete`

Authentication note:

- these routes use `X-Devflow-Observer-Token` when `observer.shared_token` is configured
- if `observer.shared_token` is empty, the middleware allows the request through

### Sync workload summary

```http
POST /api/v1/internal/runtime-workloads/sync
```

Request body shape:

```json
{
  "application_id": "999c0c88-1f1f-41d1-a67a-8159d07c878c",
  "environment": "production",
  "namespace": "devflow-pre-production",
  "workload_kind": "Deployment",
  "workload_name": "meta-service",
  "desired_replicas": 1,
  "ready_replicas": 1,
  "updated_replicas": 1,
  "available_replicas": 1,
  "unavailable_replicas": 0,
  "observed_generation": 9,
  "summary_status": "Healthy",
  "images": [
    "registry.cn-hangzhou.aliyuncs.com/devflow/meta-service:preproduction"
  ],
  "conditions": [
    {
      "type": "Available",
      "status": "True",
      "reason": "MinimumReplicasAvailable",
      "message": "Deployment has minimum availability."
    }
  ],
  "labels": {
    "app.kubernetes.io/name": "meta-service"
  },
  "annotations": {
    "kubectl.kubernetes.io/restartedAt": "2026-04-29T04:08:52Z"
  },
  "observed_at": "2026-04-29T06:35:00Z",
  "restart_at": "2026-04-29T04:08:52Z"
}
```

Validation notes:

- `application_id`, `environment`, `workload_kind`, and `workload_name` are required
- namespace must match the runtime namespace derived by runtime-service
- sync is idempotent for the same `application + environment + namespace + workload_kind + workload_name` target and updates the latest observed workload summary
- labels should include the stable release-owned identity contract used by the observers:
  - `app.kubernetes.io/name`
  - `devflow.io/release-id`
  - `devflow.application/id`
  - `devflow.environment/id`
- annotations are supplementary only; identity recovery and rollout correlation must continue to work from labels alone

### Delete workload summary

```http
POST /api/v1/internal/runtime-workloads/delete
```

Request body shape:

```json
{
  "application_id": "999c0c88-1f1f-41d1-a67a-8159d07c878c",
  "environment": "production",
  "namespace": "devflow-pre-production",
  "workload_kind": "Deployment",
  "workload_name": "meta-service",
  "observed_at": "2026-04-29T06:40:00Z"
}
```

## Response focus

### Workload overview response should emphasize

- `application_id`
- `environment`
- `namespace`
- `workload_kind`
- `workload_name`
- `desired_replicas`
- `ready_replicas`
- `updated_replicas`
- `available_replicas`
- `unavailable_replicas`
- `observed_generation`
- `summary_status`
- `images[]`
- `conditions[]`
- `observed_at`
- optional `restart_at`

One workload overview should represent the primary controller for one `application + environment`.
If a different deployable shape is needed, that should normally correspond to a different release / runtime target instead of multiple primary workload records in one overview response.

### Pod list response should emphasize

For each pod, the runtime read surface should prioritize:

- `pod_name`
- `phase`
- `ready`
- `restarts`
- `node_name`
- `pod_ip`
- `host_ip`
- `owner_kind`
- `owner_name`
- `containers[]`
- `observed_at`

### Container status should emphasize

- `name`
- `image`
- `image_id`
- `ready`
- `restart_count`
- `state`

## Validation notes

- `GET /runtime/workload` must require both `application_id` and `environment_id`
- `GET /runtime/pods` must require both `application_id` and `environment_id`
- `DELETE /runtime/pods/{pod_name}` must require `application_id` and `environment_id` in the JSON body
- `POST /runtime/rollouts` must require `application_id` and `environment_id` in the JSON body
- invalid UUID selector values return `invalid_argument`
- missing observer-owned runtime identity for read/action lookup returns `not_found` (`ErrRuntimeIdentityMissing`)
- missing Kubernetes pod or Deployment during an explicit mutation returns `not_found`
- unresolved namespace, ambiguous workload targeting, and observer-missing pod targeting return `failed_precondition`
- Kubernetes forbidden or runtime client initialization failures return `failed_precondition`

Read-model rule:

- runtime overview and pod display should read from observer/index-backed runtime records
- direct Kubernetes calls are reserved for explicit operations such as delete pod and restart workload
- rollout callback senders may report progress from observed Kubernetes state, but `runtime-service` does not own release truth
- clients should rely on the failure class to diagnose missing observer state versus lookup ambiguity; they should not retry reads or actions by bypassing the runtime observer contract
- if a runtime action acknowledgement stays pending, localize the failure by layer: targeting failures stop before mutation, observer gaps leave rollout progress missing or stuck in running, and release-service convergence must be checked through `observe_rollout` / `finalize_release` rather than inferred from the action response alone

## Public API note

Frontend and operator docs should treat the active runtime contract as:

- `GET /api/v1/runtime/workload`
- `GET /api/v1/runtime/pods`
- `DELETE /api/v1/runtime/pods/{pod_name}`
- `POST /api/v1/runtime/rollouts`

The legacy public `runtime-specs` and `runtime-spec-revisions` route families have been retired from the runtime-service HTTP surface.
Any remaining `RuntimeSpec` or `RuntimeSpecRevision` concepts are internal implementation details, not public operator-facing routes.

## Internal model notes

The current code still persists and uses these internal records:

- `RuntimeSpec`
- `RuntimeSpecRevision`
- `RuntimeObservedWorkload`
- `RuntimeObservedPod`
- `RuntimeOperation`

The matching read model now includes a workload-level observed summary alongside observed pods, so the runtime page can show:

- controller-level overview from workload index
- pod-level detail from pod index
- actions through Kubernetes

Those model names are still useful for code navigation, but the active runtime contract should be understood as observer-rebuilt in-memory state with PostgreSQL-free runtime-domain storage rather than PostgreSQL-backed runtime CRUD.

## Current storage and observer status

Current runtime behavior should be read through the storage model in `docs/system/runtime-storage-model.md`:

- the default runtime HTTP path is memory-backed
- runtime-service can accept internal workload and pod sync callbacks
- in-process observers rebuild workload and pod index state from live cluster signals
- runtime-service active/runtime-domain storage is PostgreSQL-free
- release rollout observation is also started by the active runtime startup path and consumes runtime observer state plus Kubernetes labels
- rollout observer startup follows `internal/runtime/config/config.go`: in-cluster config plus release writeback wiring enable the callback sender
- those callbacks update release-owned steps such as `observe_rollout` and `finalize_release`
- shared platform startup outside `cmd/runtime-service` may still open PostgreSQL for other services

That internal observer detail should not dominate the external API contract if the main user value is:

- inspect application pod status
- delete a pod
- trigger a rollout restart

## Source pointers

- router: `internal/runtime/transport/http/router.go`
- handler: `internal/runtime/transport/http/handler.go`
- service: `internal/runtime/service/service.go`
- repository: `internal/runtime/repository/repository.go`
- observer: `internal/runtime/observer`
