# Runtime Release Kubernetes Truth Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the remaining PostgreSQL dependency from the `runtime-service` release-runtime path, make runtime release observation depend only on Kubernetes workload labels plus live state, and let runtime-service converge workload `devflow.io/release-status` from `running` to terminal values.

**Architecture:** `release-service` becomes responsible for projecting release activity onto workload metadata via `devflow.io/release-status=running`. `runtime-service` then filters informer bootstrap and event handling using `devflow.control-plane/id` and `devflow.io/release-status`, reconstructs release identity from workload labels, writes runtime observation back without reading release storage, and updates the live workload label to terminal values when rollout observation reaches a terminal outcome.

**Tech Stack:** Go, Kubernetes client-go informers, Argo Rollouts unstructured workloads, existing runtime memory store, Go test

---

## File map

- Modify: `internal/release/domain/types.go`
  - add the canonical release-status label constant next to the existing release identity labels
- Modify: `internal/release/service/release_bundle.go`
  - inject `devflow.io/release-status=running` into rendered workload metadata and pod-template metadata
- Modify: `internal/release/service/release_bundle_test.go`
  - lock the workload label projection contract with focused assertions
- Modify: `internal/runtime/watch/release_event_source.go`
  - filter runtime release workload events by control-plane and release-status labels
- Modify: `internal/runtime/watch/release_event_source_test.go`
  - verify matching workloads enqueue and non-running workloads are skipped
- Modify: `internal/runtime/bootstrap/release_runtime.go`
  - remove release PostgreSQL store construction and runtime-side release DB reads
- Modify: `internal/runtime/bootstrap/release_runtime_test.go`
  - replace the PG-backed release-source tests with workload-label-driven runtime tests
- Modify: `internal/runtime/reconcile/release_reconciler.go`
  - stop depending on release status fetched from storage; reconcile from workload labels and runtime state only
- Modify: `internal/runtime/reconcile/release_reconciler_test.go`
  - verify the reconciler only writes back for workloads labeled `release-status=running`
- Modify: `internal/runtime/observer/kubernetes_runtime.go`
  - expose a narrow workload label update path for runtime-side terminal release-status convergence
- Modify: `internal/runtime/observer/kubernetes_runtime_test.go`
  - verify runtime-side label convergence updates workload and pod-template metadata
- Modify: `internal/runtime/config/config.go`
  - remove the temporary PostgreSQL guard once runtime release bootstrap is PostgreSQL-free
- Modify: `internal/runtime/config/config_test.go`
  - update runtime startup expectations now that release runtime no longer needs PG initialization
- Modify: `docs/services/runtime-service.md`
  - document Kubernetes-only release-runtime truth
- Modify: `docs/system/runtime-storage-model.md`
  - document that release-runtime filtering uses workload labels, not PostgreSQL
- Modify: `docs/system/current-service-extraction-reality.md`
  - align current implementation reality with the new runtime contract

### Task 1: Project Running Release Status Into Workloads

**Files:**
- Modify: `internal/release/domain/types.go`
- Modify: `internal/release/service/release_bundle.go`
- Test: `internal/release/service/release_bundle_test.go`

- [ ] **Step 1: Write the failing test for the new workload label**

Add a focused assertion to `internal/release/service/release_bundle_test.go` inside the existing deployment/rollout label checks:

```go
if got := labels[model.ReleaseStatusLabel]; got != string(model.ReleaseRunning) {
	t.Fatalf("release status label = %v", got)
}
```

Place it alongside the existing required identity label assertions so both workload metadata and pod-template metadata must carry the new label.

- [ ] **Step 2: Run the targeted test to verify it fails**

Run:

```bash
go test ./internal/release/service -run 'TestRenderDeploymentBundle|TestRenderDeploymentBundleForCanaryRelease' -v
```

Expected:

- the targeted test fails because `model.ReleaseStatusLabel` does not exist yet or the rendered labels do not include it

- [ ] **Step 3: Add the release-status label constant**

Update `internal/release/domain/types.go` near the existing label constants:

```go
const (
	ReleaseIDLabel          = "devflow.io/release-id"
	ReleaseApplicationLabel = "devflow.application/id"
	ReleaseEnvironmentLabel = "devflow.environment/id"
	ReleaseStatusLabel      = "devflow.io/release-status"
	ControlPlaneLabel       = "devflow.control-plane/id"
)
```

