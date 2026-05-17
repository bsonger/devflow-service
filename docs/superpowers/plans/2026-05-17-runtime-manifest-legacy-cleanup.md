# Runtime Manifest Legacy Cleanup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the last legacy Tekton manifest observer compatibility path so `runtime-service` uses only the manifest runtime reconciler contract.

**Architecture:** `internal/runtime/config` currently boots two manifest lanes: the old `StartTektonManifestObserver` polling path and the new `bootstrap.StartManifestRuntimeReconciler` path. This plan deletes the old lane, rewrites startup tests around the final single-lane contract, then syncs current docs and live staging/production config repos so code, runtime config, and operator docs all describe the same architecture.

**Tech Stack:** Go, client-go, Viper, runtime bootstrap/reconciler wiring, Markdown docs, YAML config

---

## File Structure

- Modify: `internal/runtime/config/config.go`
  Responsibility: remove `TektonManifestEnabled`, remove legacy startup hook variables and `startTektonManifestObserver(...)`, keep only the final manifest runtime startup contract.
- Modify: `internal/runtime/config/config_test.go`
  Responsibility: replace migration-bridge assertions with final-contract tests for manifest runtime enable/disable behavior.
- Modify: `docs/services/runtime-service.md`
  Responsibility: describe the current runtime-service manifest observation contract and remove legacy flag references.
- Modify: `docs/system/observability.md`
  Responsibility: align observer startup and operational notes with the single manifest runtime lane.
- Modify: `docs/system/runtime-storage-model.md`
  Responsibility: document that manifest runtime is Kubernetes-driven and no longer has a legacy Tekton observer compatibility path.
- Modify: `/Users/songbei/devflow-repo-config/production/runtime-service.yaml`
  Responsibility: remove the dead `observer.tekton_manifest_enabled` key from production runtime-service config.
- Modify: `/Users/songbei/devflow-repo-config/staging/runtime-service.yaml`
  Responsibility: remove the dead `observer.tekton_manifest_enabled` key from staging runtime-service config.

### Task 1: Remove Legacy Manifest Startup Wiring

**Files:**
- Modify: `internal/runtime/config/config.go`
- Test: `internal/runtime/config/config_test.go`

- [ ] **Step 1: Write the failing test for the final manifest runtime contract**

Add this test to `internal/runtime/config/config_test.go`:

```go
func TestInitRuntimeStartsManifestRuntimeReconcilerWhenEnabled(t *testing.T) {
	reset := installRuntimeConfigTestHooks()
	defer reset()

	enabled := true
	var manifestCfg bootstrap.ManifestRuntimeBootstrapConfig
	manifestCalled := false

	inClusterConfig = func() (*rest.Config, error) {
		return &rest.Config{Host: "https://cluster.example"}, nil
	}
	startKubernetesRuntimeObserverFn = func(_ context.Context, _ *rest.Config, _ runtimeobserver.KubernetesRuntimeObserverConfig) error {
		return nil
	}
	startReleaseRolloutObserverFn = func(_ context.Context, _ *rest.Config, _ runtimeobserver.ReleaseRolloutObserverConfig) error {
		return nil
	}
	startReleaseRuntimeReconcilerFn = func(_ context.Context, _ bootstrap.ReleaseRuntimeBootstrapConfig) error {
		return nil
	}
	startManifestRuntimeReconcilerFn = func(_ context.Context, cfg bootstrap.ManifestRuntimeBootstrapConfig) error {
		manifestCalled = true
		manifestCfg = cfg
		return nil
	}

	cfg := &Config{
		Observer: &ObserverConfig{
			SharedToken:            "observer-secret",
			ControlPlaneID:         "cp-1",
			TektonNamespace:        "tekton-observers",
			TektonPipeline:         "manifest-build",
			PollIntervalSeconds:    21,
			ManifestRuntimeEnabled: &enabled,
			ManifestRuntimeWorkers: 5,
		},
		Downstream: &DownstreamConfig{
			ReleaseServiceBaseURL: "http://release-service.devflow.svc.cluster.local",
		},
	}

	shutdown, err := InitRuntime(context.Background(), cfg, "runtime-service")
	if err != nil {
		t.Fatalf("InitRuntime returned error: %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown returned error: %v", err)
	}

	if !manifestCalled {
		t.Fatal("expected manifest runtime reconciler to start")
	}
	if !manifestCfg.Enabled {
		t.Fatal("expected manifest runtime enabled")
	}
	if manifestCfg.ControlPlaneID != "cp-1" {
		t.Fatalf("manifest runtime control plane = %q", manifestCfg.ControlPlaneID)
	}
	if manifestCfg.TektonNamespace != "tekton-observers" {
		t.Fatalf("manifest runtime tekton namespace = %q", manifestCfg.TektonNamespace)
	}
	if manifestCfg.TektonPipeline != "manifest-build" {
		t.Fatalf("manifest runtime tekton pipeline = %q", manifestCfg.TektonPipeline)
	}
	if manifestCfg.Workers != 5 {
		t.Fatalf("manifest runtime workers = %d", manifestCfg.Workers)
	}
	if manifestCfg.PollInterval != 21*time.Second {
		t.Fatalf("manifest runtime poll interval = %s", manifestCfg.PollInterval)
	}
	if manifestCfg.ReleaseServiceBaseURL != "http://release-service.devflow.svc.cluster.local" {
		t.Fatalf("manifest runtime release base = %q", manifestCfg.ReleaseServiceBaseURL)
	}
	if manifestCfg.ObserverToken != "observer-secret" {
		t.Fatalf("manifest runtime observer token = %q", manifestCfg.ObserverToken)
	}
}
```

