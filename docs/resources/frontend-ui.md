# Frontend UI Contract

## Ownership

- scope: UI-facing companion contract for `docs/resources/*`
- primary consumers: frontend application and product/design reviews
- source-of-truth inputs: backend resource contracts under `docs/resources/`

## Purpose

This document defines the current frontend information architecture for the active resource model.
Use it when:

- planning frontend pages
- deciding which fields to show
- deciding which create/edit forms should exist
- removing legacy UI concepts that no longer match the backend contract

## Quick reader guide

Use this document when your question is:

- which page should own which user task
- which resource should be exposed as build-time versus deploy-time versus runtime-time UI
- which backend route the frontend should call for a given user interaction
- which backend concepts should stay hidden from product UI

If you already know the owning backend contract, go directly to:

- `docs/resources/manifest.md`
- `docs/resources/release.md`
- `docs/resources/runtime-spec.md`

## API surface

This document does not define its own backend resource.
It summarizes how the frontend should call the resource APIs owned by the five active services:

- meta-owned resources: `/api/v1/meta/...`
- config-owned resources: `/api/v1/config/...`
- network-owned resources: `/api/v1/network/...`
- release-owned resources: `/api/v1/release/...`
- runtime-owned resources: `/api/v1/runtime/...`

Service-internal examples shown below still use `/api/v1/...`.

## Core principles

### 1. Environment is a first-class dimension

Within one application, these pages are environment-sensitive:

- `AppConfig`
- `Releases`
- application-environment detail views

Current product direction also keeps environment selection visible while operating in application detail.

### 2. Manifest is build-time only

`Manifest` represents:

- source selection via `git_revision`
- resolved build identity via `commit_hash`
- image output via `image_ref` / `image_tag` / `image_digest`
- frozen `services_snapshot`
- frozen `workload_config_snapshot`
- Tekton build progress and status

`Manifest` does not own:

- `environment_id`
- release rendering
- deployment bundle artifact packaging
- ArgoCD deployment state

### 3. Release is deployment-time only

`Release` represents:

- one `manifest`
- one target `environment_id`
- one rollout `strategy`
- frozen deployment-time config such as `app_config_snapshot`
- rendered deployment bundle output
- published OCI deployment artifact
- ArgoCD deployment execution
- rollout status and step progression

### 4. WorkloadConfig only describes how the workload runs

`WorkloadConfig` no longer carries:

- `name`
- `description`
- `workload_type`
- `strategy`

Those concepts must not reappear in the frontend.

### 5. Remove legacy image-resource UI

The frontend must not ask the user to select:

- `image_id`

Manifest creation is source-driven, not image-resource-driven.

### 6. Freeze point and now-state split

The frontend should keep these concepts separate:

- `Manifest` = build-side freeze point
- `Release` = deploy-side freeze point
- runtime page = current observed state plus explicit runtime actions

Product rule:

- do not use runtime page data to explain what was frozen at deploy time
- do not use release detail to pretend it is the current live runtime state
- do not use manifest detail to explain deployment-time config

## Delivery chain mental model

Frontend should reflect this chain clearly:

1. configure metadata / workload / network
2. create `Manifest` to build one image from source
3. create `Release` to deploy one manifest into one environment
4. inspect `Release` for deploy-side frozen inputs and rollout progress
5. inspect runtime page for current observed workload and pods
6. use runtime actions only for explicit live mutations

## Top-level navigation

Recommended primary navigation:

- `Applications`
- `Environments`
- `Releases`

`Applications` is the main entry.

## Application detail page

Recommended route:

- `/applications/:applicationId`

## Header

Show:

- `name`
- `description`
- `repo_address`
- `updated_at`

Primary actions:

- `Create Manifest`
- `Create Release`
- `Edit Application`

## Environment selector

Application detail must expose an environment selector near the top of the page.

The selected environment drives:

- application-environment detail
- appconfig reads
- release list reads
- release creation defaults

## Tabs

Keep these tabs:

