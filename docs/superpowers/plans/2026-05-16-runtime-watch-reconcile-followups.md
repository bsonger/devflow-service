# Runtime Watch/Reconcile Remaining Follow-ups

## Current state

The queue-driven runtime migration has landed in code and is now pushed on `main`.

Implemented:

- release queue/reconcile path
- manifest queue/reconcile path
- shared release writeback extraction
- informer-backed Tekton snapshot cache for manifest reconcile
- event-driven manifest enqueue from Tekton `PipelineRun` / `TaskRun` watch events
- release enqueue from workload watch events
- manifest result writeback aligned with legacy `commit_hash` / `image_ref` / `image_tag` / `image_digest` semantics
- startup guard that disables legacy `TektonManifestObserver` when `observer.manifest_runtime_enabled=true`
- startup guard that disables legacy `ReleaseRolloutObserver` when `observer.release_runtime_enabled=true`
- cache-first pod read verification for rollout inspection
- runtime-service config support for:
  - `observer.manifest_runtime_enabled`
  - `observer.manifest_runtime_workers`
  - `observer.release_runtime_enabled`
  - `observer.release_runtime_workers`
  - `observer.tekton_pipeline`
- `production` and `staging` runtime-service config updates in `devflow-config-repo`

Already pushed:

- `devflow-service`: `55ec80cc5c06523ceda0b82f6810af6aee6e86b7`
- `devflow-config-repo`: `c8beb201cf07d20402b599eac4cfe93f9c6d115d`

## Remaining work

### 1. Replace polling-backed Tekton cache with informer/watch

Status: implemented in code, not yet rolled out and verified in a live environment.

Still needed:

- verify live informer/watch behavior in `staging`
- confirm no missed initial manifest enqueue after runtime-service startup
- remove or demote the old polling fallback path after rollout confidence is high

Acceptance:

- no periodic full Tekton list in steady state
- manifest reconcile still reads from current snapshot, not raw event payloads
- control-plane and `tekton_pipeline` filters remain preserved

### 2. Cut over manifest processing from legacy polling observer

Current state:

- queue-driven manifest runtime exists
- legacy `TektonManifestObserver` still exists as compatibility fallback

Still needed:

- enable queue-driven manifest runtime in a real environment and verify behavior
- confirm no legacy-only writeback behavior remains
- keep `tekton_manifest_enabled=false` in live config once cutover is confirmed
- disable or remove legacy polling path after verification

Acceptance:

- manifest status/task/result writebacks are identical before and after cutover
- no duplicate terminal writebacks
- docs reflect the actual active path

### 3. Cut over release processing from legacy rollout polling observer

Current state:

- queue-driven release runtime exists
- legacy rollout observer still exists as compatibility fallback

Still needed:

- enable `observer.release_runtime_enabled` in a real environment
- verify release steps writeback parity
- keep legacy rollout observer disabled in live config once cutover is confirmed
- remove or disable legacy polling rollout path after parity is proven

Acceptance:

- only running releases owned by the current control plane reconcile
- rollout/finalize step behavior matches existing contract
- no extra polling-only writebacks remain

### 4. Replace release source polling with event-driven source where practical

Status: partially implemented in code.

Still needed:

- verify workload-watch-driven release enqueue against real rollout traffic
- decide whether `RunningReleaseSource` remains as fallback/recovery path or is removed
- keep reconcile idempotent and snapshot-driven
- use polling only where there is no reliable event source

Acceptance:

- `Manifest` and `Release` state changes enqueue reconcile keys through watches
- polling no longer drives the primary control loop for these resources

### 5. Add local cache for Pod reads used by rollout inspection

Status: cache-first read path exists and is now explicitly tested.

Still needed:

- review remaining direct workload API fallback usage and decide whether more should move behind cache-first helpers
- reduce direct `get/list pod` pressure during rollout observation where fallback is still hot

Acceptance:

- steady-state rollout inspection uses local cache
- API fallback remains only for explicit recovery/error cases

### 6. Environment rollout and rollback plan

Still needed:

- decide enablement order between `staging` and `production`
- define rollback toggles for:
  - `observer.manifest_runtime_enabled`
  - `observer.release_runtime_enabled`
  - legacy polling observers
- define runtime validation checklist after deployment

Recommended order:

1. enable in `staging`
2. verify manifest writeback parity
3. verify release step parity
4. disable legacy fallback in `staging`
5. repeat in `production`

## Verification checklist for next phase

- `go test ./internal/runtime/watch ./internal/runtime/reconcile ./internal/runtime/bootstrap ./internal/runtime/config -v`
- `go build -o /tmp/runtime-service ./cmd/runtime-service`
- live validation against one running manifest build
- live validation against one running release rollout
- release-service writeback logs show no duplicate terminal transitions
- runtime-service logs show reconcile-driven flow for the enabled lane

## Notes

- `observer.tekton_pipeline` currently filters by `PipelineRun.spec.pipelineRef.name`
- manifest runtime is now informer-backed in code when cluster config is available
- release runtime now prefers workload watch events in code when cluster config is available
- this file is the remaining-work handoff, not the current execution truth; current behavior remains documented in `docs/system/observability.md`
