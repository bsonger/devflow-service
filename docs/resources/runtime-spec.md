# Runtime Runtime API

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

- list the live pod status for one `application + environment`
- delete a specific pod for that application runtime
- trigger a rollout-style restart for that application workload

From the external API point of view, this service should be understood as a runtime operations surface first.
The internal storage model may still use `RuntimeSpec`, `RuntimeSpecRevision`, `RuntimeObservedPod`, and `RuntimeOperation`, but those are supporting implementation details rather than the primary API story.

## Main operator flows

### 1. List application pod status

Primary read flow:

1. caller provides `application_id` and `environment_id`
2. runtime-service resolves the target workload in Kubernetes
3. runtime-service returns the current pod list and pod status snapshot

This is the main runtime read surface.

### 2. Delete one pod

Primary action flow:

1. caller chooses one pod under one `application + environment`
2. runtime-service resolves the target pod in Kubernetes
3. runtime-service deletes the pod
4. Kubernetes recreates or rebalances it according to the owning controller

This action is for one concrete pod, not for the whole application rollout.

### 3. Trigger rollout / restart

Primary action flow:

1. caller provides `application_id` and `environment_id`
2. runtime-service resolves the target workload Deployment
3. runtime-service patches the Deployment with `kubectl.kubernetes.io/restartedAt`
4. Kubernetes performs the rolling restart

Current implementation note:

- the runtime action is implemented as a Deployment restart
- in product language, this can be described as triggering a rollout or restart for the application workload

## Target API surface

Service-internal route surface:

- `GET /api/v1/runtime/pods?application_id=...&environment_id=...`
- `DELETE /api/v1/runtime/pods/{pod_name}`
- `POST /api/v1/runtime/rollouts`

Pre-production shared ingress external surface:

- `GET /api/v1/runtime/pods?application_id=...&environment_id=...`
- `DELETE /api/v1/runtime/pods/{pod_name}`
- `POST /api/v1/runtime/rollouts`

Selector placement follows the repo-wide policy:

- `GET` uses query filters
- `DELETE` uses request body when the path alone is not enough
- `POST` uses request body

## Request contracts

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

- `operator`
- `deployment_name` when the implementation chooses to expose it explicitly

## Response focus

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

- `GET /runtime/pods` must require both `application_id` and `environment_id`
- `DELETE /runtime/pods/{pod_name}` must require `application_id` and `environment_id` in the JSON body
- `POST /runtime/rollouts` must require `application_id` and `environment_id` in the JSON body
- invalid UUID selector values return `invalid_argument`
- missing target application runtime returns `not_found`
- missing Kubernetes pod or Deployment returns `not_found`
- Kubernetes forbidden or runtime client initialization failures return `failed_precondition`

## Current implementation gap

The current codebase still contains an older `runtime-spec`-shaped surface, including routes such as:

- `/api/v1/runtime-specs`
- `/api/v1/runtime-specs/{id}/pods`
- `/api/v1/runtime-specs/{id}/deployments/{deployment_name}/restart`

That shape reflects the current implementation model, but it is not the preferred long-term external runtime API.
The preferred external contract is the simpler runtime operations surface documented above.

## Internal model notes

The current code still persists and uses these internal records:

- `RuntimeSpec`
- `RuntimeSpecRevision`
- `RuntimeObservedPod`
- `RuntimeOperation`

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