- `Overview`
- `Services`
- `App Config`
- `Workload Config`
- `Manifests`
- `Releases`

Do not expose legacy tabs or sections for:

- `AppRoute`
- legacy `AppService`
- image resources

`Route` is intentionally not the primary entry tab in the current application flow.
If route management is exposed, it should still be treated as a network-owned input to release, not as a release-owned resource.

## Overview tab

Show:

### Application summary

- `name`
- `description`
- `repo_address`
- `updated_at`

### Selected environment summary

- `environment.name`
- `environment.id`
- `environment.description`

### Latest manifest card

- `manifest.id`
- `git_revision`
- `commit_hash`
- `image_tag`
- `image_digest`
- `status`
- `created_at`

### Latest release card

- `release.id`
- `environment_id`
- `strategy`
- `status`
- optional `bundle_summary.artifact_ref`
- created / published timestamps when available

## Services tab

Current backend `Service` is the network service definition.

List fields:

- `name`
- `ports`
- `updated_at`

Row actions:

- `Edit`
- `Delete`

Create/edit form fields:

- `application_id`
- `name`
- `ports[].name`
- `ports[].service_port`
- `ports[].target_port`
- `ports[].protocol`

Do not introduce legacy code-repository fields here unless the backend resource changes.

## App Config tab

`AppConfig` is environment-scoped.

Recommended page shape:

- single detail panel for the selected environment
- edit through modal or drawer

Show:

- `application_id`
- `environment_id`
- `mount_path`
- `latest_revision_no`
- `latest_revision_id`
- `source_directory`
- `source_commit`
- `updated_at`
- `files[]`

For `files[]`, show:

- `name`
- `content`

Primary actions:

- `Create` when missing
- `Edit`
- `Sync from repo`

Create form fields:

- `application_id`
- `environment_id`
- `mount_path`

Update form fields:

- `application_id`
- `environment_id`
- `mount_path`

## Workload Config tab

`WorkloadConfig` is application-scoped.
There is one active record per `application_id`.

Recommended page shape:

- single detail panel
- edit through modal or drawer

Show:

- `replicas`
- `service_account_name`
- `resources`
- `probes`
- `env`
- `labels`
- `annotations`
- `updated_at`

Create/update form fields:

- `application_id`
- `replicas`
- `service_account_name`
- `resources`
- `probes`
- `env`
- `labels`
- `annotations`

Do not show or send:

- `name`
- `description`
- `workload_type`
- `strategy`

## Build vs deploy vs runtime routing

Use `Manifests` when the user asks:

- what commit did we build
- what image came out of build
- what happened in Tekton

Use `Releases` when the user asks:

- what did we deploy
- which config was frozen for deployment
- what bundle was published
- what happened during rollout

Use runtime page when the user asks:

- what is running now
- what pods are unhealthy now
- restart this workload
- delete this pod

## Manifests tab

Recommended table fields:

- `id`
- `git_revision`
- `commit_hash`
- `image_ref`
- `image_tag`
- `image_digest`
- `pipeline_id`
- `status`
- `created_at`

Row actions:

- `View details`
- `Create Release`

Manifest detail sections:

### Basic

- `id`
- `application_id`
- `git_revision`
- `repo_address`
- `commit_hash`
- `status`
- `created_at`
- `updated_at`

### Image output

- `image_ref`
- `image_tag`
- `image_digest`

### Frozen snapshots

- `services_snapshot`
- `workload_config_snapshot`

### Build execution

- `pipeline_id`
- `trace_id`
- `span_id`
- `steps`

Manifest create form fields:

- `application_id`
- `git_revision`

Rules:

- `git_revision` is optional
- default `git_revision` is `main`
- input may be branch, tag, or commit hash

Do not show or send:

- `environment_id`
- `image_id`

## Releases tab

`Release` is environment-scoped and must always be queried with:

- `application_id`
- `environment_id`

Recommended application-scoped routes:

- `/applications/:applicationId/releases`
- `/applications/:applicationId/releases/new?environment_id=...`
- `/applications/:applicationId/releases/:releaseId`