- [ ] **Step 2: Run the targeted test and verify it fails because legacy assumptions still dominate the startup contract**

Run:

```bash
go test ./internal/runtime/config -run TestInitRuntimeStartsManifestRuntimeReconcilerWhenEnabled -count=1
```

Expected: FAIL because `config_test.go` still assumes the legacy Tekton manifest observer path exists and the new final-contract test is not yet wired into the cleaned startup path.

- [ ] **Step 3: Remove the legacy manifest config field and startup branch**

Update `internal/runtime/config/config.go` to this shape:

```go
type ObserverConfig struct {
	SharedToken            string `mapstructure:"shared_token" json:"shared_token" yaml:"shared_token"`
	ControlPlaneID         string `mapstructure:"control_plane_id" json:"control_plane_id" yaml:"control_plane_id"`
	TektonNamespace        string `mapstructure:"tekton_namespace" json:"tekton_namespace" yaml:"tekton_namespace"`
	TektonPipeline         string `mapstructure:"tekton_pipeline" json:"tekton_pipeline" yaml:"tekton_pipeline"`
	PollIntervalSeconds    int    `mapstructure:"poll_interval_seconds" json:"poll_interval_seconds" yaml:"poll_interval_seconds"`
	ManifestRuntimeEnabled *bool  `mapstructure:"manifest_runtime_enabled" json:"manifest_runtime_enabled" yaml:"manifest_runtime_enabled"`
	ManifestRuntimeWorkers int    `mapstructure:"manifest_runtime_workers" json:"manifest_runtime_workers" yaml:"manifest_runtime_workers"`
	ReleaseRuntimeEnabled  *bool  `mapstructure:"release_runtime_enabled" json:"release_runtime_enabled" yaml:"release_runtime_enabled"`
	ReleaseRuntimeWorkers  int    `mapstructure:"release_runtime_workers" json:"release_runtime_workers" yaml:"release_runtime_workers"`
}

var (
	initObservability = platformobservability.Init
	inClusterConfig   = rest.InClusterConfig

	startKubernetesRuntimeObserverFn = runtimeobserver.StartKubernetesRuntimeObserver
	startReleaseRolloutObserverFn    = runtimeobserver.StartReleaseRolloutObserver
	startManifestRuntimeReconcilerFn = bootstrap.StartManifestRuntimeReconciler
	startReleaseRuntimeReconcilerFn  = bootstrap.StartReleaseRuntimeReconciler
)

func InitRuntime(ctx context.Context, config *Config, serviceName string) (func(context.Context) error, error) {
	// ... existing observability bootstrap stays unchanged ...

	if err := startKubernetesRuntimeObserver(ctx, config); err != nil {
		return shutdown, err
	}
	if err := startReleaseRolloutObserver(ctx, config); err != nil {
		return shutdown, err
	}
	if err := startManifestRuntimeReconciler(ctx, config); err != nil {
		return shutdown, err
	}
	if err := startReleaseRuntimeReconciler(ctx, config); err != nil {
		return shutdown, err
	}
	return shutdown, nil
}
```

