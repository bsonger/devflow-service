# Metadata drift proof: `meta-service` Argo sync seam

This document is a tracked evidence artifact for the live `meta-service` Argo CD drift investigation.

It is intentionally code-adjacent: it ties the live Argo `Application` status surface to the repo-owned `ignoreDifferences` contract, the release-owned identity labels that correlate runtime rollouts back to releases, and the runtime mutation path that can still produce drift-like symptoms. Future executors should start here instead of re-reading planner notes or assuming the problem is still only `kubectl.kubernetes.io/restartedAt`.

It is not a second authority for lifecycle ownership or terminality. For normative wording, return to:

- `docs/system/flow-overview.md`
- `docs/system/release-steps.md`
- `docs/system/release-writeback.md`

## Why this artifact exists

Before changing Argo ignore rules, we need a repeatable way to answer four questions against the real `meta-service` `Application`:

1. **What does Argo currently think is OutOfSync?**
2. **Does the live `Application` still target the same ignore-difference contract the repo renders?**
3. **Does runtime/release correlation still come from the release-owned label contract rather than runtime-local ownership data?**
4. **Is the remaining drift on the known runtime restart annotation path, a workload-kind mismatch, or a different object/field entirely?**

This artifact gives a single workflow that answers those questions, while still being usable when cluster access is unavailable in the current environment.

## Evidence scope and proof split

This file is evidence, not an alternate authority.

Use the proof surfaces in this order:

1. focused Go seam tests for behavioral ownership and terminality
2. `bash scripts/verify-metadata-audit.sh` for metadata/doc routing consistency only
3. this live proof workflow to localize a real cluster drift report
4. `bash scripts/verify.sh` as the final repo-wide anti-drift gate

That split keeps live drift localization grounded in tracked evidence without letting this document overclaim ownership of release semantics already defined elsewhere.

## Current repo-backed hypothesis set

The repo already proves all of the following:

- runtime-service writes `spec.template.metadata.annotations["kubectl.kubernetes.io/restartedAt"]` onto live workload objects via `internal/runtime/service/service.go`
- runtime observers still read that timestamp back as a real operational signal
- runtime rollout writeback derives release/application/environment correlation from workload labels via `internal/runtime/observer/release_rollout.go`
- release-service already renders an Argo `ignoreDifferences` entry for the actual release workload kind, while preserving the narrow restartedAt-only ignore pointer
- release/runtime identity correlation depends on canonical labels, not on drift-prone annotations

That means both **"the ignore rule is missing"** and **"runtime needs its own release truth store to correlate rollouts"** are no longer valid default assumptions.

## Code seam to inspect first

The release Argo client now exposes a structured inspection seam in:

- `internal/release/transport/argo/client.go`

The seam summarizes the fields that matter for this investigation:

- application name / namespace / project
- `status.sync.status`
- `status.health.status`
- `status.operationState.phase` and message
- `status.reconciledAt`
- source repo URL and target revision
- destination server/namespace
- rendered `spec.ignoreDifferences` targets
- the concrete `status.resources[]` entries Argo currently marks `OutOfSync`
- the subset of those resources that are plausible `restartedAt` candidates (`Deployment` or `Rollout`, depending on the rendered release shape)

This summary is intended to be the first-class diagnostic surface for sync-truth regressions.

## Correlation seam to keep proved

The runtime/release handoff boundary remains intentionally narrow:

- release-service publishes canonical workload identity labels (`app.kubernetes.io/name`, `devflow.io/release-id`, `devflow.application/id`, `devflow.environment/id`)
- runtime-service observes those labels from Kubernetes and uses them to associate rollout state with the release writeback callback path
- annotations such as `kubectl.kubernetes.io/restartedAt` remain supplementary runtime state, not ownership or release identity

The regression checks that keep this true are:

- `go test ./internal/runtime/transport/http ./internal/runtime/observer ./internal/release/transport/http ./internal/release/service -run 'TestDeleteRuntimePodReturnsAcknowledgement|TestRolloutRuntimeReturnsAcknowledgement|TestWriteReleaseStepsRollingObserverSkipsReleaseOwnedHandoffStep|TestHandleArgoEventUpdatesReleaseStatus|TestReleaseStatusConvergenceRequiresReleaseOwnedStartDeploymentBeforeClosingRelease'`
- `bash scripts/verify-metadata-audit.sh`

Those proofs matter because S03 is only valid if Argo drift repair does **not** weaken the release/runtime ownership split.

## Live inspection workflow