Recommended page model:

- keep the `Releases` tab as the environment-scoped list and entry point
- use a dedicated page or large drawer for `Create Release`
- use a dedicated detail page for one release because the user needs to monitor asynchronous execution after submit

The release flow page should be built around four stages:

1. choose environment and target manifest
2. inspect frozen manifest resources before submit
3. choose rollout options and create the release
4. land on release detail and track execution asynchronously

The frontend should not treat release creation as a synchronous success path.
`POST /releases` creates the record and starts asynchronous execution; the user then needs a detail page that keeps following status and steps.

### Release list page

Recommended table fields:

- `id`
- `manifest_id`
- `environment_id`
- `strategy`
- `type`
- `status`
- `external_ref`
- `created_at`

If the release detail already resolves manifest data, also show in the list when available:

- manifest `commit_hash`
- manifest `image_digest`

Recommended filter bar:

- environment selector
- status filter
- type filter
- manifest filter
- manual refresh

Rules:

- environment selector is required
- do not show a release list before `environment_id` is chosen
- `include_deleted` is not a normal end-user control and should stay out of the main UI

Primary list actions:

- `Create Release`
- `View details`
- `Open bundle preview` only when the product wants a quick peek and the detail route already exists

Recommended row status treatment:

- `Pending`, `Running`, and `Syncing` should look active and navigable
- `Succeeded` should look stable
- `Failed` and `SyncFailed` should surface an error state immediately
- `RollingBack` and `RolledBack` should be visually distinct from normal failure

Manifest-driven entry rule:

- the `Manifests` tab may expose `Create Release`
- clicking that action should carry `manifest_id` into the release create page and keep the current environment selection visible

### Release create page

Purpose:

- let the user confirm the exact deployment target and frozen build inputs before creating the release

Recommended sections, in order:

#### 1. Target environment

Show:

- selected `environment_id`
- environment display name when already resolved in the application context
- environment description when available

Rules:

- `environment_id` is required
- the field must stay visible, not hidden in implicit state
- if the user enters from the `Releases` tab, prefill from the current environment selector

#### 2. Manifest selection

Show a manifest chooser scoped to the current application:

- `id`
- `git_revision`
- `commit_hash`
- `image_tag`
- `image_digest`
- `status`
- `created_at`

Rules:

- only manifests belonging to the current `application_id` should be listed
- prefer enabling the primary CTA for manifests in `Succeeded`
- compatibility mode may still allow `Ready`, but `Pending`, `Running`, and `Failed` should not look selectable for release creation
- if the user entered from `Create Release` on a manifest row, prefill `manifest_id`

#### 3. Frozen resource preview

Before create, the frontend should show what the chosen manifest already froze.

Use:

- `GET /api/v1/manifests/{id}/resources`

On the pre-production shared ingress:

- `GET /api/v1/release/manifests/{id}/resources`

Show grouped resources when available:

- `configmap`
- `deployment` or `rollout`
- `services`
- `virtualservice`

This preview belongs on the create page because it helps the user verify the deployment target.
It is not the same as release bundle preview.

Important rule:

- do not promise release bundle preview before the release exists
- `bundle-preview` is a post-create detail-page feature because the API requires `release_id`

#### 4. Rollout options

Release create form fields:

- `manifest_id`
- `environment_id`
- `strategy`
- `type`

Recommended `strategy` options:

- `rolling`
- `blueGreen`
- `canary`

Recommended `type` options:

- default to `Upgrade`
- expose `Rollback` only when the product truly supports a rollback flow for the current user path
- keep `Install` out of the common path unless the product explicitly needs first-time install semantics

Rules:

- `strategy` is required
- `type` may default to `Upgrade`
- do not ask the user to input:
  - `app_config_snapshot`
  - `routes_snapshot`
  - `artifact_repository`
  - `artifact_tag`
  - `artifact_digest`
  - `artifact_ref`
  - `steps`
  - `status`

Those are backend-frozen or backend-managed fields.

#### 5. Submit and transition

On submit:

- call `POST /api/v1/releases`
- on the shared ingress, call `POST /api/v1/release/releases`
- after `201 Created`, navigate immediately to the release detail route

Known precondition failures should be presented as user-readable blocking messages:

- manifest not ready
- effective app config missing
- runtime service unavailable
- deploy target cluster not ready

These are workflow blockers, not generic form-validation errors.

### Release detail page

Purpose:

- give the user one place to track release execution from creation to terminal state

### Basic

- `id`
- `execution_intent_id`
- `application_id`
- `manifest_id`
- `environment_id`
- `strategy`
- `type`
- `status`
- `external_ref`
- `created_at`
- `updated_at`

Recommended header actions:

- `Refresh`
- `Open manifest`
- `Open bundle preview`
- `Copy external ref` when present

#### Status header

The detail page header should highlight:

- top-level `status`
- selected `strategy`
- selected `type`
- manifest identity
- environment identity
- current active step summary when one step is `Running`

If the release is still active, show an execution banner such as:

- `Creating deployment bundle`
- `Observing rollout`
- `Switching traffic`

The banner text should come from the currently running step when available.

### Deployment artifact

- `artifact_repository`
- `artifact_tag`
- `artifact_digest`
- `artifact_ref`

### Bundle summary

The normal release detail read should also expose a lightweight `bundle_summary` when render has already fixed a canonical bundle fact.

Recommended shape:

```json
{
  "bundle_summary": {
    "available": true,
    "namespace": "checkout",
    "artifact_name": "demo-api",
    "bundle_digest": "sha256:bundle",
    "primary_workload_kind": "Rollout",
    "resource_counts": {
      "configmaps": 1,
      "services": 2,
      "rollouts": 1,
      "virtualservices": 1,
      "total": 5
    },
    "artifact": {
      "repository": "zot.zot.svc.cluster.local:5000/devflow/releases/demo-api",
      "tag": "95cccbf1-2e15-4a08-ad39-94019f59edea",
      "digest": "sha256:artifact",
      "ref": "oci://zot.zot.svc.cluster.local:5000/devflow/releases/demo-api@sha256:artifact"
    },
    "rendered_at": "2026-04-29T10:00:00Z",
    "published_at": "2026-04-29T10:00:12Z"
  }
}
```

This section is for quick detail-page rendering and should remain much lighter than full `bundle-preview`.

### Runtime tracking

- `trace_id`
- `span_id`
- `steps`

Runtime tracking presentation rules:

- render `steps` as the primary execution timeline
- use `steps[].name` as the main label
- keep `steps[].code` visible in an advanced details affordance for debugging
- show `progress`, `status`, `message`, `start_time`, and `end_time` inline

Polling rules:

- while `status` is `Pending`, `Running`, or `Syncing`, poll release detail on a short interval such as `5s`
- stop polling automatically on terminal states such as `Succeeded`, `Failed`, `RolledBack`, or `SyncFailed`
- manual refresh should remain available even after polling stops

Strategy-sensitive timeline note:

- rolling, blue-green, and canary releases have different step shapes
- the frontend should render whatever `steps` the backend returns instead of hardcoding a fixed step count
- if the page needs per-strategy visual hints, derive them from `strategy`, not from guessed step order alone

#### Execution intent panel

When `execution_intent_id` is present, the detail page should expose an execution intent panel.

Use:

- `GET /api/v1/intents/{id}`

On the shared ingress:

- `GET /api/v1/release/intents/{id}`

Show:

- `id`
- `kind`
- `status`
- `message`
- `last_error`
- `claimed_by`
- `claimed_at`
- `lease_expires_at`
- `attempt_count`

This panel is especially useful when release execution is still `Pending` or repeatedly failing.

#### Bundle preview panel

The detail page should fetch bundle preview lazily when the user expands the panel.

Use:

- `GET /api/v1/releases/{id}/bundle-preview`

On the shared ingress:

- `GET /api/v1/release/releases/{id}/bundle-preview`

Show:

