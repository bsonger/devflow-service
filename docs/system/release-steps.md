# Release Steps

## Reader and action

Reader:
- internal engineer, operator, or agent working on release execution, release UI, or release callback handling

Post-read action:
- look at any `Release.steps[*].code` value and immediately know what that step means, who owns it, what work should happen there, and what success or failure means operationally

## Purpose

This document is the current repo-local execution guide for release step semantics.

Use it when you need to answer questions such as:

- what does a given release step actually do
- which component is supposed to advance this step
- why is a release stuck on one step
- which steps differ between rolling, blue-green, and canary release strategies

This document is about execution semantics.
The resource contract for `Release` still lives in `docs/resources/release.md`.
The writeback route contract still lives in `docs/system/release-writeback.md`.

## Core model

Every release has:

- one top-level `status`
- one strategy-specific ordered `steps` list
- one stable machine-oriented `step code`
- one human-oriented `step name`

Important rules:

- `steps[*].code` is the stable identifier and should be used by writeback and automation
- `steps[*].name` is display text and may evolve over time
- the frontend should render the returned `steps` list instead of assuming a fixed step count
- the release create call initializes the full step list early, then release dispatch and later callback senders advance different steps over time
- release-service should keep `steps` in canonical execution order and should not append unknown ad-hoc step entries at runtime

## Execution phases

Release steps are advanced by three owners:

1. release create phase
Purpose:
- validate inputs
- freeze deployment inputs
- create the release record and initial steps

2. release execution phase
Purpose:
- build the environment-specific deployment bundle
- publish that bundle
- create the external deployment object

3. rollout callback phase
Purpose:
- accept rollout-state callbacks after deployment handoff
- progress traffic movement when callback senders are wired
- finalize the release outcome

## Step overview
Current wiring note:

- `release-service` advances create/render/publish/Argo-start steps directly during normal create/dispatch
- release-owned callback routes under `/api/v1/verify/...` can advance later rollout steps
- `internal/runtime/observer/release_rollout.go` exists in-tree, but the active `runtime-service` startup path does not start it


### Common steps for all strategies

These steps appear on all release strategies:

- `freeze_inputs`
- `ensure_namespace`
- `ensure_pull_secret`
- `ensure_appproject_destination`
- `render_deployment_bundle`
- `publish_bundle`
- `create_argocd_application`
- `finalize_release`

### Rolling-only steps

- `start_deployment`
- `observe_rollout`

### Blue-green-only steps

- `deploy_preview`
- `observe_preview`
- `switch_traffic`
- `verify_active`

### Canary-only steps

- `deploy_canary`
- `canary_10`
- `canary_30`
- `canary_60`
- `canary_100`

## Step-by-step meaning

### `freeze_inputs`

Owner:
- release-service create phase

Meaning:
- the service validates the release request and freezes the deployment inputs that must not drift during execution

What should happen:

- validate `manifest_id`
- validate `environment_id`
- normalize `strategy`
- default `type` when needed
- resolve the target application and environment relationship
- ensure the chosen manifest is deployable
- freeze `app_config_snapshot`
- read frozen manifest-side build inputs such as image and workload snapshots
- initialize the strategy-specific step list
- persist the initial release record

Success means:

- the release record exists
- immutable deployment inputs are persisted on the release
- the remaining steps are initialized and pending

Failure usually means:

- manifest is not ready
- effective app config is missing
- deploy target cluster is not ready
- required downstream runtime dependency is unavailable

### `render_deployment_bundle`

Owner:
- release-service dispatch path

Meaning:
- the executor renders environment-specific deployment objects from frozen manifest inputs plus release-time inputs

What should happen:

- combine manifest image and workload snapshots with release environment inputs
- render Kubernetes YAML for the selected rollout strategy
- prepare the final deployment bundle that will be published and deployed
- fix one canonical rendered bundle fact for that release

Success means:

- the deployment bundle content is rendered without validation failure
- the system has one stable rendered output for that release, even if publication happens later

Failure usually means:

- invalid frozen inputs
- render logic failure
- unsupported strategy-specific configuration

