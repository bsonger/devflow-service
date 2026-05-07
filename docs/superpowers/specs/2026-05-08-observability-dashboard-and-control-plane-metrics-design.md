# Observability Dashboard and Control-Plane Metrics Design

## Purpose

This design defines the target Grafana information architecture and the next
control-plane metrics expansion for `devflow-service`.

It addresses two linked problems:

1. the current Grafana dashboard layout is not strict enough and mixes
   environments, dashboard categories, and maturity levels in ways that create
   operator confusion
2. the current metrics surface is strong at HTTP/service health but still too
   weak at control-plane diagnosis, especially around manifest build,
   configuration sync, Argo rollout, observer polling, and writeback callbacks

The design goal is a stable, low-confusion observability structure that supports
fast diagnosis of both service health problems and DevFlow delivery-pipeline
failures.

## Scope

This design covers:

- Grafana folder structure
- Grafana dashboard naming rules
- Grafana dashboard category boundaries
- which existing dashboards should be deleted
- which new dashboards should exist immediately
- which dashboards depend on future metric implementation
- which control-plane metric families should be added next
- which new failure-series should carry exemplars

This design does not yet implement the metrics or dashboards.

## Non-goals

This design does not include:

- alert policy design
- front-end page-level web analytics design
- LogQL or trace retention redesign beyond already-agreed exemplar usage
- changes to the current low-value HTTP path filtering policy

## Constraints agreed during brainstorming

The user-approved constraints are:

- old DevFlow dashboards should be deleted rather than preserved during a
  migration window
- production and pre-production must each have their own Grafana folder
- production and pre-production should remain structurally identical
- dashboard categories should be explicit and stable
- `Delivery / ...` should be a first-class category rather than folded into
  `Service / ...`
- `Page/API / ...` should initially mean backend API route observability only,
  not frontend page analytics

## Proposed approaches

### Approach A — Environment-first folders with category-stable dashboards

Create one folder per environment and organize dashboards inside each folder by
stable category prefix.

Example:

- `devflow-production`
- `devflow-pre-production`

Inside each folder:

- `Infra / ...`
- `Service / ...`
- `Delivery / ...`
- `Page/API / ...`

Pros:

- strongest environment isolation
- least operator confusion
- safest for production diagnosis
- stable naming for docs, on-call playbooks, and future alerts

Cons:

- more dashboards overall
- requires immediate cleanup of the current mixed structure

Recommendation: **accept**.

### Approach B — Category-first folders with environment variables

Group dashboards by category globally, then switch environment with variables.

Pros:

- fewer folders
- simpler reuse story

Cons:

- too easy to inspect the wrong environment
- weak production/pre-production separation
- does not satisfy the “remove confusion” goal

Recommendation: reject.

### Approach C — Environment folders with only a few large dashboards

Use environment folders but collapse all observability into a few broad
catch-all dashboards.

Pros:

- lower dashboard count
- simple top-level navigation

Cons:

- poor drilldown clarity
- dashboard responsibilities drift over time
- future control-plane observability still ends up mixed together

Recommendation: reject.

## Selected design

Use **Approach A**.

## Grafana information architecture

### Folder structure

Create exactly two DevFlow folders:

- `devflow-production`
- `devflow-pre-production`

These folders are the only supported top-level entry points for DevFlow
application observability.

The existing generic infrastructure dashboards such as `kubernetes` and
`node-exporter` may remain in Grafana, but they are not part of the DevFlow
folder contract and should not be treated as the primary DevFlow operator view.

### Naming rules

Within each folder, dashboard names must start with a stable category prefix.

Allowed prefixes:

- `Infra /`
- `Service /`
- `Delivery /`
- `Page/API /`

Do not create dashboards without one of those prefixes.

Do not use ambiguous names such as:

- `Overview`
- `Kubernetes Workloads`
- `Node Health`
- `Release Control`

without the category prefix.

### Environment symmetry

The production and pre-production folders must stay structurally identical:

- same dashboard names
- same variable structure
- same panel intent
- same category layout

Only the environment-specific query scope should differ.

## Dashboard set

Each environment folder should contain the same dashboard set.

### Infra

#### `Infra / Cluster`

Purpose:

- answer whether services are alive at the Kubernetes workload layer
- expose pod, deployment, restart, and scrape health

Expected panels:

- running pods
- pod phase count
- deployment desired vs available
- unavailable replicas
- pod restarts
- scrape targets up
- scrape health by service
- pod CPU by pod
- pod memory by pod

#### `Infra / Node`

Purpose:

- answer whether node-level resource pressure is contributing to service issues

Expected panels:

- node CPU used %
- node memory used %
- root disk used %
- node load
- DevFlow pod count by node
- DevFlow memory by node
- DevFlow pod CPU by pod
- DevFlow pod memory by pod

### Service

#### `Service / Overview`

Purpose:

- identify which service is unhealthy at the HTTP/API layer

Expected panels:

- request rate by service
- 4xx rate by service
- 5xx rate by service
- p95 latency by service
- p99 latency by service
- inflight by service
- top routes by request rate
- slowest routes p95
- 5xx exemplar jump-off panel

#### `Service / Detail`

Purpose:

- isolate a problem to a route inside a selected service

Expected panels:

- request rate by route
- status class rate
- p50 / p95 / p99 by route
- 4xx by route
- 5xx by route
- request body size p95
- response body size p95
- inflight
- request totals by status code
- route-specific 5xx exemplar panel

Variables:

- `service`
- `route`

#### `Service / Dependency`

Purpose:

- show whether downstream dependency behavior explains service-level failures

Expected panels:

- dependency call rate
- dependency error rate
- dependency error ratio
- dependency latency p95
- errors by dependency, action, and result

### Delivery

#### `Delivery / Release`

Purpose:

- expose overall release throughput, failure rate, rollback rate, and latency

Expected panels:

- releases created / 1h
- releases succeeded / 1h
- releases failed / 1h
- rollbacks / 1h
- release throughput by type
- release duration p50 / p95 / p99
- release failure ratio
- rollback by type

#### `Delivery / Manifest Build`

Purpose:

- expose manifest build health, task failures, and build latency

Expected panels once metrics exist:

- manifests created / 1h
- manifests succeeded / failed / 1h
- manifest build duration p50 / p95
- task failures by `task_name`
- task duration by `task_name`
- build failure exemplar panel

#### `Delivery / Config Sync`

Purpose:

- expose app-config repository sync volume, failure rate, and latency

Expected panels once metrics exist:

- sync total / 1h
- sync success / failed / 1h
- sync failure ratio
- sync duration p50 / p95
- sync failure exemplar panel

#### `Delivery / Argo Rollout`

Purpose:

- separate Argo application creation failures from rollout failures

Expected panels once metrics exist:

- Argo application create success / failed
- Argo application create duration
- rollout success / failed
- rollout duration p50 / p95
- rollout failure exemplar panel

#### `Delivery / Observer & Writeback`

Purpose:

- prove that background observers and callback/writeback paths are still
  operating correctly

Expected panels once metrics exist:

- observer sync total / failed
- observer sync duration
- writeback total / failed
- writeback duration
- callback failures by `callback_type`
- observer/writeback failure exemplar panel

### Page/API

#### `Page/API / Route Detail`

Purpose:

- provide API route-specific observability independent of service grouping

Expected panels:

- route req/s
- route 4xx / 5xx
- route p95 / p99
- route request / response size
- route split by service
- route split by method
- route-specific 5xx exemplar panel

This first version is backend API only.

It explicitly does not cover frontend page analytics or Web Vitals yet.

## Dashboard cleanup policy

Delete the current DevFlow-specific dashboards that do not conform to the new
folder and naming rules.

That includes the current production/pre-production dashboards with names such
as:

- `DevFlow Production Overview`
- `DevFlow Production Service Detail`
- `DevFlow Production Release Control`
- `DevFlow Production Dependency Detail`
- `DevFlow Production Kubernetes Workloads`
- `DevFlow Production Node Health`
- `DevFlow Pre-Production Overview`
- `DevFlow Pre-Production Service Detail`
- `DevFlow Pre-Production Release Control`
- `DevFlow Pre-Production Dependency Detail`
- `DevFlow Pre-Production Kubernetes Workloads`
- `DevFlow Pre-Production Node Health`

These should not coexist with the new folder structure because the user’s goal
is to reduce visual confusion, not to provide a transition period.

The generic Grafana dashboards `kubernetes` and `node-exporter` are not part of
this deletion set, but they are also not part of the DevFlow supported
information architecture.

## Current vs future dashboard readiness

### Can be built immediately from current metrics