- [ ] **Step 4: Write the minimal workload label projection**

Update `internal/release/service/release_bundle.go` inside `releaseWorkloadLabels(...)`:

```go
requiredLabels := map[string]any{
	"app.kubernetes.io/name":      selectorName,
	model.ReleaseIDLabel:          release.ID.String(),
	model.ReleaseApplicationLabel: release.ApplicationID.String(),
	model.ReleaseEnvironmentLabel: strings.TrimSpace(release.EnvironmentID),
	model.ReleaseStatusLabel:      string(release.Status),
	observer.ObserveStateLabel:    observer.ObserveStateRunning,
}
```

Keep the existing “required labels override incoming workload labels” behavior so user-provided labels cannot replace the projected release status.

- [ ] **Step 5: Run the targeted test to verify it passes**

Run:

```bash
go test ./internal/release/service -run 'TestRenderDeploymentBundle|TestRenderDeploymentBundleForCanaryRelease' -v
```

Expected:

- PASS
- workload metadata and pod-template metadata now both contain `devflow.io/release-status=running`

- [ ] **Step 6: Commit the slice**

```bash
git add internal/release/domain/types.go internal/release/service/release_bundle.go internal/release/service/release_bundle_test.go
git commit -m "feat: project release running status onto workloads"
```

### Task 2: Remove Runtime Release PostgreSQL Reads

**Files:**
- Modify: `internal/runtime/watch/release_event_source.go`
- Modify: `internal/runtime/watch/release_event_source_test.go`
- Modify: `internal/runtime/bootstrap/release_runtime.go`
- Modify: `internal/runtime/bootstrap/release_runtime_test.go`
- Modify: `internal/runtime/reconcile/release_reconciler.go`
- Modify: `internal/runtime/reconcile/release_reconciler_test.go`
- Modify: `internal/runtime/config/config.go`
- Modify: `internal/runtime/config/config_test.go`

- [ ] **Step 1: Write the failing event-source test for release-status filtering**

Add a focused test to `internal/runtime/watch/release_event_source_test.go`:

```go
func TestReleaseEventSourceSkipsNonRunningReleaseStatus(t *testing.T) {
	queue := &stubReleaseQueue{}
	source := NewReleaseEventSource(nil, queue, "cp-1")

	source.handleObject(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				releasedomain.ReleaseIDLabel:    "11111111-1111-1111-1111-111111111111",
				releasedomain.ControlPlaneLabel: "cp-1",
				releasedomain.ReleaseStatusLabel:"succeeded",
			},
		},
	})

	if len(queue.items) != 0 {
		t.Fatalf("queue items = %#v, want empty", queue.items)
	}
}
```

- [ ] **Step 2: Run the watch test to verify it fails**

Run:

```bash
go test ./internal/runtime/watch -run 'TestReleaseEventSource' -v
```

Expected:

- FAIL because the current event source enqueues any workload with a release ID regardless of `release-status`

- [ ] **Step 3: Implement the event-source filter**

Update `internal/runtime/watch/release_event_source.go`:

```go
if strings.TrimSpace(labels[releasedomain.ReleaseStatusLabel]) != string(releasedomain.ReleaseRunning) {
	return
}
```

Keep the existing control-plane filter and place the new status check before `s.queue.Add(releaseID)`.

- [ ] **Step 4: Replace the bootstrap release-store path with workload-label truth**

Update `internal/runtime/bootstrap/release_runtime.go`:

- delete the `releaserepo` import
- delete `releaseStore := releaserepo.NewPostgresStore()`
- delete `runtimeStoreRunningReleaseSource`
- delete `releaseStateSourceWithRuntimeStore`’s dependency on `releaserepo.Store`
- make `defaultReleaseRuntimeBootstrapDeps(...)` always use `watch.NewReleaseEventSource(...)` when workload cache is available
- when workload cache is unavailable, use a runtime-store-backed fallback that derives candidates only from observed workloads carrying:
  - `releasedomain.ReleaseIDLabel`
  - `releasedomain.ControlPlaneLabel`
  - `releasedomain.ReleaseStatusLabel == string(releasedomain.ReleaseRunning)`

The fallback `ListRunningReleases(...)` implementation should shape items like:

```go
items = append(items, &watch.RunningRelease{
	ReleaseID:      releaseID,
	ControlPlaneID: strings.TrimSpace(workload.Labels[releasedomain.ControlPlaneLabel]),
	Status:         strings.TrimSpace(workload.Labels[releasedomain.ReleaseStatusLabel]),
})
```

- [ ] **Step 5: Rewrite the release-state lookup to use workload labels only**

Update `newReleaseStateSource(...)` and `GetRunningRelease(...)` in `internal/runtime/bootstrap/release_runtime.go` so they return runtime reconcile input directly from observed workload labels:

```go
return &reconcile.RunningRelease{
	ReleaseID:      releaseID,
	ApplicationID:  spec.ApplicationID,
	EnvironmentID:  strings.TrimSpace(spec.Environment),
	ControlPlaneID: strings.TrimSpace(workload.Labels[releasedomain.ControlPlaneLabel]),
	Status:         strings.TrimSpace(workload.Labels[releasedomain.ReleaseStatusLabel]),
}, nil
```

If the workload is missing `release-status=running`, return `nil, nil` rather than guessing.

- [ ] **Step 6: Update the reconciler guard to trust workload-projected status**

Keep the runtime-side status gate in `internal/runtime/reconcile/release_reconciler.go`:

```go
if strings.ToLower(strings.TrimSpace(release.Status)) != "running" {
	return nil
}
```

But make sure `release.Status` now comes from workload labels, not the release repository.

- [ ] **Step 7: Replace the temporary PostgreSQL startup guard**

Update `internal/runtime/config/config.go` by deleting:

```go
if !platformdb.IsInitialized() {
	return nil
}
```

The release runtime should start without PG once the bootstrap path is clean.

- [ ] **Step 8: Add and update the runtime tests**

Update `internal/runtime/bootstrap/release_runtime_test.go`:

- replace `stubReleaseStore`-based tests with runtime observed workload label fixtures
- add one test that includes:

```go
Labels: map[string]string{
	releasedomain.ReleaseIDLabel:     releaseID.String(),
	releasedomain.ControlPlaneLabel:  "cp-1",
	releasedomain.ReleaseStatusLabel: string(releasedomain.ReleaseRunning),
}
```

- add one test with:

```go
Labels: map[string]string{
	releasedomain.ReleaseIDLabel:     releaseID.String(),
	releasedomain.ControlPlaneLabel:  "cp-1",
	releasedomain.ReleaseStatusLabel: string(releasedomain.ReleaseSucceeded),
}
```

and verify it does not produce a running release

Update `internal/runtime/config/config_test.go`:

- remove the fake PostgreSQL initialization from `TestInitRuntimeStartsReleaseRuntimeReconcilerWhenEnabled`
- invert `TestInitRuntimeSkipsReleaseRuntimeReconcilerWhenPostgresUnavailable` into a startup-success test without any PG setup

Suggested assertion:

```go
if !releaseRuntimeCalled {
	t.Fatal("expected release runtime reconciler to start without postgres")
}
```

- [ ] **Step 9: Run the runtime-focused verification**

Run:

```bash
go test ./internal/runtime/watch ./internal/runtime/bootstrap ./internal/runtime/reconcile ./internal/runtime/config -v
go build -o /tmp/runtime-service ./cmd/runtime-service
```

Expected:

- PASS
- `runtime-service` builds without any release PostgreSQL bootstrap requirement

- [ ] **Step 10: Commit the slice**

```bash
git add internal/runtime/watch/release_event_source.go internal/runtime/watch/release_event_source_test.go internal/runtime/bootstrap/release_runtime.go internal/runtime/bootstrap/release_runtime_test.go internal/runtime/reconcile/release_reconciler.go internal/runtime/reconcile/release_reconciler_test.go internal/runtime/config/config.go internal/runtime/config/config_test.go
git commit -m "refactor: remove runtime release postgres dependency"
```

### Task 3: Converge Workload Release Status To Terminal Values

**Files:**
- Modify: `internal/runtime/reconcile/release_reconciler.go`
- Modify: `internal/runtime/reconcile/release_reconciler_test.go`
- Modify: `internal/runtime/observer/kubernetes_runtime.go`
- Modify: `internal/runtime/observer/kubernetes_runtime_test.go`
- Modify: `internal/runtime/bootstrap/release_runtime.go`

- [ ] **Step 1: Write the failing terminal label convergence test**

