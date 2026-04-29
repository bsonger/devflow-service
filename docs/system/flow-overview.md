# Flow Overview

## Purpose

This document is the reader-first overview of the current DevFlow delivery chain.
Use it when you want one place to understand how the active services connect across:

- build input selection
- manifest freeze
- release freeze
- bundle render and publish
- Argo CD deployment
- runtime observation and runtime actions

This document is intentionally overview-oriented.
For exact field contracts and route contracts, follow the linked owning docs.

## Quick reader guide

Use this document when your first question is:

- how does one application move from source to running workload
- where are the freeze points
- which service owns which stage of the flow
- where should I look when a build, deploy, or runtime page looks wrong

If you already know which resource you need, go directly to:

- `docs/resources/manifest.md`
- `docs/resources/release.md`
- `docs/resources/runtime-spec.md`

## End-to-end stages

The current end-to-end chain is:

1. metadata resolution
2. manifest freeze and image build
3. release freeze
4. deployment bundle render
5. OCI artifact publish
6. Argo CD deployment
7. runtime observation
8. runtime operator actions

## Stage 1. Metadata resolution

Owning truth:

- `meta-service`
- `config-service`
- `network-service`

What happens:

- `meta-service` provides application / environment / cluster metadata
- `config-service` provides workload config and app config truth
- `network-service` provides service and route truth

Why this matters:

- later stages should freeze these upstream facts instead of re-querying them forever

## Stage 2. Manifest freeze and image build

Owning service:

- `release-service`

Owning resource:

- `Manifest`

What gets frozen:

- application metadata needed for build
- `workload_config_snapshot`
- `services_snapshot`
- requested `git_revision`
- resolved immutable `commit_hash`

What happens next:

- `release-service` starts Tekton image build
- Tekton produces workload image output
- manifest status and steps are written back onto the durable manifest row

Key boundary rule:

- `Manifest` is the build-side freeze point
- `Manifest` does not own environment-specific deploy inputs

See:

- `docs/resources/manifest.md`
- `docs/system/diagrams.md`

## Stage 3. Release freeze

Owning service:

- `release-service`

Owning resource:

- `Release`

What gets frozen:

- `manifest_id`
- `environment_id`
- `app_config_snapshot`
- `routes_snapshot`
- rollout `strategy`
- initial execution `steps`

Where those inputs come from:

- build-side frozen inputs come from persisted `Manifest`
- deploy-time config comes from `config-service`
- deploy-time route inputs come from `network-service`
- target deploy metadata comes from `meta-service`

Key boundary rule:

- `Release` is the deploy-side freeze point
- `Release` consumes `Manifest`; it does not replace `Manifest`

See:

- `docs/resources/release.md`
- `docs/system/release-steps.md`

## Stage 4. Deployment bundle render

Owning service:

- `release-service`

What happens:

- release-time frozen inputs are combined into final Kubernetes objects
- one canonical rendered bundle fact is produced for the release
- bundle preview reads from this release-owned rendered output

Typical rendered objects:

- `ServiceAccount`
- `ConfigMap`
- `Service`
- `Deployment` or `Rollout`
- `VirtualService` when route flow is active

Key boundary rule:

- rendered deployment YAML belongs to `Release`, not `Manifest`

## Stage 5. OCI artifact publish

Owning service:

- `release-service`

What happens:

- rendered bundle is packaged as one OCI artifact payload
- artifact is pushed to the configured registry
- artifact metadata is written back onto `Release`

Primary release-side artifact fields:

- `artifact_repository`
- `artifact_tag`
- `artifact_digest`
- `artifact_ref`

Key boundary rule:

- workload image output belongs to `Manifest`
- deployment bundle artifact belongs to `Release`

## Stage 6. Argo CD deployment

Owning service:

- `release-service`

External controller:

- Argo CD

What happens:

- `release-service` creates or updates an Argo CD `Application`
- Argo CD pulls the release-owned OCI artifact
- Argo CD syncs the rendered bundle into Kubernetes
- `release-service` stops at deployment initiation and does not own long-running rollout observation
- `release-service` does not read Argo CD application status during normal release detail reads
- rollout progress should be observed by `runtime-service` and written back onto the release record

Key boundary rule:

- Argo deploys the release-generated bundle, not the original config repo directly

See:

- `docs/system/release-writeback.md`
- `docs/system/diagrams.md`

## Stage 7. Runtime observation

Owning service:

- `runtime-service`

What happens:

- runtime observer/index state tracks workload summary and pod state
- runtime-service now also runs the rolling release rollout observer for active releases
- that observer checks Kubernetes Deployment health for running rolling releases
- it advances `start_deployment`, `observe_rollout`, and `finalize_release` through release writeback routes
- runtime reads should come from runtime-owned observed data
- runtime page display should not require direct Kubernetes reads for every refresh

Primary runtime read surfaces:

- `GET /api/v1/runtime/workload`
- `GET /api/v1/runtime/pods`

Key boundary rule:

- runtime read model is separate from build/deploy freeze records

See:

- `docs/resources/runtime-spec.md`
- `docs/system/runtime-observer.md`

## Stage 8. Runtime operator actions

Owning service:

- `runtime-service`

What happens:

- operator can delete one pod
- operator can restart / rollout one workload
- these action routes call Kubernetes explicitly
- after success, UI should refresh runtime read surfaces from the observer/index model
- restart can now resolve the primary Deployment server-side and does not require the UI to send `deployment_name` in the common path

Primary runtime action surfaces:

- `DELETE /api/v1/runtime/pods/{pod_name}`
- `POST /api/v1/runtime/rollouts`

Key boundary rule:

- runtime actions mutate Kubernetes
- runtime reads prefer observer/index-backed state

## Freeze-point summary

### Build-side freeze point

Resource:

- `Manifest`

Answers:

- what was built
- which commit was built
- which image was produced
- what service/workload shape was frozen for build

### Deploy-side freeze point

Resource:

- `Release`

Answers:

- what was deployed
- where it was deployed
- which config and routes were frozen
- which deployment artifact was published
- what happened during rollout

### Runtime read/action boundary

Surface:

- runtime API

Answers:

- what is running now
- what pods are currently observed
- restart this workload
- delete this pod

## Failure routing cheat sheet

If the problem looks like this, start here:

- build did not start or image result is wrong
  - `docs/resources/manifest.md`
- deployment bundle looks wrong
  - `docs/resources/release.md`
  - `docs/system/diagrams.md`
- Argo rollout state looks wrong
  - `docs/system/release-writeback.md`
  - `docs/system/release-steps.md`
- runtime page data looks stale or duplicated
  - `docs/resources/runtime-spec.md`
  - `docs/system/runtime-observer.md`
- service ownership is unclear
  - `docs/services/`
  - `docs/system/diagrams.md`

## Source pointers

- architecture overview: `docs/system/architecture.md`
- visual diagrams: `docs/system/diagrams.md`
- manifest contract: `docs/resources/manifest.md`
- release contract: `docs/resources/release.md`
- runtime contract: `docs/resources/runtime-spec.md`
- runtime observer/index model: `docs/system/runtime-observer.md`
