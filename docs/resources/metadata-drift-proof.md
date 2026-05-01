# Metadata drift proof: `meta-service` Argo sync seam

This document is the tracked proof workflow for the live `meta-service` Argo CD drift investigation.

It is intentionally code-adjacent: it ties the live Argo `Application` status surface to the repo-owned `ignoreDifferences` contract and to the runtime mutation path that can still produce drift-like symptoms. Future executors should start here instead of re-reading planner notes or assuming the problem is still only `kubectl.kubernetes.io/restartedAt`.

## Why this artifact exists

Before changing Argo ignore rules, we need a repeatable way to answer three questions against the real `meta-service` `Application`:

1. **What does Argo currently think is OutOfSync?**
2. **Does the live `Application` still target the same ignore-difference contract the repo renders?**
3. **Is the remaining drift on the known runtime restart annotation path, a workload-kind mismatch, or a different object/field entirely?**

This artifact gives a single workflow that answers those questions even when cluster access is unavailable in the current environment.

## Current repo-backed hypothesis set

The repo already proves all of the following:

- runtime-service writes `spec.template.metadata.annotations["kubectl.kubernetes.io/restartedAt"]` onto live `Deployment` objects via `internal/runtime/service/service.go`
- runtime observers still read that timestamp back as a real operational signal
- release-service already renders an Argo `ignoreDifferences` entry for `apps/Deployment` at `/spec/template/metadata/annotations/kubectl.kubernetes.io~1restartedAt`
- release/runtime identity correlation depends on canonical labels, not on drift-prone annotations

That means **"the ignore rule is missing" is no longer a valid default assumption**.

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

If `argocd` CLI access is available, also capture the controller’s own comparison view:

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

## Evidence capture template

If cluster access is available, paste the results into this section before changing code.
If cluster access is not available, leave the placeholders and record that the environment blocked live proof collection.

### Live command results

- `kubectl get application -n argocd meta-service -o jsonpath='{.status.sync.status}'`
  - result: `ENVIRONMENT_UNAVAILABLE`
- `kubectl get application -n argocd meta-service -o jsonpath='{range .spec.ignoreDifferences[*]}...{end}'`
  - result: `ENVIRONMENT_UNAVAILABLE`
- `kubectl get application -n argocd meta-service -o jsonpath='{range .status.resources[*]}...{end}'`
  - result: `ENVIRONMENT_UNAVAILABLE`
- `argocd app diff meta-service --grpc-web`
  - result: `ENVIRONMENT_UNAVAILABLE`

### Interpreted outcome

- current best classification: `repo-only proof available; live drift source not yet re-confirmed in this environment`
- next live question to answer: `does Argo report only apps/Deployment drift, and if so, does the diff still reduce to restartedAt after normalization?`

## What this proves today

From tracked repo state alone, this artifact now proves:

- the metadata identity contract is label-based and intentionally narrow
- the runtime restart annotation is supplementary runtime state, not release identity
- release-service already attempts to suppress restart-annotation-only drift at the Argo layer
- there is now a first-class Argo inspection seam and concrete live command workflow for proving whether the remaining drift is restart-annotation-only, a workload-kind mismatch, or an adjacent path/object

## Related tracked context

- `docs/resources/metadata-contract-audit.md`
- `docs/resources/runtime-spec.md`
- `docs/services/runtime-service.md`
- `internal/runtime/service/service.go`
- `internal/runtime/observer/kubernetes_runtime.go`
- `internal/release/service/release.go`
- `internal/release/transport/argo/client.go`
- `scripts/verify-metadata-audit.sh`