These dashboards can be supported now because the repository already exposes the
required HTTP, release-summary, dependency, and infrastructure metrics:

- `Infra / Cluster`
- `Infra / Node`
- `Service / Overview`
- `Service / Detail`
- `Service / Dependency`
- `Delivery / Release`
- `Page/API / Route Detail`

### Require new metrics first

These dashboards should not be treated as complete until the corresponding
control-plane metrics exist:

- `Delivery / Manifest Build`
- `Delivery / Config Sync`
- `Delivery / Argo Rollout`
- `Delivery / Observer & Writeback`

## Control-plane metrics expansion

The current metrics surface is not enough for delivery-path diagnosis.

The next metrics implementation pass should add the following families.

### Metric label rules

All new control-plane metrics must continue to use only low-cardinality labels.

Required base labels:

- `service_name`
- `service_namespace`
- `deployment_environment_name`

Allowed bounded labels where relevant:

- `release_type`
- `strategy`
- `pipeline_type`
- `task_name`
- `sync_source`
- `observer_type`
- `callback_type`
- `action`
- `result`
- `stage`

Forbidden labels:

- `trace_id`
- `span_id`
- `request_id`
- `release_id`
- `manifest_id`
- `application_id`
- `environment_id`
- raw error strings

### Exemplar rules

#### Failure counters

Failure counters should support exemplars whenever a trace context or
background-created span context exists.

This includes:

- `manifest_failed_total`
- `manifest_task_failed_total`
- `app_config_sync_failed_total`
- `release_failed_total`
- `release_stage_failed_total`
- `argo_application_create_failed_total`
- `argo_rollout_failed_total`
- `runtime_observer_sync_failed_total`
- `release_writeback_failed_total`
- `runtime_action_failed_total`
- existing HTTP 5xx counters
- existing dependency error counters

#### Duration histograms

Important control-plane duration histograms should offer exemplars on failed and
slow samples.

This includes:

- `manifest_duration_seconds`
- `manifest_task_duration_seconds`
- `app_config_sync_duration_seconds`
- `release_duration_seconds`
- `release_stage_duration_seconds`
- `argo_application_create_duration_seconds`
- `argo_rollout_duration_seconds`
- `runtime_observer_sync_duration_seconds`
- `release_writeback_duration_seconds`
- `runtime_action_duration_seconds`

#### Metrics that do not need exemplars

Resource and capacity metrics do not need exemplars:

- success totals
- in-flight gauges
- pod CPU
- pod memory
- node resource metrics

### Planned metric families

#### 1. Manifest metrics

Purpose:

- count how many manifests entered build flow
- count build success and failure
- observe build latency

Metrics:

- `manifest_total`
- `manifest_success_total`
- `manifest_failed_total`
- `manifest_duration_seconds`

Primary ownership:

- `release-service`

Candidate seams:

- `internal/manifest/service`
- `internal/manifest/transport/http`

#### 2. Manifest task / Tekton task metrics

Purpose:

- show which build task is failing
- show task latency by bounded task name

Metrics:

- `manifest_task_total`
- `manifest_task_failed_total`
- `manifest_task_duration_seconds`

Primary ownership:

- `runtime-service`

Candidate seam:

- `internal/runtime/observer/tekton_manifest.go`

#### 3. Config sync metrics

Purpose:

- count sync attempts from config repository
- count sync success and failure
- observe sync duration

Metrics:

- `app_config_sync_total`
- `app_config_sync_success_total`
- `app_config_sync_failed_total`
- `app_config_sync_duration_seconds`

Stable label value:

- `sync_source="git_repo"`

Primary ownership:

- `config-service`

Candidate seams:

- `internal/platform/configrepo/repository.go`
- the app-config sync service path that calls repository sync

#### 4. Release stage metrics

Purpose:

- make release failures attributable to a specific release stage
- observe duration of each release stage

Metrics:

- `release_stage_total`
- `release_stage_failed_total`
- `release_stage_duration_seconds`

Stable bounded `stage` set:

- `render_bundle`
- `publish_bundle`
- `create_argocd_application`
- `start_deployment`
- `observe_rollout`
- `finalize_release`

Primary ownership:

- `release-service`

Candidate seam:

- canonical release orchestration in `internal/release/service`

#### 5. Argo application creation metrics

Purpose:

- separate release creation from Argo application creation success/failure

