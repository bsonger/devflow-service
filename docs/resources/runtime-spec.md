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
Its core jobs are:

- show the current workload overview for one `application + environment`
- list the live pod status for one `application + environment`
- delete a specific pod for that application runtime
- trigger a rollout-style restart for that application workload

From the external API point of view, this service should be understood as a runtime operations surface first.
The internal storage model may still use `RuntimeSpec`, `RuntimeSpecRevision`, `RuntimeObservedPod`, and `RuntimeOperation`, but those are supporting implementation details rather than the primary API story.
For operator understanding, the preferred model is:

- display reads from runtime-owned observed index data
- explicit actions call Kubernetes

The current observed index now includes:

- `RuntimeObservedWorkload`
- `RuntimeObservedPod`
- `RuntimeOperation`

Current code still contains PostgreSQL-backed runtime persistence, but that should be treated as an implementation detail or migration residue rather than the main API contract.

## Main operator flows

### 1. Show application workload overview

Primary read flow:

1. caller provides `application_id` and `environment_id`
2. runtime-service resolves the target runtime binding
3. runtime-service reads the latest observed workload summary from runtime-owned index data
4. runtime-service returns one workload overview

This is the controller-level runtime read surface.

### 2. List application pod status

Primary read flow:

1. caller provides `application_id` and `environment_id`
2. runtime-service resolves the target runtime binding
3. runtime-service reads the latest observed pod list from runtime-owned index data
4. runtime-service returns the current pod list and pod status snapshot

This is the instance-level runtime read surface.

### 3. Delete one pod

Primary action flow:

1. caller chooses one pod under one `application + environment`
2. runtime-service resolves the target pod in Kubernetes
3. runtime-service deletes the pod
4. Kubernetes recreates or rebalances it according to the owning controller

This action is for one concrete pod, not for the whole application rollout.

### 4. Trigger rollout / restart

Primary action flow:

1. caller provides `application_id` and `environment_id`
2. runtime-service resolves the target workload Deployment
3. runtime-service patches the Deployment with `kubectl.kubernetes.io/restartedAt`
4. Kubernetes performs the rolling restart

Current implementation note:

- the runtime action is implemented as a Deployment restart
- in product language, this can be described as triggering a rollout or restart for the application workload

## API surface

Current service-internal route surface:

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
  "deployment_name": "meta-service",
  "operator": "songbei"
}
```

Required fields:

- `application_id`
- `environment_id`
- `deployment_name` in the current implementation

Optional fields:

- `operator`

Future simplification note:

- if runtime-service can resolve the single primary workload automatically, `deployment_name` can become server-resolved later
- until then, frontend and callers should send it explicitly

## Internal observer sync surface

These routes are service-internal observer callbacks and are not part of the shared external ingress contract:

- `POST /api/v1/internal/runtime-spec-workloads/sync`
- `POST /api/v1/internal/runtime-spec-workloads/delete`
- `POST /api/v1/internal/runtime-spec-pods/sync`
- `POST /api/v1/internal/runtime-spec-pods/delete`

### Sync workload summary

```http
POST /api/v1/internal/runtime-spec-workloads/sync
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
- sync is idempotent at the runtime-spec level and updates the latest observed workload summary

### Delete workload summary

```http
POST /api/v1/internal/runtime-spec-workloads/delete
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
- missing target application runtime returns `not_found`
- missing Kubernetes pod or Deployment returns `not_found`
- Kubernetes forbidden or runtime client initialization failures return `failed_precondition`

Read-model rule:

- runtime overview and pod display should read from observer/index-backed runtime records
- direct Kubernetes calls are reserved for explicit operations such as delete pod and restart workload

## Current implementation gap

The current codebase still contains an older `runtime-spec`-shaped surface, including routes such as:

- `/api/v1/runtime-specs`
- `/api/v1/runtime-specs/{id}/pods`
- `/api/v1/runtime-specs/{id}/deployments/{deployment_name}/restart`

That shape reflects the current implementation model, but it is not the preferred long-term external runtime API.
The preferred external contract is the simpler runtime operations surface documented above.

The `runtime/workload` overview endpoint now belongs to the active preferred external contract.

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

## Current pre-production status

As of April 29, 2026:

- `GET /api/v1/runtime/workload` is deployed on pre-production and returns workload overview data
- the runtime-service database schema includes `runtime_observed_workloads`
- runtime-service code can accept internal workload sync callbacks

Remaining operational gap:

- the current `resource-observer` deployment has not yet been updated in this repo to automatically post workload summary sync events
- workload overview can be read today, but automatic population still depends on observer-side follow-up work

Those records may continue to exist for implementation, history, or observer-sync purposes.
But they should not dominate the external API contract if the main user value is:

- inspect application pod status
- delete a pod
- trigger a rollout restart

## Source pointers

- router: `internal/runtime/transport/http/router.go`
- handler: `internal/runtime/transport/http/handler.go`
- service: `internal/runtime/service/service.go`
- repository: `internal/runtime/repository/repository.go`
- observer: `internal/runtime/observer`