- `namespace`
- `artifact_name`
- `bundle_digest`
- `rendered_at`
- optional `published_at`
- `frozen_inputs`
- `rendered_bundle`

Do not fetch bundle preview on every background poll.
It is heavier than the normal detail read and is better treated as on-demand inspection.

Bundle preview should be rendered in two layers:

- `Frozen inputs`
- `Rendered bundle`

`Frozen inputs` should let the user inspect the exact release-time source material:

- manifest summary
- services snapshot
- workload snapshot
- app config snapshot
- routes snapshot

## Runtime page

The runtime page should follow one clear split:

- runtime overview reads from runtime-owned observed workload index data
- pod list reads from runtime-owned observed pod index data
- action buttons call runtime action APIs that then call Kubernetes

Do not make routine page rendering depend on live Kubernetes reads.

### Runtime information architecture

Recommended layout:

1. `Workload Summary`
2. `Conditions`
3. `Pods`
4. `Actions`

This keeps three different concepts separate:

- current runtime state
- current pod instances
- explicit operator actions

### Workload Summary

Use the runtime overview endpoint:

- `GET /api/v1/runtime/workload?application_id=...&environment_id=...`

Show:

- `workload_kind`
- `workload_name`
- `namespace`
- `summary_status`
- `desired_replicas`
- `ready_replicas`
- `updated_replicas`
- `available_replicas`
- `unavailable_replicas`
- `images[]`
- `observed_at`
- optional `restart_at`

Presentation rules:

- treat this card as the controller-level summary for one `application + environment`
- show `summary_status` as the primary badge
- show replica counts in one compact line such as `ready / desired`
- collapse long image lists by default and expand on demand
- show observed freshness from `observed_at`

This panel is not the place to show full desired config or full rendered YAML.

### Conditions

Show workload conditions from the same overview response:

- `conditions[].type`
- `conditions[].status`
- `conditions[].reason`
- `conditions[].message`
- `conditions[].last_transition_time`

Presentation rules:

- show degraded or progressing conditions first
- allow long condition messages to wrap
- keep this section close to the workload summary

### Pods

Use:

- `GET /api/v1/runtime/pods?application_id=...&environment_id=...`

Show:

- `pod_name`
- `phase`
- `ready`
- `owner_kind`
- `owner_name`
- `containers[]`
- `observed_at`

Presentation rules:

- pods are the instance-level detail view under the workload summary
- do not duplicate workload-level replica summary in every row
- container restarts and state belong in expanded row details when possible

### Actions

Actions are the only place that should actively call Kubernetes through runtime-service.

Use:

- `POST /api/v1/runtime/rollouts`
- `DELETE /api/v1/runtime/pods/{pod_name}`

Presentation rules:

- keep restart action at workload level
- keep delete action at individual pod row level
- require clear loading state and success/failure feedback
- after an action succeeds, refresh workload + pods from runtime index endpoints
- restart payload may omit `deployment_name` and let runtime-service resolve the primary Deployment automatically

Recommended restart request:

```json
{
  "application_id": "999c0c88-1f1f-41d1-a67a-8159d07c878c",
  "environment_id": "b780ca97-a213-4763-bfb9-43f7e3a11ee7",
  "operator": "songbei"
}
```

Use explicit `deployment_name` only when the UI intentionally exposes multiple Deployment targets.

Recommended delete-pod request:

```json
{
  "application_id": "999c0c88-1f1f-41d1-a67a-8159d07c878c",
  "environment_id": "b780ca97-a213-4763-bfb9-43f7e3a11ee7",
  "operator": "songbei"
}
```

Runtime refresh rule after action:

1. call restart or delete
2. if response is `204`, keep the action control in loading / pending state briefly
3. refetch `GET /api/v1/runtime/workload`
4. refetch `GET /api/v1/runtime/pods`
5. update the page from those index-backed reads only

### Runtime page boundaries

Keep these concepts separate in the UI:

- `Runtime` = what is currently running
- `Rendered YAML` = what one release produced
- `Desired Config` = what app config / workload config / service / route declare

