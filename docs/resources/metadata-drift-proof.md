# Metadata drift proof: `meta-service` Argo sync seam

This document is the tracked proof workflow for the live `meta-service` Argo CD drift investigation.

It is intentionally code-adjacent: it ties the live Argo `Application` status surface to the repo-owned `ignoreDifferences` contract, the release-owned identity labels that correlate runtime rollouts back to releases, and the runtime mutation path that can still produce drift-like symptoms. Future executors should start here instead of re-reading planner notes or assuming the problem is still only `kubectl.kubernetes.io/restartedAt`.

## Why this artifact exists

Before changing Argo ignore rules, we need a repeatable way to answer four questions against the real `meta-service` `Application`:

1. **What does Argo currently think is OutOfSync?**
2. **Does the live `Application` still target the same ignore-difference contract the repo renders?**
3. **Does runtime/release correlation still come from the release-owned label contract rather than runtime-local ownership data?**
4. **Is the remaining drift on the known runtime restart annotation path, a workload-kind mismatch, or a different object/field entirely?**

This artifact gives a single workflow that answers those questions, while still being usable when cluster access is unavailable in the current environment.

## Current repo-backed hypothesis set

The repo already proves all of the following:

- runtime-service writes `spec.template.metadata.annotations["kubectl.kubernetes.io/restartedAt"]` onto live `Deployment` objects via `internal/runtime/service/service.go`
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
- the subset of those resources that are plausible `restartedAt` candidates (`Deployment` resources)

This summary is intended to be the first-class diagnostic surface for sync-truth regressions.

## Correlation seam to keep proved

The runtime/release handoff boundary remains intentionally narrow:

- release-service publishes canonical workload identity labels (`app.kubernetes.io/name`, `devflow.io/release-id`, `devflow.application/id`, `devflow.environment/id`)
- runtime-service observes those labels from Kubernetes and uses them to associate rollout state with the release writeback callback path
- annotations such as `kubectl.kubernetes.io/restartedAt` remain supplementary runtime state, not ownership or release identity

The regression checks that keep this true are:

- `go test ./internal/runtime/observer -run 'TestDeriveReleaseRolloutContext|TestReleaseOwnedSelector|TestWriteReleaseStepsRollingObserverSkipsReleaseOwnedHandoffStep'`
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
rg -n "ignoreDifferences|jsonPointers|restartedAt|kind: Deployment|name: meta-service" /tmp/meta-service-application.yaml
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
| `OutOfSync` and the only `status.resources[]` offender is `apps/Deployment meta-service` | The remaining drift may still be the restart annotation path. | Confirm the live `Application.spec.ignoreDifferences` still includes the restartedAt pointer and inspect Argo diff output for that deployment specifically. |
| `OutOfSync` but the offender is not a `Deployment` | The restart-annotation hypothesis is incomplete or wrong. | Inspect the reported kind/object before changing ignore rules. |
| `OutOfSync` on `Deployment`, but live `spec.ignoreDifferences` does **not** include `/spec/template/metadata/annotations/kubectl.kubernetes.io~1restartedAt` | The live `Application` is stale, hand-edited, or no longer matches rendered desired state. | Repair the Argo application handoff/update path before adding broader ignores. |
| `OutOfSync` on `Deployment`, ignore rule is present, but Argo diff shows another path/object | The restart annotation is not the active drift root cause. | Narrow the fix to the new diff path instead of widening ignores. |
| `requiresPruning=true` on an out-of-sync resource | The drift is about ownership/target-set mismatch, not annotation mutation. | Investigate desired/live resource targeting and pruning expectations. |

## Expected diff signals

The evidence you want from the live system is one of these concrete outcomes:

### Expected signal A: restart-annotation-only drift

You should see:

- `status.resources[]` includes `apps / Deployment / meta-service / OutOfSync`
- `spec.ignoreDifferences[]` includes `apps / Deployment / /spec/template/metadata/annotations/kubectl.kubernetes.io~1restartedAt`
- Argo diff output for that deployment references only the pod-template `restartedAt` annotation

If all three are true, the next executor should investigate **why Argo is still surfacing the diff despite the matching ignore pointer**.

### Expected signal B: workload-kind mismatch

You should see:

- `status.resources[]` marks an object kind other than `Deployment` as `OutOfSync`
- or the mutated object is a rollout/custom workload while the ignore rule only targets `apps/Deployment`

If true, the fix is not to broaden all metadata ignores blindly; it is to align the ignore target to the actual workload kind or to move the runtime mutation off that object.

### Expected signal C: adjacent object or field drift

You should see:

- the deployment is present, but the diff points at a different JSON path
- or another object in `status.resources[]` is also `OutOfSync`
- or `requiresPruning=true` indicates desired/live set mismatch rather than metadata mutation

If true, treat the restart-annotation path as background context, not root cause.

## Open question

The still-unresolved live question is:

> If the live `meta-service` drift really reduces to `kubectl.kubernetes.io/restartedAt` on an `apps/Deployment`, why does Argo still report `OutOfSync` when the rendered `Application` already includes that exact ignore pointer?

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
func releaseApplicationIgnoreDifferences() appv1.IgnoreDifferences {
    return appv1.IgnoreDifferences{
        {
            Group: "apps",
            Kind:  "Deployment",
            JSONPointers: []string{
                "/spec/template/metadata/annotations/kubectl.kubernetes.io~1restartedAt",
            },
        },
    }
}
```

That is the contract the live `Application` should be compared against.

## Evidence capture

If cluster access is available, paste the results into this section before changing code.
If cluster access is not available, leave the placeholders and record that the environment blocked live proof collection.

### Live command results

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

### Interpreted outcome

- current best classification: `live deployment-only drift persists even though the Application still carries the narrow restartedAt ignore pointer`
- proven ownership boundary: `runtime/release rollout correlation still depends on release-owned labels; no runtime-local release store was reintroduced to repair Argo sync`
- next live question to answer: `does the Argo controller diff still reduce to restartedAt after normalization, or is another Deployment field/path keeping meta-service OutOfSync?`

## What this proves today

From tracked repo state plus live cluster evidence, this artifact now proves:

- the metadata identity contract is label-based and intentionally narrow
- the runtime restart annotation is supplementary runtime state, not release identity
- runtime rollout writeback still resolves release/application/environment context from workload labels
- release-service already attempts to suppress restart-annotation-only drift at the Argo layer
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