Delete `startTektonManifestObserver(...)` entirely from `internal/runtime/config/config.go`.

- [ ] **Step 4: Run the targeted test suite for runtime config startup**

Run:

```bash
go test ./internal/runtime/config -count=1
```

Expected: FAIL in old tests that still reference `TektonManifestEnabled` or `startTektonManifestObserverFn`.

- [ ] **Step 5: Commit the wiring cleanup**

```bash
git add internal/runtime/config/config.go internal/runtime/config/config_test.go
git commit -m "refactor: remove legacy manifest observer startup wiring"
```

### Task 2: Rewrite Runtime Startup Tests Around The Final Contract

**Files:**
- Modify: `internal/runtime/config/config_test.go`

- [ ] **Step 1: Replace the legacy default-startup test with a final-contract baseline**

Rewrite the old “starts all observers” test so it no longer references `TektonManifestObserverConfig` or `startTektonManifestObserverFn`:

```go
func TestInitRuntimeStartsBaselineObserversWhenClusterConfigAvailable(t *testing.T) {
	reset := installRuntimeConfigTestHooks()
	defer reset()

	var kubernetesCfg runtimeobserver.KubernetesRuntimeObserverConfig
	var rolloutCfg runtimeobserver.ReleaseRolloutObserverConfig
	var manifestRuntimeCfg bootstrap.ManifestRuntimeBootstrapConfig
	var releaseRuntimeCfg bootstrap.ReleaseRuntimeBootstrapConfig
	kubernetesCalled := false
	rolloutCalled := false
	manifestRuntimeCalled := false
	releaseRuntimeCalled := false

	inClusterConfig = func() (*rest.Config, error) {
		return &rest.Config{Host: "https://cluster.example"}, nil
	}
	startKubernetesRuntimeObserverFn = func(_ context.Context, cfg *rest.Config, observerCfg runtimeobserver.KubernetesRuntimeObserverConfig) error {
		kubernetesCalled = true
		if cfg == nil {
			t.Fatal("kubernetes observer received nil rest config")
		}
		kubernetesCfg = observerCfg
		return nil
	}
	startReleaseRolloutObserverFn = func(_ context.Context, cfg *rest.Config, observerCfg runtimeobserver.ReleaseRolloutObserverConfig) error {
		rolloutCalled = true
		if cfg == nil {
			t.Fatal("release rollout observer received nil rest config")
		}
		rolloutCfg = observerCfg
		return nil
	}
	startManifestRuntimeReconcilerFn = func(_ context.Context, cfg bootstrap.ManifestRuntimeBootstrapConfig) error {
		manifestRuntimeCalled = true
		manifestRuntimeCfg = cfg
		return nil
	}
	startReleaseRuntimeReconcilerFn = func(_ context.Context, cfg bootstrap.ReleaseRuntimeBootstrapConfig) error {
		releaseRuntimeCalled = true
		releaseRuntimeCfg = cfg
		return nil
	}

	cfg := &Config{
		Observer: &ObserverConfig{
			SharedToken:         "observer-secret",
			ControlPlaneID:      "cp-1",
			PollIntervalSeconds: 27,
		},
		Downstream: &DownstreamConfig{
			ReleaseServiceBaseURL: "http://release-service.devflow.svc.cluster.local",
		},
	}

	shutdown, err := InitRuntime(context.Background(), cfg, "runtime-service")
	if err != nil {
		t.Fatalf("InitRuntime returned error: %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown returned error: %v", err)
	}

	if !kubernetesCalled {
		t.Fatal("expected Kubernetes runtime observer to start")
	}
	if !rolloutCalled {
		t.Fatal("expected release rollout observer to start")
	}
	if manifestRuntimeCalled {
		t.Fatal("expected manifest runtime reconciler to stay disabled by default")
	}
	if releaseRuntimeCalled {
		t.Fatal("expected release runtime reconciler to stay disabled by default")
	}
	if !kubernetesCfg.Enabled {
		t.Fatal("expected kubernetes observer enabled")
	}
	if kubernetesCfg.PollInterval != 27*time.Second {
		t.Fatalf("kubernetes poll interval = %s", kubernetesCfg.PollInterval)
	}
	if !rolloutCfg.Enabled {
		t.Fatal("expected rollout observer enabled")
	}
	if rolloutCfg.ReleaseServiceBaseURL != "http://release-service.devflow.svc.cluster.local" {
		t.Fatalf("rollout release base = %q", rolloutCfg.ReleaseServiceBaseURL)
	}
	if manifestRuntimeCfg.Enabled {
		t.Fatal("expected manifest runtime config to remain zero-valued when disabled")
	}
	if releaseRuntimeCfg.Enabled {
		t.Fatal("expected release runtime config to remain zero-valued when disabled")
	}
}
```