### 1. Confirm the repo contract still matches the intended seam

Run the focused verifier:

```sh
bash scripts/verify-metadata-audit.sh
```

What this proves locally:

- canonical release/application/environment labels are still release-owned contract
- drift-prone runtime annotations are filtered from rendered desired state
- Argo `ignoreDifferences` still covers `kubectl.kubernetes.io/restartedAt`
- the ignore target remains workload-kind-aware (`Deployment` for rolling releases, `Rollout` for blue-green/canary)
- runtime rollout writeback still derives release/application/environment identity from workload labels
- runtime restart writeback and runtime observer parsing still point at the same annotation path
- this document is still linked from the broader audit docs

### 2. Inspect the live Argo `Application`

When cluster access is available, capture the live application summary and resource table:

```sh
kubectl get application -n argocd meta-service -o yaml > /tmp/meta-service-application.yaml
kubectl get application -n argocd meta-service -o jsonpath='{.status.sync.status}{"\n"}{.status.health.status}{"\n"}{.status.operationState.phase}{"\n"}{.status.operationState.message}{"\n"}'
kubectl get application -n argocd meta-service -o jsonpath='{range .spec.ignoreDifferences[*]}{.group}{"\t"}{.kind}{"\t"}{.name}{"\t"}{.namespace}{"\t"}{range .jsonPointers[*]}{.}{","}{end}{"\n"}{end}'
kubectl get application -n argocd meta-service -o jsonpath='{range .status.resources[*]}{.group}{"\t"}{.kind}{"\t"}{.namespace}{"\t"}{.name}{"\t"}{.status}{"\t"}{.health.status}{"\t"}{.requiresPruning}{"\n"}{end}'
```

If `kubectl` jsonpath output looks suspiciously blank for `jsonPointers`, confirm against the raw YAML instead of assuming the pointer is absent:

```sh
rg -n "ignoreDifferences|jsonPointers|restartedAt|kind: Deployment|kind: Rollout|name: meta-service" /tmp/meta-service-application.yaml
```

If `argocd` CLI access is configured, also capture the controller’s own comparison view:

```sh
argocd app get meta-service --grpc-web
argocd app diff meta-service --grpc-web
```

### 3. Compare the live status against the repo-owned expectations

Use the following interpretation table.

| Live signal | What it means | Likely next step |
|---|---|---|
| `status.sync.status=Synced` | The original drift is no longer present. | Stop changing ignore rules; preserve this doc as historical proof only. |
| `OutOfSync` and the only `status.resources[]` offender is the primary rollout workload (`apps/Deployment` for rolling releases, `argoproj.io/Rollout` for blue-green/canary) | The remaining drift may still be the restart annotation path. | Confirm the live `Application.spec.ignoreDifferences` still includes the restartedAt pointer and inspect Argo diff output for that workload specifically. |
| `OutOfSync` but the offender is not the primary rollout workload | The restart-annotation hypothesis is incomplete or wrong. | Inspect the reported kind/object before changing ignore rules. |
| `OutOfSync` on the primary rollout workload, but live `spec.ignoreDifferences` does **not** include `/spec/template/metadata/annotations/kubectl.kubernetes.io~1restartedAt` for that workload kind | The live `Application` is stale, hand-edited, or no longer matches rendered desired state. | Repair the Argo application handoff/update path before adding broader ignores. |
| `OutOfSync` on the primary rollout workload, ignore rule is present, but Argo diff shows another path/object | The restart annotation is not the active drift root cause. | Narrow the fix to the new diff path instead of widening ignores. |
| `requiresPruning=true` on an out-of-sync resource | The drift is about ownership/target-set mismatch, not annotation mutation. | Investigate desired/live resource targeting and pruning expectations. |

## Expected diff signals

The evidence you want from the live system is one of these concrete outcomes:

### Expected signal A: restart-annotation-only drift

You should see:

- `status.resources[]` includes the primary rollout workload as `OutOfSync`
- `spec.ignoreDifferences[]` includes the matching workload kind plus `/spec/template/metadata/annotations/kubectl.kubernetes.io~1restartedAt`
- Argo diff output for that workload references only the pod-template `restartedAt` annotation

If all three are true, the next executor should investigate **why Argo is still surfacing the diff despite the matching ignore pointer**.

### Expected signal B: workload-kind mismatch

You should see:

- `status.resources[]` marks an object kind other than the active release workload kind as `OutOfSync`
- or the mutated object is a rollout/custom workload while the ignore rule only targets another kind