So:

- runtime page should show current observed workload + pods
- release detail page should show rendered bundle and frozen inputs
- config pages should show desired config editing surfaces

`Rendered bundle` should let the user inspect the final output:

- resource groups for quick navigation
- rendered resource summaries
- per-resource YAML
- combined `bundle.yaml`

When bundle preview is not ready yet:

- treat `409 failed_precondition` with message `bundle not ready` as a normal loading-state signal
- explain that the release exists but render has not produced a readable bundle yet

Recommended not-ready error shape:

```json
{
  "error": {
    "code": "failed_precondition",
    "message": "bundle not ready"
  }
}
```

Recommended full bundle preview response shape:

```json
{
  "data": {
    "release_id": "95cccbf1-2e15-4a08-ad39-94019f59edea",
    "manifest_id": "7c1a-...",
    "application_id": "9f2c-...",
    "environment_id": "pre-production",
    "strategy": "canary",
    "namespace": "checkout",
    "artifact_name": "demo-api",
    "bundle_digest": "sha256:bundle",
    "artifact": {
      "repository": "zot.zot.svc.cluster.local:5000/devflow/releases/demo-api",
      "tag": "95cccbf1-2e15-4a08-ad39-94019f59edea",
      "digest": "sha256:artifact",
      "ref": "oci://zot.zot.svc.cluster.local:5000/devflow/releases/demo-api@sha256:artifact"
    },
    "frozen_inputs": {
      "manifest_summary": {
        "manifest_id": "7c1a-...",
        "commit_hash": "abc123",
        "image_ref": "registry.example.com/demo-api:abc123",
        "image_digest": "sha256:image"
      },
      "services": [
        {
          "name": "demo-api",
          "ports": [
            {
              "name": "http",
              "service_port": 80,
              "target_port": 8080,
              "protocol": "TCP"
            }
          ]
        }
      ],
      "workload": {
        "replicas": 2,
        "service_account_name": "demo-api",
        "env": [
          { "name": "LOG_LEVEL", "value": "info" }
        ]
      },
      "app_config": {
        "mount_path": "/etc/config",
        "data": {
          "LOG_LEVEL": "info"
        },
        "files": [
          {
            "name": "app.yaml",
            "content": "log_level: info"
          }
        ]
      },
      "routes": [
        {
          "name": "api",
          "host": "demo.example.com",
          "path": "/",
          "service_name": "demo-api",
          "service_port": 80
        }
      ]
    },
    "rendered_bundle": {
      "resource_groups": [
        {
          "kind": "Service",
          "items": [
            { "name": "demo-api", "namespace": "checkout" },
            { "name": "demo-api-canary", "namespace": "checkout" }
          ]
        },
        {
          "kind": "Rollout",
          "items": [
            { "name": "demo-api", "namespace": "checkout" }
          ]
        }
      ],
      "rendered_resources": [
        {
          "kind": "Service",
          "name": "demo-api",
          "namespace": "checkout",
          "summary": {
            "ports": [
              { "name": "http", "port": 80, "target_port": 8080, "protocol": "TCP" }
            ]
          },
          "yaml": "apiVersion: v1\nkind: Service\n..."
        }
      ],
      "files": [
        {
          "path": "01-service-demo-api.yaml",
          "content": "apiVersion: v1\nkind: Service\n..."
        },
        {
          "path": "bundle.yaml",
          "content": "---\n..."
        }
      ]
    },
    "rendered_at": "2026-04-29T10:00:00Z",
    "published_at": "2026-04-29T10:00:12Z"
  }
}
```

### Frozen deployment inputs

- `routes_snapshot`
- `app_config_snapshot`
- manifest-side `services`
- manifest-side `workload`

Presentation rules:

- `app_config_snapshot` should be visible by default because it is an active release input
- `routes_snapshot` should stay collapsed or hidden when empty because route flow is currently deferred in the mainline frontend
- when manifest metadata is available, also show a compact manifest summary alongside frozen release inputs:
  - `manifest_id`
  - `commit_hash`
  - `image_ref` or `image_digest`