- [ ] **Step 2: Delete legacy compatibility tests and add the final disabled-path test**

Remove tests equivalent to `TestInitRuntimeSkipsTektonObserverWhenDisabled` and `TestInitRuntimeSkipsLegacyTektonObserverWhenManifestRuntimeEnabled`. Add this replacement test:

```go
func TestInitRuntimeSkipsManifestRuntimeReconcilerWhenDisabled(t *testing.T) {
	reset := installRuntimeConfigTestHooks()
	defer reset()

	disabled := false
	manifestCalled := false

	inClusterConfig = func() (*rest.Config, error) {
		return &rest.Config{Host: "https://cluster.example"}, nil
	}
	startKubernetesRuntimeObserverFn = func(_ context.Context, _ *rest.Config, _ runtimeobserver.KubernetesRuntimeObserverConfig) error {
		return nil
	}
	startReleaseRolloutObserverFn = func(_ context.Context, _ *rest.Config, _ runtimeobserver.ReleaseRolloutObserverConfig) error {
		return nil
	}
	startReleaseRuntimeReconcilerFn = func(_ context.Context, _ bootstrap.ReleaseRuntimeBootstrapConfig) error {
		return nil
	}
	startManifestRuntimeReconcilerFn = func(_ context.Context, _ bootstrap.ManifestRuntimeBootstrapConfig) error {
		manifestCalled = true
		return nil
	}

	cfg := &Config{
		Observer: &ObserverConfig{
			ManifestRuntimeEnabled: &disabled,
			PollIntervalSeconds:    15,
		},
		Downstream: &DownstreamConfig{
			ReleaseServiceBaseURL: "http://release-service.devflow.svc.cluster.local",
		},
	}

	shutdown, err := InitRuntime(context.Background(), cfg, "runtime-service")
	if err != nil {
		t.Fatalf("InitRuntime returned error: %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown returned error: %v", err)
	}

	if manifestCalled {
		t.Fatal("expected manifest runtime reconciler to stay disabled")
	}
}
```

- [ ] **Step 3: Verify the rewritten startup tests pass**

Run:

```bash
go test ./internal/runtime/config -count=1
```

Expected: PASS with no references to `TektonManifestEnabled` or `startTektonManifestObserverFn`.

- [ ] **Step 4: Grep for legacy manifest startup references in active runtime code**

Run:

```bash
rg -n "TektonManifestEnabled|startTektonManifestObserverFn|startTektonManifestObserver" internal/runtime
```

Expected: no matches.

- [ ] **Step 5: Commit the test cleanup**

```bash
git add internal/runtime/config/config_test.go
git commit -m "test: align runtime manifest startup contract"
```

### Task 3: Sync Current Docs To The Final Manifest Runtime Contract

**Files:**
- Modify: `docs/services/runtime-service.md`
- Modify: `docs/system/observability.md`
- Modify: `docs/system/runtime-storage-model.md`

- [ ] **Step 1: Update `docs/services/runtime-service.md` to remove legacy flag language**

Add or replace the manifest runtime section with text shaped like:

```md
## Manifest Runtime

`runtime-service` no longer runs a legacy Tekton manifest polling fallback.

Current manifest behavior:

- `observer.manifest_runtime_enabled=true` starts the manifest runtime reconciler
- `observer.manifest_runtime_enabled=false` leaves the manifest runtime lane disabled
- manifest build observation and writeback flow through the reconciler-owned Kubernetes/Tekton runtime path

`observer.tekton_manifest_enabled` is no longer part of the runtime-service contract.
```

- [ ] **Step 2: Update `docs/system/observability.md` to describe the removed compatibility path**

Edit the runtime observer section to include language shaped like:

```md
`runtime-service` has no manifest polling compatibility lane anymore.
Manifest startup is controlled only by `observer.manifest_runtime_enabled`.
If that flag is disabled, the manifest runtime reconciler does not start.
If that flag is enabled, the reconciler owns manifest-side Tekton observation and release-service writeback.
```