### `publish_bundle`

Owner:
- release-service dispatch path

Meaning:
- the rendered deployment bundle is published to OCI as a release-owned artifact

What should happen:

- upload the bundle
- consume the already-rendered bundle fact for that release
- resolve artifact identity
- write back:
  - `artifact_repository`
  - `artifact_tag`
  - `artifact_digest`
  - `artifact_ref`

Success means:

- the release has a usable deployment artifact reference

Failure usually means:

- OCI publish failure
- registry authentication or connectivity failure
- artifact writeback failure

### `create_argocd_application`

Owner:
- release-service dispatch path

Meaning:
- the deployment controller object for this release is created and pointed at the published artifact

What should happen:

- create or update the ArgoCD application for the target environment
- bind the published artifact reference into that application
- persist external deployment identity such as the application name or external reference

Success means:

- the external deployment controller has accepted the release for rollout

Failure usually means:

- ArgoCD application create or update failed
- deployment target metadata is invalid
- cluster-side controller configuration is missing or inconsistent

### `start_deployment`

Owner:
- release-service dispatch path

Strategy:
- rolling only

Meaning:
- the standard rolling deployment has been kicked off and should now start moving in the cluster

What should happen:

- ensure the deployment object references the new artifact
- submit or activate the rollout in a way that lets Kubernetes begin the update

Success means:

- rollout start has been handed over to the runtime or cluster layer

Failure usually means:

- rollout could not be triggered
- deployment controller rejected the desired state

### `observe_rollout`

Owner:
- rollout callback sender

Strategy:
- rolling only

Meaning:
- the system is waiting for the rolling deployment to converge to a healthy terminal state

What should happen:

- observe rollout progress
- update messages as rollout state changes
- mark failure if the rollout becomes unhealthy or times out

Success means:

- the rolling update completed successfully and the new workload is active

Failure usually means:

- pods failed readiness or startup
- rollout stalled
- a callback sender reported a terminal unhealthy state

### `deploy_preview`

Owner:
- rollout callback sender

Strategy:
- blue-green only

Meaning:
- the new version is deployed as the preview environment before traffic moves

What should happen:

- create or activate the preview workload
- make the preview addressable for verification

Success means:

- preview deployment exists and is ready for observation

Failure usually means:

- preview workload failed to create
- rollout object could not reach a preview-ready state

### `observe_preview`

Owner:
- rollout callback sender

Strategy:
- blue-green only

Meaning:
- the system is checking whether the preview environment is healthy enough for traffic switch

What should happen:

- observe pod and workload health
- wait for readiness and stability
- block traffic switch until preview is healthy

Success means:

- preview is healthy and traffic can be switched safely

Failure usually means:

- preview readiness failed
- preview remained unstable
- health checks never converged

### `switch_traffic`

Owner:
- rollout callback sender

Strategy:
- blue-green only

Meaning:
- user traffic is moved from the old active version to the new active version

What should happen:

- switch the serving path to the new version
- record the state transition clearly

Success means:

- live traffic now points at the new release

Failure usually means:

- traffic switch operation failed
- routing or service cutover did not converge

### `verify_active`

Owner:
- rollout callback sender

Strategy:
- blue-green only

Meaning:
- after traffic switch, the system verifies that the newly active version is healthy under real serving conditions

What should happen:

- confirm post-switch health
- ensure the new active version remains stable

Success means:

- the switched version is healthy and the rollout can be finalized

Failure usually means:

- the active version became unhealthy after cutover
- rollback or remediation may be needed

### `deploy_canary`

Owner:
- rollout callback sender

Strategy:
- canary only

Meaning:
- the initial canary version is deployed and prepared for staged traffic movement

What should happen:

- deploy the canary workload
- prepare routing or rollout state for progressive percentages

Success means:

- staged traffic progression can begin

Failure usually means:

- the initial canary deployment could not become ready

### `canary_10`

Owner:
- rollout callback sender

Strategy:
- canary only

Meaning:
- 10% of traffic is moved to the new version

What should happen:

- shift traffic to the 10% stage
- verify that the stage remains healthy before continuing

