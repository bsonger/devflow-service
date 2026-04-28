# Frontend UI Contract

## Purpose

This document defines the current frontend information architecture for the active resource model.
It is the UI-facing companion to the individual resource docs under `docs/resources/`.

Use this doc when:

- planning frontend pages
- deciding which fields to show
- deciding which create/edit forms should exist
- removing legacy UI concepts that no longer match the backend contract

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
- rendered deployment bundle output
- ArgoCD deployment execution
- runtime tracking and final rollout status

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

`Route` is intentionally out of the current frontend scope.

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
- `created_at`

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

Release detail sections:

### Basic

- `id`
- `application_id`
- `manifest_id`
- `environment_id`
- `strategy`
- `type`
- `status`
- `external_ref`
- `created_at`
- `updated_at`

### Deployment artifact

- `artifact_repository`
- `artifact_tag`
- `artifact_digest`
- `artifact_ref`

### Runtime tracking

- `trace_id`
- `span_id`
- `steps`

### Resolved deployment inputs

- `routes_snapshot`
- `app_config_snapshot`

Release create form fields:

- `manifest_id`
- `environment_id`
- `strategy`
- `type`

Rules:

- `environment_id` is required and must be visible in the form
- frontend must pass `environment_id` on release list reads

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

Use:

- `GET /api/v1/applications/{id}/environments`
- `POST /api/v1/applications/{id}/environments`
- `GET /api/v1/applications/{id}/environments/{environment_id}`
- `DELETE /api/v1/applications/{id}/environments/{environment_id}`

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

### Release create

- always pass:
  - `manifest_id`
  - `environment_id`
  - `strategy`

## Source docs

- `docs/resources/application.md`
- `docs/resources/application-environment.md`
- `docs/resources/environment.md`
- `docs/resources/service.md`
- `docs/resources/appconfig.md`
- `docs/resources/workloadconfig.md`
- `docs/resources/manifest.md`
- `docs/resources/release.md`
