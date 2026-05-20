# Release Code Map

## Reader and outcome

Reader:
- engineer or agent who needs to change `release-service` lifecycle code without re-learning the whole execution path from scratch

Outcome:
- know which file owns each release lifecycle concern
- know which entrypoint to read first for create, dispatch, operation, timeout, and callback-related behavior
- avoid reintroducing pass-through abstractions or scattering ownership across unrelated files

## Purpose

This document is the repo-local code map for the current `release-service` lifecycle implementation.

Use it when you need to answer questions such as:

- where release create starts
- where dispatch moves from bundle render to Argo handoff
- where operator actions such as pause, cancel, and rollback live
- where terminal release status convergence is applied
- which file owns rollout observe-state terminal cleanup

This is not the resource contract.
For API/resource semantics and step meaning, continue to use:

- `docs/resources/release.md`
- `docs/system/release-steps.md`
- `docs/system/release-writeback.md`
- `docs/services/release-service.md`

## Read order

If you are new to the code path, read in this order:

1. `internal/release/service/release.go`
2. `internal/release/service/release_prepare_runtime.go`
3. `internal/release/service/release_handoff_runtime.go`
4. `internal/release/service/release_observe_controller.go`
5. `internal/release/service/release_status_manager.go`
6. `internal/release/service/release_operation_manager.go`
7. `internal/release/service/release_timeout_runtime.go`
8. `internal/release/service/release_remediation_manager.go`

## Ownership map

### Top-level orchestrator

File:
- `internal/release/service/release.go`

Owns:
- release create entrypoint
- dispatch entrypoint
- release load/list helpers
- step normalization
- release bundle preview/read helpers
- top-level orchestration glue between the specialized runtimes/controllers

Read here first when:
- you want the shortest path from HTTP create into actual execution
- you need to see the ordered dispatch flow
- you are checking whether a concern belongs in the orchestrator or in a more focused owner

Current important entrypoints:
- `Create`
- `DispatchRelease`
- `executeReleasePhases`
- `UpdateStep`

### Prepare phase

File:
- `internal/release/service/release_prepare_runtime.go`

Owns:
- `render_deployment_bundle`
- `publish_bundle`
- OCI artifact metadata derivation from rendered bundle content
- publish-step operator messages

Read here when:
- release bundle rendering changed
- OCI artifact publication changed
- bundle metadata fields such as `artifact_repository`, `artifact_digest`, or `artifact_ref` changed

### Handoff phase

File:
- `internal/release/service/release_handoff_runtime.go`

Owns:
- `create_argocd_application`
- Argo application metadata persistence
- Argo application sync request
- handoff-step operator messages
- release-owned handoff step selection such as `start_deployment`, `deploy_preview`, or `deploy_canary`

Read here when:
- Argo CD handoff changed
- release-owned deployment start step semantics changed
- application labels/annotations on the Argo `Application` changed

### Observe terminal convergence

File:
- `internal/release/service/release_observe_controller.go`

Owns:
- terminal `observe-state=done` updates on the Argo `Application`
- terminal `observe-state=done` updates on rendered workloads
- release-owned terminal cleanup after top-level status closure

Read here when:
- terminal workload observe-state updates changed
- Argo/workload convergence cleanup changed
- release closes correctly but cluster metadata cleanup looks wrong

### Status transitions

File:
- `internal/release/service/release_status_manager.go`

Owns:
- top-level release status transition write
- terminal metrics emission trigger
- rollback-source remediation sync trigger
- observe-terminal trigger after durable status change

Read here when:
- top-level `Release.status` changes are wrong
- terminal metrics are missing or duplicated
- rollback source remediation state is not converging

### Operator actions

File:
- `internal/release/service/release_operation_manager.go`

Owns:
- pause
- resume
- cancel
- rollback request handling
- rollback target selection
- rollback release creation
- cancellation-triggered remediation marking

Read here when:
- operator actions behave incorrectly
- rollback picks the wrong target
- cancel should or should not trigger cleanup/rollback

### Timeout policy and control-state compatibility

File:
- `internal/release/service/release_timeout_runtime.go`

Owns:
- timeout scanning
- compatibility mapping from persisted release fields into control-state
- timeout-driven step writes
- timeout-driven top-level status closure

Read here when:
- a release stalls forever
- pause/finalize/observe timeout behavior is wrong
- control-state mapping and persisted release state disagree

### Remediation source sync

File:
- `internal/release/service/release_remediation_manager.go`

Owns:
- syncing rollback-source remediation outcome from rollback release terminal state

Read here when:
- source release remediation status does not move to `RollbackDone` or `RollbackFailed`

## Dispatch flow map

The current non-intent dispatch flow is:

1. `release.go:Create`
2. `release.go:DispatchRelease`
3. `release.go:executeReleasePhases`
4. `release_prepare_runtime.go:renderDeploymentBundle`
5. `release_prepare_runtime.go:publishDeploymentBundle`
6. `release_handoff_runtime.go:run`
7. later callback/observer paths advance callback-owned rollout steps
8. `release_status_manager.go:updateStatus` closes top-level terminal truth
9. `release_observe_controller.go:runTerminal` marks terminal observe-state cleanup

## Design rules for future changes

- Keep `release.go` as the orchestrator, not the place where every detail accumulates again.
- Do not reintroduce pass-through controller layers that only forward to another owner.
- Prefer one owner per lifecycle concern:
  - prepare
  - handoff
  - observe terminal cleanup
  - status transition
  - operator action
  - timeout/remediation
- If a helper exists only to preserve an older call site and has no independent ownership value, either move callers to the real owner or delete the shim.
- When adding a new release lifecycle concern, decide whether it is:
  - phase execution
  - terminal convergence
  - operator action
  - timeout/control-state logic
  - remediation
  before choosing its file.

## Verification hints

When changing this area, prioritize:

```sh
go test ./internal/release/service
go test ./internal/release/transport/http ./internal/runtime/observer
bash scripts/verify.sh
```

If the change touches public API or resource semantics, also check:

```sh
make openapi-check
```
