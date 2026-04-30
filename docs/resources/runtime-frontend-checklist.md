# Runtime Frontend Checklist

## Purpose

This is the short frontend integration checklist for the runtime page.

Use this file when you only need:

- which routes to call
- what order to call them in
- how actions should refresh reads

## Runtime page load

Use `docs/system/flow-overview.md` as the stage-routing starting point when rollout ownership is unclear.
For the runtime page itself, treat these reads as the runtime-owned surface consuming release-owned labels from the observed cluster state.

Always provide:

- `application_id`
- `environment_id`

Load in this order:

1. `GET /api/v1/runtime/workload`
2. `GET /api/v1/runtime/pods`

Recommended query shape:

```http
GET /api/v1/runtime/workload?application_id=...&environment_id=...
GET /api/v1/runtime/pods?application_id=...&environment_id=...
```

## What each route is for

### `GET /api/v1/runtime/workload`

Use for:

- workload summary card
- conditions
- replica counts
- current image summary
- restart timestamp
- release-aware runtime identity recovered from stable workload labels:
  - `app.kubernetes.io/name`
  - `devflow.io/release-id`
  - `devflow.application/id`
  - `devflow.environment/id`

### `GET /api/v1/runtime/pods`

Use for:

- pod table
- per-pod readiness
- per-container state
- delete-pod row action

## Restart action

Use:

```http
POST /api/v1/runtime/rollouts
```

Recommended body:

```json
{
  "application_id": "999c0c88-1f1f-41d1-a67a-8159d07c878c",
  "environment_id": "b780ca97-a213-4763-bfb9-43f7e3a11ee7",
  "operator": "songbei"
}
```

Notes:

- `deployment_name` is optional
- default UI path should omit it and let runtime-service resolve the primary Deployment

## Delete pod action

Use:

```http
DELETE /api/v1/runtime/pods/{pod_name}
```

Recommended body:

```json
{
  "application_id": "999c0c88-1f1f-41d1-a67a-8159d07c878c",
  "environment_id": "b780ca97-a213-4763-bfb9-43f7e3a11ee7",
  "operator": "songbei"
}
```

## Refresh rule after action

After restart or delete succeeds:

1. keep the action control in temporary loading state
2. refetch `GET /api/v1/runtime/workload`
3. refetch `GET /api/v1/runtime/pods`
4. update UI only from those read responses

Do not do an ad-hoc direct Kubernetes read from the frontend.
Do not require annotations to recover release/application/environment identity; annotations are supplementary only.

## Page contract summary

- read path = runtime owned observer/index state
- write path = runtime-service calling Kubernetes
- release truth stays owned by `release-service`
- runtime-service may send rollout callbacks after observing cluster state, but does not own release truth
- annotations are supplementary only; stable identity comes from labels

## Source pointers

- full runtime API contract: `docs/resources/runtime-spec.md`
- full UI contract: `docs/resources/frontend-ui.md`
- runtime observer model: `docs/system/runtime-observer.md`
