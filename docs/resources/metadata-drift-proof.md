# Metadata drift proof: `meta-service` restartedAt OutOfSync

This document captures the current tracked proof package for the live `meta-service` Argo CD drift finding that motivated the downstream sync-fix slices.

It exists so later executors can start from a repo-tracked evidence bundle instead of planner-only context or a hand-waved guess about which metadata surface is drifting.

## Live finding summary

The current live finding is:

- **Application under investigation:** `meta-service`
- **Out-of-sync resource:** the live Kubernetes `Deployment` rendered for `meta-service`
- **Drifting field:** `spec.template.metadata.annotations["kubectl.kubernetes.io/restartedAt"]`
- **Write path in repo code:** `internal/runtime/service/service.go` → `(*k8sExecutor).RestartDeployment`

This is the runtime-side patch path:

```go
patch := []byte(fmt.Sprintf(
    `{"spec":{"template":{"metadata":{"annotations":{"kubectl.kubernetes.io/restartedAt":"%s"}}}}}`,
    time.Now().Format(time.RFC3339),
))
_, err := k.clientset.AppsV1().Deployments(namespace).Patch(
    ctx,
    name,
    types.StrategicMergePatchType,
    patch,
    metav1.PatchOptions{},
)
```

That code mutates the live `Deployment` pod-template annotation in-cluster after release rendering has already produced the desired object for Argo CD.

## Why this field matters

`kubectl.kubernetes.io/restartedAt` is not part of release/application/environment identity.

However, it is still observed and persisted by the runtime observer:

- `internal/runtime/observer/kubernetes_runtime.go` reads `deployment.Spec.Template.Annotations`
- `parseRestartAt` extracts `annotations["kubectl.kubernetes.io/restartedAt"]`
- the observer stores that timestamp in runtime observed workload state as `RestartAt`

That means the restart annotation is a real runtime signal, but it is also a candidate Argo drift source because the runtime action path writes it onto live workload state outside release rendering.

## Release-side desired-state counterpoint

The release path already tries to suppress this exact drift signal at the Argo `Application` layer.

`internal/release/service/release.go` configures:

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

So the repo-tracked state today is already internally consistent on two points:

1. runtime-service really does patch `kubectl.kubernetes.io/restartedAt` on the live `Deployment`
2. release-service already tells Argo CD to ignore drift at that JSON pointer for `Deployment`

## What this proves

This proof package is meant to retire the wrong hypotheses before remediation work starts.

It proves that:

- the live drift is **not** primarily about missing release/application/environment labels
- the drift source is **not** the Argo `Application` metadata handoff surface itself
- the known runtime mutation path is the deployment restart patch in `internal/runtime/service/service.go`
- the observed drift candidate is the pod-template annotation path `kubectl.kubernetes.io/restartedAt`

## Open question

The remaining open question is:

> Why does `meta-service` still show OutOfSync if the current Argo `ignoreDifferences` rule already lists `/spec/template/metadata/annotations/kubectl.kubernetes.io~1restartedAt` for `Deployment`?

This task does **not** answer that question conclusively from repo-tracked code alone.

Plausible next-slice investigation directions include:

- the live drifting resource is not the `apps/Deployment` object shape the ignore rule targets
- the live diff is attached to an adjacent field or adjacent object, not only this exact JSON pointer
- the generated desired object, live object, or Argo normalization path differs from the repo assumption in a way this static audit cannot prove
- another controller or mutation path is contributing additional drift beyond `kubectl.kubernetes.io/restartedAt`

Downstream sync-fix work should treat that question as unresolved until validated against the live Argo diff output.

## Repeatable verifier

Use the focused tracked verifier introduced for this slice:

```sh
bash scripts/verify-metadata-audit.sh
```

That script re-checks the repo-tracked seam package for:

- release label injection on rendered workloads
- Argo `ignoreDifferences` coverage for `kubectl.kubernetes.io/restartedAt`
- runtime restart patch ownership in `internal/runtime/service/service.go`
- runtime observer label consumption and restart-annotation parsing
- audit-doc links back to this proof package

## Related tracked context

- `docs/resources/metadata-contract-audit.md`
- `docs/resources/runtime-spec.md`
- `docs/services/runtime-service.md`
- `internal/runtime/service/service.go`
- `internal/runtime/observer/kubernetes_runtime.go`
- `internal/release/service/release.go`