Add a focused test to `internal/runtime/reconcile/release_reconciler_test.go` that starts with a workload labeled `devflow.io/release-status=running` and expects a terminal label update when the workload becomes healthy:

```go
if got := updater.labels[releasedomain.ReleaseStatusLabel]; got != string(releasedomain.ReleaseSucceeded) {
	t.Fatalf("release status label = %q", got)
}
```

Use a healthy observed workload fixture:

```go
&runtimedomain.RuntimeObservedWorkload{
	DesiredReplicas:     3,
	ReadyReplicas:       3,
	UnavailableReplicas: 0,
	Labels: map[string]string{
		releasedomain.ReleaseIDLabel:     releaseID.String(),
		releasedomain.ControlPlaneLabel:  "cp-1",
		releasedomain.ReleaseStatusLabel: string(releasedomain.ReleaseRunning),
	},
}
```

- [ ] **Step 2: Run the targeted reconcile test to verify it fails**

Run:

```bash
go test ./internal/runtime/reconcile -run 'TestReleaseReconciler' -v
```

Expected:

- FAIL because the reconciler does not yet update workload release-status labels

- [ ] **Step 3: Add a narrow label updater dependency**

Extend `internal/runtime/reconcile/release_reconciler.go`:

```go
type ReleaseStatusLabelUpdater interface {
	UpdateReleaseStatusLabel(ctx context.Context, workload *runtimedomain.RuntimeObservedWorkload, status releasedomain.ReleaseStatus) error
}
```

Add it to `ReleaseReconciler` and the constructor:

```go
type ReleaseReconciler struct {
	releases       ReleaseStateSource
	runtimeStore   runtimerepo.Store
	stepsWriter    ReleaseStepsWriter
	labelUpdater   ReleaseStatusLabelUpdater
	controlPlaneID string
}
```

- [ ] **Step 4: Implement terminal label convergence in the reconciler**

In `internal/runtime/reconcile/release_reconciler.go`, after `deriveWritebackState(workload)`:

```go
if phase == releasedomain.StepSucceeded && r.labelUpdater != nil {
	if err := r.labelUpdater.UpdateReleaseStatusLabel(ctx, workload, releasedomain.ReleaseSucceeded); err != nil {
		return err
	}
}
```

Do not change non-terminal behavior. This slice only requires runtime to converge the label away from `running` when observation is terminal.

- [ ] **Step 5: Implement the workload label updater**

Add a minimal runtime-owned updater in `internal/runtime/observer/kubernetes_runtime.go` that:

- resolves the live workload object from `RuntimeObservedWorkload`
- updates `metadata.labels[devflow.io/release-status]`
- updates `spec.template.metadata.labels[devflow.io/release-status]` when a pod template exists

Follow the existing unstructured label update pattern already used in release code:

```go
labels := obj.GetLabels()
if labels == nil {
	labels = map[string]string{}
}
labels[releasedomain.ReleaseStatusLabel] = string(status)
obj.SetLabels(labels)
```

And for rollout/deployment templates:

```go
templateLabels, _, _ := unstructured.NestedStringMap(obj.Object, "spec", "template", "metadata", "labels")
if templateLabels == nil {
	templateLabels = map[string]string{}
}
templateLabels[releasedomain.ReleaseStatusLabel] = string(status)
_ = unstructured.SetNestedStringMap(obj.Object, templateLabels, "spec", "template", "metadata", "labels")
```

- [ ] **Step 6: Wire the updater into release runtime bootstrap**

Update `internal/runtime/bootstrap/release_runtime.go` so `reconcile.NewReleaseReconciler(...)` receives the concrete runtime workload label updater.

- [ ] **Step 7: Add the observer-level update tests**

Add focused tests to `internal/runtime/observer/kubernetes_runtime_test.go` covering:

- one `Deployment`
- one rollout-like `unstructured.Unstructured`

Both should begin with:

```go
releasedomain.ReleaseStatusLabel: string(releasedomain.ReleaseRunning)
```

and end with:

```go
releasedomain.ReleaseStatusLabel: string(releasedomain.ReleaseSucceeded)
```

on both workload metadata and pod-template metadata.

- [ ] **Step 8: Run the runtime terminal convergence verification**

Run:

```bash
go test ./internal/runtime/reconcile ./internal/runtime/observer ./internal/runtime/bootstrap -v
```

Expected:

- PASS
- runtime can converge workload release-status labels to terminal values without using PostgreSQL

- [ ] **Step 9: Commit the slice**

```bash
git add internal/runtime/reconcile/release_reconciler.go internal/runtime/reconcile/release_reconciler_test.go internal/runtime/observer/kubernetes_runtime.go internal/runtime/observer/kubernetes_runtime_test.go internal/runtime/bootstrap/release_runtime.go
git commit -m "feat: converge runtime workload release status labels"
```

### Task 4: Align Documentation With Kubernetes-Only Runtime Truth

**Files:**
- Modify: `docs/services/runtime-service.md`
- Modify: `docs/system/runtime-storage-model.md`
- Modify: `docs/system/current-service-extraction-reality.md`

- [ ] **Step 1: Write the failing documentation assertions**

Before editing docs, confirm the old wording still exists:

```bash
rg -n "release rollout observation is also started|release rollout observer startup is active|PostgreSQL-free" docs/services/runtime-service.md docs/system/runtime-storage-model.md docs/system/current-service-extraction-reality.md
```

Expected:

- existing lines still mention PostgreSQL-free runtime storage
- docs do not yet mention `devflow.io/release-status`

- [ ] **Step 2: Update runtime-service docs**

Add language to `docs/services/runtime-service.md` that explicitly states:

```text
runtime-service release-runtime observation trusts Kubernetes workload labels and live state only.
The workload metadata contract includes devflow.io/release-status=running alongside release_id, application_id, environment_id, and control_plane_id.
release-service writes the initial running projection and runtime-service converges it to terminal values from runtime observation.
runtime-service does not query PostgreSQL or release-service HTTP to confirm running release state.
```

- [ ] **Step 3: Update runtime storage model docs**

Add language to `docs/system/runtime-storage-model.md` that says:

```text
The release-runtime queue path filters observed workloads by devflow.control-plane/id and devflow.io/release-status=running.
Release-runtime candidate selection is rebuilt from workload labels and runtime observed state rather than release PostgreSQL reads.
When rollout observation becomes terminal, runtime-service updates devflow.io/release-status on the live workload object.
```

- [ ] **Step 4: Update current implementation reality**

Update `docs/system/current-service-extraction-reality.md` so the runtime-service row and follow-up notes say that:

```text
release-runtime observation is label-projected from Kubernetes workloads, remains PostgreSQL-free, and converges terminal workload release-status labels
```

- [ ] **Step 5: Run the documentation sanity check**

Run:

```bash
rg -n "devflow.io/release-status|release-service HTTP|PostgreSQL" docs/services/runtime-service.md docs/system/runtime-storage-model.md docs/system/current-service-extraction-reality.md
```

Expected:

- the three docs mention `devflow.io/release-status`
- none of them describe runtime release observation as reading release PostgreSQL truth

- [ ] **Step 6: Run the repo-local verification that still applies**

Run:

```bash
go test ./internal/release/service ./internal/runtime/watch ./internal/runtime/bootstrap ./internal/runtime/reconcile ./internal/runtime/observer ./internal/runtime/config -v
go build -o /tmp/release-service ./cmd/release-service
go build -o /tmp/runtime-service ./cmd/runtime-service
```

Expected:

- PASS
- both `release-service` and `runtime-service` still build

If `bash scripts/verify.sh` still fails on removed `deployments/` paths, record that as an unrelated pre-existing verifier drift rather than a failure of this slice.

- [ ] **Step 7: Commit the slice**

```bash
git add docs/services/runtime-service.md docs/system/runtime-storage-model.md docs/system/current-service-extraction-reality.md
git commit -m "docs: align runtime release kubernetes truth"
```

## Final verification

- [ ] **Step 1: Run the full targeted backend verification**

```bash
go test ./internal/release/service ./internal/runtime/watch ./internal/runtime/bootstrap ./internal/runtime/reconcile ./internal/runtime/observer ./internal/runtime/config -v
go build -o /tmp/release-service ./cmd/release-service
go build -o /tmp/runtime-service ./cmd/runtime-service
```

Expected:

- all targeted tests pass
- both binaries build successfully

- [ ] **Step 2: Inspect the final diff**

```bash
git diff --stat HEAD~3..HEAD
git status --short
```

Expected:

- only the planned release/runtime/doc files changed
- no accidental PostgreSQL dependency remains under `internal/runtime/**`