Metrics:

- `argo_application_create_total`
- `argo_application_create_success_total`
- `argo_application_create_failed_total`
- `argo_application_create_duration_seconds`

Primary ownership:

- `release-service`

Candidate seams:

- release orchestration path that creates the Argo application
- `internal/release/transport/argo`

#### 6. Argo rollout metrics

Purpose:

- separate Argo application creation from actual rollout convergence

Metrics:

- `argo_rollout_total`
- `argo_rollout_success_total`
- `argo_rollout_failed_total`
- `argo_rollout_duration_seconds`

Primary ownership:

- runtime observer path plus release writeback boundary

Candidate seams:

- `internal/runtime/observer/release_rollout.go`
- `internal/release/transport/http/release_writeback.go`

#### 7. Observer sync metrics

Purpose:

- prove that background observers are still polling and not failing silently

Metrics:

- `runtime_observer_sync_total`
- `runtime_observer_sync_failed_total`
- `runtime_observer_sync_duration_seconds`

Stable bounded `observer_type` set:

- `kubernetes_runtime`
- `tekton_manifest`
- `release_rollout`

Primary ownership:

- `runtime-service`

Candidate seams:

- `internal/runtime/observer/kubernetes_runtime.go`
- `internal/runtime/observer/tekton_manifest.go`
- `internal/runtime/observer/release_rollout.go`

#### 8. Writeback metrics

Purpose:

- show whether callback/writeback traffic is arriving and succeeding
- isolate failure by callback family

Metrics:

- `release_writeback_total`
- `release_writeback_failed_total`
- `release_writeback_duration_seconds`

Stable bounded `callback_type` set:

- `argo_event`
- `release_step`
- `manifest_status`
- `manifest_task`
- `manifest_result`

Primary ownership:

- release and manifest writeback ingress boundaries

Candidate seams:

- `internal/release/transport/http/release_writeback.go`
- `internal/manifest/transport/http/manifest_writeback.go`

#### 9. Runtime action metrics

Purpose:

- show whether operator-style runtime actions are failing or becoming slow

Metrics:

- `runtime_action_total`
- `runtime_action_failed_total`
- `runtime_action_duration_seconds`

Stable bounded `action` set:

- `delete_pod`
- `rollout_workload`
- `sync_runtime_pod`
- `sync_runtime_workload`

Primary ownership:

- `runtime-service`

Candidate seams:

- `internal/runtime/service`
- `internal/runtime/transport/http/handler.go`

## Implementation priority

### P0

Implement first because they unlock the core delivery-path diagnosis surface:

1. `release_stage_*`
2. `app_config_sync_*`
3. `argo_application_create_*`
4. `argo_rollout_*`
5. `release_writeback_*`

### P1

6. `manifest_*`
7. `manifest_task_*`
8. `runtime_observer_sync_*`

### P2

9. `runtime_action_*`

## Error handling and trace context expectations

The exemplar design assumes:

- HTTP-driven control-plane paths already have request trace context
- background observers create their own spans so failures still attach to a
  traceable context even without a user request
- metrics never copy trace identifiers into labels

If a failure occurs outside any trace context, the metric should still record
correctly, but no exemplar will be attached.

## Testing and verification consequences

When implementation begins, verification should expand to cover at least:

- new metric names appear in unit/integration tests where appropriate
- new labels remain low-cardinality and bounded
- failure-series attach exemplars when a trace context exists
- dashboard queries do not use forbidden high-cardinality labels
- old dashboards are absent from the supported folder structure

## Open questions resolved by this design

- Should old DevFlow dashboards be deleted? **Yes.**
- Should production and pre-production have separate folders? **Yes.**
- Should the two environments be structurally identical? **Yes.**
- Should `Delivery / ...` be first-class? **Yes.**
- Should `Page/API / ...` initially mean API routes only? **Yes.**

## Final recommendation

Proceed in two phases:

### Phase 1

Restructure Grafana and delete the old DevFlow dashboards.

Deliver:

- `devflow-production` folder
- `devflow-pre-production` folder
- the immediately-supported dashboards built from the current metric surface

### Phase 2

Implement the control-plane metrics expansion in `devflow-service`, then add the
remaining `Delivery / ...` dashboards once their metrics exist.

This sequencing keeps the Grafana structure clean immediately while avoiding
pretend dashboards that query metrics the codebase does not yet emit.