If true, the fix is not to broaden all metadata ignores blindly; it is to align the ignore target to the actual workload kind or to move the runtime mutation off that object.

### Expected signal C: adjacent object or field drift

You should see:

- the primary rollout workload is present, but the diff points at a different JSON path
- or another object in `status.resources[]` is also `OutOfSync`
- or `requiresPruning=true` indicates desired/live set mismatch rather than metadata mutation

If true, treat the restart-annotation path as background context, not root cause.

## Open question

The still-unresolved live question is:

> If the live `meta-service` drift really reduces to `kubectl.kubernetes.io/restartedAt` on the rendered primary workload, why does Argo still report `OutOfSync` when the rendered `Application` already includes that exact ignore pointer for the active workload kind?

This document intentionally does not guess past the available evidence. Use the live inspection workflow above to decide whether the remaining problem is:

- Argo normalization not matching the expected pointer behavior
- a workload-kind mismatch
- an adjacent field on the same object
- or a different out-of-sync resource entirely

## Known runtime mutation path

This is still the relevant runtime-side write path:

- `internal/runtime/service/service.go` → `(*k8sExecutor).RestartDeployment`

It patches:

```go
patch := []byte(fmt.Sprintf(
    `{"spec":{"template":{"metadata":{"annotations":{"kubectl.kubernetes.io/restartedAt":"%s"}}}}}`,
    time.Now().Format(time.RFC3339),
))
```

That path remains important because it explains why the restart annotation can appear in the live workload even when the release-rendered desired state intentionally filters it out.

## Known runtime/release correlation path

This is still the relevant runtime-side correlation seam:

- `internal/runtime/observer/release_rollout.go` → `deriveReleaseRolloutContext`

It resolves writeback identity from workload labels first:

```go
releaseID, err := uuid.Parse(strings.TrimSpace(workload.Labels[releasedomain.ReleaseIDLabel]))
applicationID, err := uuid.Parse(strings.TrimSpace(workload.Labels[releasedomain.ReleaseApplicationLabel]))
environmentID := strings.TrimSpace(workload.Labels[releasedomain.ReleaseEnvironmentLabel])
```

The environment label may fall back to `workload.Environment` when absent, but the release and application identifiers must still come from release-owned labels. That is the contract preserving release/runtime ownership boundaries after the Argo fix.

## Current release-side ignore contract

The release path still renders this ignore rule in `internal/release/service/release.go`:

```go
func releaseApplicationIgnoreDifferences(release *model.Release) appv1.IgnoreDifferences {
    return appv1.IgnoreDifferences{
        releaseWorkloadRestartedAtIgnoreDifference(release),
    }
}
```

The target kind is chosen by `releasePrimaryWorkloadIgnoreTarget(release)`:

- rolling releases → `apps` / `Deployment`
- blue-green and canary releases → `argoproj.io` / `Rollout`

The pointer remains:

```text
/spec/template/metadata/annotations/kubectl.kubernetes.io~1restartedAt
```

That is the contract the live `Application` should be compared against.

## Evidence capture

If cluster access is available, paste the results into this section before changing code.
If cluster access is not available, leave the placeholders and record that the environment blocked live proof collection.

### Live command results

#### Argo drift-localization refresh

- `kubectl get application -n argocd meta-service -o jsonpath='{.status.sync.status}'`
  - result: `OutOfSync`
- `kubectl get application -n argocd meta-service -o jsonpath='{.status.health.status}{"\n"}{.status.operationState.phase}{"\n"}{.status.operationState.message}{"\n"}'`
  - result:
    - `Healthy`
    - `Succeeded`
    - `successfully synced (all tasks run)`
- `kubectl get application -n argocd meta-service -o jsonpath='{range .spec.ignoreDifferences[*]}...{end}'`
  - result: `apps    Deployment            ,`
  - note: the terse jsonpath formatter did not surface the JSON pointer value cleanly in this environment.
- `rg -n "ignoreDifferences|jsonPointers|restartedAt|kind: Deployment|name: meta-service" /tmp/meta-service-application.yaml`
  - result: raw Application YAML confirms `jsonPointers:` includes `/spec/template/metadata/annotations/kubectl.kubernetes.io~1restartedAt` for the `apps/Deployment` target.
- `kubectl get application -n argocd meta-service -o jsonpath='{range .status.resources[*]}...{end}'`
  - result:
    - `ConfigMap devflow/meta-service Synced`
    - `Service devflow/meta-service Synced`
    - `ServiceAccount devflow/meta-service Synced`
    - `apps Deployment devflow/meta-service OutOfSync`