Success means:

- the first canary stage is healthy and progression can continue

Failure usually means:

- errors or instability appeared at the first staged traffic level

### `canary_30`

Owner:
- rollout callback sender

Strategy:
- canary only

Meaning:
- 30% of traffic is moved to the new version

What should happen:

- advance traffic from the previous stage
- watch for regression before continuing

Success means:

- the 30% stage is healthy and progression can continue

Failure usually means:

- error rate or health degraded after higher traffic exposure

### `canary_60`

Owner:
- rollout callback sender

Strategy:
- canary only

Meaning:
- 60% of traffic is moved to the new version

What should happen:

- continue staged traffic increase
- verify stability under a majority of traffic

Success means:

- the release remains healthy under majority traffic

Failure usually means:

- instability became visible only at higher traffic levels

### `canary_100`

Owner:
- rollout callback sender

Strategy:
- canary only

Meaning:
- all traffic is moved to the new version after earlier stages passed

What should happen:

- promote the release from staged rollout to full traffic ownership
- verify the new version is now serving the whole workload

Success means:

- full traffic has reached the new version and rollout is ready for finalization

Failure usually means:

- final promotion failed
- the new version became unhealthy at full traffic

### `finalize_release`

Owner:
- rollout callback sender

Meaning:
- the system closes the rollout and writes the final terminal outcome

What should happen:

- confirm the strategy-specific rollout reached its terminal condition
- write final step outcome
- let top-level release status settle to success, failure, or rollback terminal state

Success means:

- the release is truly finished and should not continue polling as active work

Failure usually means:

- a terminal verification failed late
- the rollout ended in a final unhealthy state

## Strategy timelines

### Rolling

Expected sequence:

1. `freeze_inputs`
2. `ensure_namespace`
3. `ensure_pull_secret`
4. `ensure_appproject_destination`
5. `render_deployment_bundle`
6. `publish_bundle`
7. `create_argocd_application`
8. `start_deployment`
9. `observe_rollout`
10. `finalize_release`

### Blue-green

Expected sequence:

1. `freeze_inputs`
2. `ensure_namespace`
3. `ensure_pull_secret`
4. `ensure_appproject_destination`
5. `render_deployment_bundle`
6. `publish_bundle`
7. `create_argocd_application`
8. `deploy_preview`
9. `observe_preview`
10. `switch_traffic`
11. `verify_active`
12. `finalize_release`

### Canary

Expected sequence:

1. `freeze_inputs`
2. `ensure_namespace`
3. `ensure_pull_secret`
4. `ensure_appproject_destination`
5. `render_deployment_bundle`
6. `publish_bundle`
7. `create_argocd_application`
8. `deploy_canary`
9. `canary_10`
10. `canary_30`
11. `canary_60`
12. `canary_100`
13. `finalize_release`

## How to read a stuck release

Use this order:

1. read the top-level `status`
2. find the latest non-terminal step
3. identify the owner of that step
4. inspect the systems that own that phase

Practical routing:

- stuck at `freeze_inputs`:
  inspect release create validation, manifest readiness, app config availability, and deploy target readiness
- stuck at `render_deployment_bundle` or `publish_bundle`:
  inspect release-service dispatch and artifact publication flow
- stuck at `create_argocd_application`:
  inspect ArgoCD application creation and deployment target metadata
- stuck at rolling, blue-green, or canary observation steps:
  inspect release writeback routes and whichever callback sender is expected in that environment
- stuck at `finalize_release`:
  inspect the last rollout callback and terminal-state writeback path

## Relationship to top-level release status

The top-level release status is the summary.
The steps explain why that summary exists.

Use the top-level status for:

- list-page badges
- quick filtering
- terminal versus active distinctions

Use the step list for:

- detail-page progress
- operator debugging
- writeback targeting
- explaining what the release is doing right now

## Related docs

- `docs/resources/release.md` for the release contract and API surface
- `docs/system/release-writeback.md` for callback and writeback route behavior
- `docs/resources/frontend-ui.md` for the frontend detail-page and polling contract