- [ ] **Step 3: Update `docs/system/runtime-storage-model.md` to reflect the Kubernetes-only manifest truth**

Replace stale compatibility wording with text shaped like:

```md
Manifest runtime state is now derived through the runtime reconciler's live Kubernetes/Tekton view.
The old `tekton_manifest_enabled` compatibility switch and legacy Tekton manifest observer startup path have been removed.
```

- [ ] **Step 4: Verify active docs no longer mention the removed flag**

Run:

```bash
rg -n "tekton_manifest_enabled|legacy Tekton manifest observer|manifest polling fallback" docs/services docs/system
```

Expected: no matches in current runtime-service / system truth docs except historical archive-like material outside the edited files.

- [ ] **Step 5: Commit the doc sync**

```bash
git add docs/services/runtime-service.md docs/system/observability.md docs/system/runtime-storage-model.md
git commit -m "docs: remove manifest legacy runtime compatibility"
```

### Task 4: Remove Dead Runtime Config From Live Config Repos

**Files:**
- Modify: `/Users/songbei/devflow-repo-config/production/runtime-service.yaml`
- Modify: `/Users/songbei/devflow-repo-config/staging/runtime-service.yaml`

- [ ] **Step 1: Remove the dead key from production config**

Edit `/Users/songbei/devflow-repo-config/production/runtime-service.yaml` so the `observer` block changes from:

```yaml
observer:
  tekton_manifest_enabled: false
  manifest_runtime_enabled: true
```

to:

```yaml
observer:
  manifest_runtime_enabled: true
```

- [ ] **Step 2: Remove the dead key from staging config**

Edit `/Users/songbei/devflow-repo-config/staging/runtime-service.yaml` with the same shape change:

```yaml
observer:
  manifest_runtime_enabled: true
```

- [ ] **Step 3: Verify both runtime-service configs no longer contain the removed key**

Run:

```bash
rg -n "tekton_manifest_enabled" /Users/songbei/devflow-repo-config/production /Users/songbei/devflow-repo-config/staging
```

Expected: no matches for runtime-service config files.

- [ ] **Step 4: Commit the external config repo changes**

Run in `/Users/songbei/devflow-repo-config`:

```bash
git add production/runtime-service.yaml staging/runtime-service.yaml
git commit -m "refactor: remove runtime manifest legacy flag"
git push origin main
```

- [ ] **Step 5: Record the config repo commit SHA in the service repo handoff note**

Append a short note to the implementation session summary or follow-up comment with this shape:

```md
Config repo sync:
- devflow-repo-config `<commit-sha>` removes `observer.tekton_manifest_enabled` from staging and production runtime-service config.
```

### Task 5: Run Verification And Publish The Service Repo Change

**Files:**
- Modify: `internal/runtime/config/config.go`
- Modify: `internal/runtime/config/config_test.go`
- Modify: `docs/services/runtime-service.md`
- Modify: `docs/system/observability.md`
- Modify: `docs/system/runtime-storage-model.md`

- [ ] **Step 1: Run focused runtime verification**

Run:

```bash
go test ./internal/runtime/config ./internal/runtime/... -count=1
```

Expected: PASS.

- [ ] **Step 2: Run repo-level verification required by the recovery contract**

Run:

```bash
bash scripts/verify.sh
```

Expected: PASS, or a pre-existing unrelated repo-contract failure that is documented before proceeding.

- [ ] **Step 3: Run a final legacy-reference scan in active code and docs**

Run:

```bash
rg -n "tekton_manifest_enabled|StartTektonManifestObserver|startTektonManifestObserver" internal/runtime docs/services docs/system
```

Expected: no matches in active runtime code and current truth docs.

- [ ] **Step 4: Commit and push the service repo cleanup**

```bash
git add internal/runtime/config/config.go internal/runtime/config/config_test.go docs/services/runtime-service.md docs/system/observability.md docs/system/runtime-storage-model.md
git commit -m "refactor: remove manifest runtime legacy compatibility"
git push origin main
```

- [ ] **Step 5: Capture deployment and rollback notes in the closeout**

Use this closeout shape:

```md
Deployment note:
- `runtime-service` must be redeployed after the service repo push.
- `devflow-repo-config` staging and production runtime-service config must already be applied before or with the rollout.

Rollback note:
- rollback requires reverting both the service repo cleanup commit and the config repo commit that removed `tekton_manifest_enabled`.
```