- `services` and `workload` should be shown as original frozen inputs, not as rendered Kubernetes objects

### Release page flow summary

Recommended frontend sequence:

1. load environment-scoped release list
2. open create page with current `environment_id`
3. list manifests for the application
4. read manifest frozen resources after `manifest_id` is chosen
5. submit `POST /releases`
6. navigate to release detail
7. poll release detail until terminal
8. fetch execution intent and bundle preview on demand

## Environment pages

Recommended list route:

- `/environments`

List fields:

- `name`
- `cluster_id`
- `description`
- `updated_at`

Create/edit form fields:

- `name`
- `cluster_id`
- `description`
- `labels`

## Application-environment binding

The frontend should allow attaching an environment to an application.

Create attach payload:

```json
{
  "environment_id": "b780ca97-a213-4763-bfb9-43f7e3a11ee7"
}
```

Use service-internal routes:

- `GET /api/v1/applications/{id}/environments`
- `POST /api/v1/applications/{id}/environments`
- `GET /api/v1/applications/{id}/environments/{environment_id}`
- `DELETE /api/v1/applications/{id}/environments/{environment_id}`

On the pre-production shared ingress, use:

- `GET /api/v1/meta/applications/{id}/environments`
- `POST /api/v1/meta/applications/{id}/environments`
- `GET /api/v1/meta/applications/{id}/environments/{environment_id}`
- `DELETE /api/v1/meta/applications/{id}/environments/{environment_id}`

Application-environment detail is the right place to aggregate:

- environment metadata
- environment-scoped `appconfig`
- application-scoped `workloadconfig`
- environment-scoped `releases`

## Forms: modal vs page

Recommended modal/drawer forms:

- create/edit `Environment`
- create/edit `Service`
- create/edit `AppConfig`
- create/edit `WorkloadConfig`
- create `Manifest`

Recommended dedicated page or large drawer:

- release detail
- manifest detail
- create `Release`

## Legacy UI that must be removed

Remove these concepts from the frontend:

- `AppRoute` management in the main application flow
- legacy `AppService`
- `image_id`
- standalone image-resource picker
- `workload_type`
- workload `strategy`
- manifest-side rendered deployment output fields

## API call checklist

### Manifest list/create

- pass `application_id`
- optional `git_revision` on create

### AppConfig read/create/update

- always use `application_id + environment_id`

### Release list

- always pass:
  - `application_id`
  - `environment_id`
- optional filters:
  - `status`
  - `type`
  - `manifest_id`

### Release create

- list manifests with:
  - `application_id`
- fetch manifest frozen resources with:
  - `manifest_id`
- always pass:
  - `manifest_id`
  - `environment_id`
  - `strategy`
- optional:
  - `type`

### Release detail

- fetch:
  - `release_id`
- poll by:
  - `release_id` while top-level status is active
- fetch bundle preview lazily by:
  - `release_id`
- fetch intent detail lazily by:
  - `execution_intent_id`

### Runtime page

- always pass:
  - `application_id`
  - `environment_id`
- read workload overview from:
  - `GET /api/v1/runtime/workload`
- read pod list from:
  - `GET /api/v1/runtime/pods`
- trigger restart with:
  - `POST /api/v1/runtime/rollouts`
- delete one pod with:
  - `DELETE /api/v1/runtime/pods/{pod_name}`
- optional for restart requests:
  - `deployment_name` when the UI wants to target a specific Deployment explicitly

Minimal frontend runtime sequence:

1. load `runtime/workload`
2. load `runtime/pods`
3. render summary + conditions + pods
4. on restart/delete success, refetch both runtime read endpoints

## Source pointers

- `docs/resources/application.md`
- `docs/resources/application-environment.md`
- `docs/resources/environment.md`
- `docs/resources/service.md`
- `docs/resources/appconfig.md`
- `docs/resources/workloadconfig.md`
- `docs/resources/manifest.md`
- `docs/resources/release.md`
- `docs/resources/intent.md`