- `argocd app diff meta-service --grpc-web`
  - result: `UNAVAILABLE_IN_ENVIRONMENT`
  - note: local `argocd` CLI returned `Argo CD server address unspecified`.

#### Runtime acknowledgement -> observer -> writeback refresh

Representative runtime target used for the live walk:

- `application_id`: `999c0c88-1f1f-41d1-a67a-8159d07c878c`
- `environment_id`: `b780ca97-a213-4763-bfb9-43f7e3a11ee7`
- observed release label before/after action: `devflow.io/release-id=9dd5e401-89c3-4acc-891c-440f861a8045`
- shared ingress host: `https://devflow-pre-production.bei.com`

Inspection order used:

1. `GET /api/v1/runtime/workload`
2. `GET /api/v1/runtime/pods`
3. `POST /api/v1/runtime/rollouts`
4. re-read runtime workload/pods to confirm observer-facing progression
5. inspect live Kubernetes `Deployment` / pods for the mutated restart annotation plus release/application/environment labels
6. inspect `release-service` and `runtime-service` logs for callback/writeback activity
7. inspect `GET /api/v1/release/releases/{release_id}` to verify final release-step convergence instead of claiming success from the action response

Live results from that walk:

- `GET /api/v1/runtime/workload?application_id=999c0c88-1f1f-41d1-a67a-8159d07c878c&environment_id=b780ca97-a213-4763-bfb9-43f7e3a11ee7`
  - result before action: `200` with workload `Deployment/meta-service`, `summary_status=Healthy`, `ready_replicas=1`, and stable labels:
    - `app.kubernetes.io/name=meta-service`
    - `devflow.io/release-id=9dd5e401-89c3-4acc-891c-440f861a8045`
    - `devflow.application/id=999c0c88-1f1f-41d1-a67a-8159d07c878c`
    - `devflow.environment/id=b780ca97-a213-4763-bfb9-43f7e3a11ee7`
- `GET /api/v1/runtime/pods?...`
  - result before action: `200` with one running pod owned by `ReplicaSet/meta-service-88b649cbc` and the same release/application/environment labels.
- `POST /api/v1/runtime/rollouts`
  - request body:
    ```json
    {
      "application_id": "999c0c88-1f1f-41d1-a67a-8159d07c878c",
      "environment_id": "b780ca97-a213-4763-bfb9-43f7e3a11ee7",
      "operator": "gsd-auto"
    }
    ```
  - result: `200` acknowledgement with:
    - `operation_type=deployment_restart`
    - `target_kind=deployment`
    - `target_name=meta-service`
    - `mutation_state=accepted`
    - `convergence_state=pending_observation`
    - `accepted_at=2026-05-01T04:55:32.989625833Z`
- re-read `GET /api/v1/runtime/workload` after the acknowledgement
  - result: `observed_generation` advanced from `13` to `14`
  - result: `annotations.kubectl.kubernetes.io/restartedAt` advanced from `2026-04-29T11:57:06Z` to `2026-05-01T04:55:32Z`
  - result: workload remained `Healthy` with `ready=1/1`, `updated=1/1`, `available=1/1`
- re-read `GET /api/v1/runtime/pods` after the acknowledgement
  - result: pod identity rolled from `meta-service-88b649cbc-cmj9r` to `meta-service-8c9b9c857-hl4lq`
  - result: the new pod still carried the same release/application/environment labels, proving release correlation remained label-derived through the real restart path
- `kubectl get deploy -n devflow meta-service -o jsonpath='...'`
  - result: live Deployment generation `14`, observedGeneration `14`, restartedAt `2026-05-01T04:55:32Z`, updated/ready/available replicas `1/1/1`
- `kubectl get pods -n devflow -l app.kubernetes.io/name=meta-service -o jsonpath='...'`
  - result: the live pod `meta-service-8c9b9c857-hl4lq` carried:
    - `devflow.io/release-id=9dd5e401-89c3-4acc-891c-440f861a8045`
    - `devflow.application/id=999c0c88-1f1f-41d1-a67a-8159d07c878c`
    - `devflow.environment/id=b780ca97-a213-4763-bfb9-43f7e3a11ee7`
- `kubectl logs deploy/release-service -n devflow-pre-production --tail=200 | rg 'verify/release/steps|...'`
  - result: release-service accepted three callback/writeback `POST /api/v1/verify/release/steps` requests immediately after the runtime action (`04:55:41Z`, `04:55:56Z`, `04:55:56Z`)
- `kubectl logs deploy/runtime-service -n devflow-pre-production --tail=200 | rg 'rollout|...'`
  - result: runtime-service logs captured the external runtime action acknowledgement request; recent callback evidence was clearer from the release-service ingress logs than from runtime-service info logs in this environment
- `GET /api/v1/release/releases/9dd5e401-89c3-4acc-891c-440f861a8045`
  - result: top-level release `status` remained `Running`
  - result: callback-owned steps converged successfully:
    - `observe_rollout`: `Succeeded` — `deployment healthy (ready=1/1, updated=1/1, available=1/1)`
    - `finalize_release`: `Succeeded` — `release finalized after deployment became healthy`
  - result: release-owned handoff step did **not** converge terminally:
    - `start_deployment`: `Running` with progress `10` and message `deployment sync started`

### Interpreted outcome

- current best classification for Argo drift: `live deployment-only drift persists even though the Application still carries the narrow restartedAt ignore pointer`
- narrowed localization from the refreshed session: `the only live offender remains apps/Deployment devflow/meta-service, and Argo status does not currently advertise a pruning mismatch for that resource`
- proven ownership boundary: `runtime/release rollout correlation still depends on release-owned labels; no runtime-local release store was reintroduced to repair Argo sync`
- proven acknowledgement contract: `the shared-ingress runtime rollout action still acknowledges acceptance first and returns convergence_state=pending_observation rather than claiming rollout completion`
- proven convergence layering: `observer-facing runtime state and callback-owned release steps converged after the restart, but the top-level release remained Running because the release-owned start_deployment handoff step still governs terminal closure`
- remaining live question for the Argo seam: `does the Argo controller diff still reduce to restartedAt after normalization, or is another Deployment field/path keeping meta-service OutOfSync?`

## What this proves today

From tracked repo state plus live cluster evidence, this artifact now proves:

- the metadata identity contract is label-based and intentionally narrow
- the runtime restart annotation is supplementary runtime state, not release identity
- runtime rollout writeback still resolves release/application/environment context from workload labels
- the real shared-ingress runtime action path still returns acknowledgement-first `pending_observation` responses rather than premature success claims
- the same real runtime restart path still preserves release/application/environment correlation through workload and pod labels after pod replacement
- callback-owned writeback steps can reach `observe_rollout=Succeeded` and `finalize_release=Succeeded` on the observed release path without reopening release identity ownership on the runtime side
- late observer callbacks preserved finalized-release terminality at the step layer in the observed path, but top-level release closure still depends on the release-owned `start_deployment` handoff step remaining non-terminal here
- release-service already attempts to suppress restart-annotation-only drift at the Argo layer
- the release-side ignore rule is workload-kind-aware even though the current live `meta-service` evidence is deployment-shaped
- the live `meta-service` Application still reports a single `apps/Deployment` drift signal while carrying the restartedAt ignore pointer, so the remaining seam is now a real Argo/live-diff localization problem rather than a missing contract problem

## Related tracked context

- `docs/resources/metadata-contract-audit.md`
- `docs/resources/runtime-spec.md`
- `docs/services/runtime-service.md`
- `internal/runtime/service/service.go`
- `internal/runtime/observer/kubernetes_runtime.go`
- `internal/runtime/observer/release_rollout.go`
- `internal/release/service/release.go`
- `internal/release/transport/argo/client.go`
- `scripts/verify-metadata-audit.sh`
ntentionally narrow
- the runtime restart annotation is supplementary runtime state, not release identity
- runtime rollout writeback still resolves release/application/environment context from workload labels
- release-service already attempts to suppress restart-annotation-only drift at the Argo layer
- the release-side ignore rule is workload-kind-aware even though the current live `meta-service` evidence is deployment-shaped
- the live `meta-service` Application still reports a single `apps/Deployment` drift signal while carrying the restartedAt ignore pointer, so the remaining seam is now a real Argo/live-diff localization problem rather than a missing contract problem

## Related tracked context

- `docs/resources/metadata-contract-audit.md`
- `docs/resources/runtime-spec.md`
- `docs/services/runtime-service.md`
- `internal/runtime/service/service.go`
- `internal/runtime/observer/kubernetes_runtime.go`
- `internal/runtime/observer/release_rollout.go`
- `internal/release/service/release.go`
- `internal/release/transport/argo/client.go`
- `scripts/verify-metadata-audit.sh`
