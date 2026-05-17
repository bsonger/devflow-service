# Runtime Manifest Hard Delete Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the remaining manifest-runtime legacy observer implementation and old manifest writeback fallback so runtime-service keeps exactly one manifest runtime implementation and one callback contract.

**Architecture:** The active manifest runtime implementation already lives in `internal/runtime/bootstrap/manifest_runtime.go`, but runtime still carries a dead `internal/runtime/observer/tekton_manifest.go` implementation plus fallback posting to `/api/v1/manifests/tekton/*`. This plan deletes the dead observer package surface, rewrites bootstrap tests around release-owned-only callback paths, then updates current runtime docs so the code and docs describe the same single-contract architecture.

**Tech Stack:** Go, Tekton client types, runtime bootstrap/reconcile packages, Go tests, Markdown docs

---

## File Structure

- Delete: `internal/runtime/observer/tekton_manifest.go`
  Responsibility: remove the dead legacy Tekton manifest observer implementation that is no longer started by runtime-service.
- Delete: `internal/runtime/observer/tekton_manifest_test.go`
  Responsibility: remove tests that only validate the deleted legacy observer implementation and its fallback path ordering.
- Modify: `internal/runtime/bootstrap/manifest_runtime.go`
  Responsibility: remove legacy manifest callback path constants and remove fallback posting to `/api/v1/manifests/tekton/*`.
- Modify: `internal/runtime/bootstrap/manifest_runtime_test.go`
  Responsibility: rewrite manifest writer tests so they prove release-owned-only callback behavior.
- Modify: `docs/services/runtime-service.md`
  Responsibility: describe that runtime-service no longer contains the legacy manifest observer implementation and no longer falls back to the old manifest callback paths.
- Modify: `docs/system/observability.md`
  Responsibility: remove current-truth wording that says manifest runtime may fall back to old manifest callback paths.
- Modify: `docs/system/runtime-storage-model.md`
  Responsibility: align runtime storage/callback truth with the hard-deleted manifest fallback.

### Task 1: Delete The Dead Legacy Tekton Manifest Observer Files

**Files:**
- Delete: `internal/runtime/observer/tekton_manifest.go`
- Delete: `internal/runtime/observer/tekton_manifest_test.go`
- Test: `internal/runtime/observer`

- [ ] **Step 1: Write the failing package-level verification by checking for the dead files and their runtime references**

Run:

```bash
rg -n "StartTektonManifestObserver|TektonManifestObserver|manifestTektonStatusLegacyPath|manifestTektonResultLegacyPath|manifestTektonTasksLegacyPath" internal/runtime/observer internal/runtime/bootstrap
```

Expected: matches in `internal/runtime/observer/tekton_manifest.go`, `internal/runtime/observer/tekton_manifest_test.go`, and `internal/runtime/bootstrap/manifest_runtime.go`.

- [ ] **Step 2: Delete the dead legacy observer implementation file**

Delete:

```text
internal/runtime/observer/tekton_manifest.go
```

This file is no longer referenced by runtime startup and should be removed entirely rather than left dormant.

- [ ] **Step 3: Delete the dead legacy observer test file**

Delete:

```text
internal/runtime/observer/tekton_manifest_test.go
```

These tests only validate the deleted legacy observer implementation and its fallback behavior, so they must be removed with the code they cover.

- [ ] **Step 4: Run focused observer-package tests to verify the deletion does not leave package-level compile holes**

Run:

```bash
go test ./internal/runtime/observer -count=1
```

Expected: PASS if no surviving observer test or helper still depends on the deleted Tekton manifest observer types.

- [ ] **Step 5: Commit the dead observer deletion**

```bash
git add internal/runtime/observer
git commit -m "refactor: delete legacy manifest observer implementation"
```

### Task 2: Remove Manifest Writeback Fallback From Runtime Bootstrap

**Files:**
- Modify: `internal/runtime/bootstrap/manifest_runtime.go`
- Modify: `internal/runtime/bootstrap/manifest_runtime_test.go`

- [ ] **Step 1: Write the failing bootstrap test that proves only release-owned callback paths are used**

Add this test to `internal/runtime/bootstrap/manifest_runtime_test.go`:

```go
func TestManifestWriterPostsOnlyReleaseOwnedPaths(t *testing.T) {
	var observedPaths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observedPaths = append(observedPaths, r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	writer := newManifestWriterAdapter(writeback.NewReleaseWriter(
		server.URL,
		"observer-token",
		server.Client(),
	))

	ctx := context.Background()
	if err := writer.WriteStatus(ctx, reconcile.ManifestStatusWrite{
		ManifestID: "manifest-1",
		PipelineID: "pipe-1",
		Status:     model.ManifestRunning,
		Message:    "building",
	}); err != nil {
		t.Fatalf("WriteStatus() error = %v", err)
	}
	if err := writer.WriteTask(ctx, reconcile.ManifestTaskWrite{
		ManifestID: "manifest-1",
		PipelineID: "pipe-1",
		TaskName:   "git-clone",
		TaskRun:    "pipe-1-git-clone",
		Status:     model.StepRunning,
		Message:    "cloning",
	}); err != nil {
		t.Fatalf("WriteTask() error = %v", err)
	}
	if err := writer.WriteResult(ctx, reconcile.ManifestResultWrite{
		ManifestID:  "manifest-1",
		PipelineID:  "pipe-1",
		CommitHash:  "abc123",
		ImageRef:    "repo/demo@sha256:abc",
		ImageTag:    "20260517",
		ImageDigest: "sha256:abc",
	}); err != nil {
		t.Fatalf("WriteResult() error = %v", err)
	}

	want := []string{
		manifestTektonStatusPath,
		manifestTektonTasksPath,
		manifestTektonResultPath,
	}
	if !reflect.DeepEqual(observedPaths, want) {
		t.Fatalf("observed paths = %v, want %v", observedPaths, want)
	}
}
```

- [ ] **Step 2: Run the targeted bootstrap test and verify it fails before removing the fallback**

Run:

```bash
go test ./internal/runtime/bootstrap -run TestManifestWriterPostsOnlyReleaseOwnedPaths -count=1
```

Expected: FAIL because `manifest_runtime.go` still supports multi-path fallback and the new single-path assertion is not yet true.

- [ ] **Step 3: Remove the legacy manifest path constants and fallback loop from bootstrap**

Update `internal/runtime/bootstrap/manifest_runtime.go` to this shape:

```go
const (
	manifestTektonStatusPath    = "/api/v1/release/manifests/tekton/status"
	manifestTektonResultPath    = "/api/v1/release/manifests/tekton/result"
	manifestTektonTasksPath     = "/api/v1/release/manifests/tekton/tasks"
	defaultManifestPollInterval = 15 * time.Second
	defaultTektonNamespace      = "tekton-pipelines"
)

func (a *manifestWriterAdapter) WriteStatus(ctx context.Context, input reconcile.ManifestStatusWrite) error {
	return a.postJSON(ctx, manifestTektonStatusPath, map[string]any{
		"manifest_id": strings.TrimSpace(input.ManifestID),
		"pipeline_id": strings.TrimSpace(input.PipelineID),
		"status":      string(input.Status),
		"message":     strings.TrimSpace(input.Message),
	})
}

func (a *manifestWriterAdapter) WriteTask(ctx context.Context, input reconcile.ManifestTaskWrite) error {
	return a.postJSON(ctx, manifestTektonTasksPath, map[string]any{
		"manifest_id": strings.TrimSpace(input.ManifestID),
		"pipeline_id": strings.TrimSpace(input.PipelineID),
		"task_name":   strings.TrimSpace(input.TaskName),
		"task_run":    strings.TrimSpace(input.TaskRun),
		"status":      string(input.Status),
		"message":     strings.TrimSpace(input.Message),
	})
}

func (a *manifestWriterAdapter) WriteResult(ctx context.Context, input reconcile.ManifestResultWrite) error {
	return a.postJSON(ctx, manifestTektonResultPath, map[string]any{
		"manifest_id":  strings.TrimSpace(input.ManifestID),
		"pipeline_id":  strings.TrimSpace(input.PipelineID),
		"commit_hash":  strings.TrimSpace(input.CommitHash),
		"image_ref":    strings.TrimSpace(input.ImageRef),
		"image_tag":    strings.TrimSpace(input.ImageTag),
		"image_digest": strings.TrimSpace(input.ImageDigest),
	})
}

func (a *manifestWriterAdapter) postJSON(ctx context.Context, path string, payload any) error {
	if a == nil || a.writer == nil {
		return ErrManifestRuntimeNotConfigured
	}
	return a.writer.PostJSON(ctx, path, payload)
}
```

Delete `uniqueManifestPaths(...)` entirely if nothing else uses it.

- [ ] **Step 4: Delete or rewrite bootstrap tests that assert fallback ordering**

Remove any bootstrap test that expects runtime-side fallback from:

```text
/api/v1/release/manifests/tekton/*
```

to:

```text
/api/v1/manifests/tekton/*
```

If there is a negative-path test, rewrite it so a not-found on the release-owned path remains a not-found failure instead of triggering a fallback retry.

- [ ] **Step 5: Run focused bootstrap tests and commit**

Run:

```bash
go test ./internal/runtime/bootstrap -count=1
git add internal/runtime/bootstrap/manifest_runtime.go internal/runtime/bootstrap/manifest_runtime_test.go
git commit -m "refactor: remove manifest callback fallback"
```

Expected: tests PASS and the commit contains only the bootstrap-side fallback cleanup.

### Task 3: Sync Current Runtime Docs To The Hard-Delete Contract

**Files:**
- Modify: `docs/services/runtime-service.md`
- Modify: `docs/system/observability.md`
- Modify: `docs/system/runtime-storage-model.md`

- [ ] **Step 1: Update `docs/services/runtime-service.md` to say the legacy implementation is gone**

Add or replace the manifest-runtime wording with text shaped like:

```md
Manifest runtime implementation is now singular:

- runtime-service no longer contains the legacy `TektonManifestObserver` implementation
- manifest runtime writeback targets only `/api/v1/release/manifests/tekton/status`
- manifest runtime writeback targets only `/api/v1/release/manifests/tekton/tasks`
- manifest runtime writeback targets only `/api/v1/release/manifests/tekton/result`
- runtime-service no longer falls back to `/api/v1/manifests/tekton/*`
```

- [ ] **Step 2: Update `docs/system/observability.md` to remove fallback wording from current truth**

Replace current wording that preserves legacy manifest fallback with text shaped like:

```md
Manifest reconcile rules:

- write manifest status, task, and result callbacks through `/api/v1/release/manifests/tekton/*`
- runtime-service no longer retries those callbacks against the older `/api/v1/manifests/tekton/*` paths
```

- [ ] **Step 3: Update `docs/system/runtime-storage-model.md` to describe the final callback contract**

Replace stale fallback wording with text shaped like:

```md
Manifest runtime callback truth is release-owned only.
The active runtime manifest reconciler posts only to `/api/v1/release/manifests/tekton/status`,
`/api/v1/release/manifests/tekton/tasks`, and `/api/v1/release/manifests/tekton/result`.
```

- [ ] **Step 4: Verify current truth docs no longer describe runtime-side fallback**

Run:

```bash
rg -n "/api/v1/manifests/tekton|legacy Tekton manifest observer|fall back to old manifest callback" docs/services docs/system
```

Expected: no matches in current-truth runtime docs except explicit “removed/no longer” historical wording that matches the new contract.

- [ ] **Step 5: Commit the doc cleanup**

```bash
git add docs/services/runtime-service.md docs/system/observability.md docs/system/runtime-storage-model.md
git commit -m "docs: remove runtime manifest legacy fallback contract"
```

### Task 4: Run Final Verification And Publish

**Files:**
- Delete: `internal/runtime/observer/tekton_manifest.go`
- Delete: `internal/runtime/observer/tekton_manifest_test.go`
- Modify: `internal/runtime/bootstrap/manifest_runtime.go`
- Modify: `internal/runtime/bootstrap/manifest_runtime_test.go`
- Modify: `docs/services/runtime-service.md`
- Modify: `docs/system/observability.md`
- Modify: `docs/system/runtime-storage-model.md`

- [ ] **Step 1: Run focused runtime verification for observer and bootstrap packages**

Run:

```bash
go test ./internal/runtime/bootstrap ./internal/runtime/observer ./internal/runtime/... -count=1
```

Expected: PASS.

- [ ] **Step 2: Run a final legacy-reference scan in active runtime code and current docs**

Run:

```bash
rg -n "StartTektonManifestObserver|TektonManifestObserver|manifestTektonStatusLegacyPath|manifestTektonTasksLegacyPath|manifestTektonResultLegacyPath|/api/v1/manifests/tekton" internal/runtime docs/services docs/system
```

Expected: no matches in active runtime code, and only explicit “removed/no longer” wording in current truth docs if retained intentionally.

- [ ] **Step 3: Run repo-level verification**

Run:

```bash
bash scripts/verify.sh
```

Expected: PASS, or the same known unrelated repository-contract failure caused by stale deleted `deployments/` checks. If it still fails for that known reason, record the exact failure instead of treating it as a regression from this slice.

- [ ] **Step 4: Commit and push the hard-delete cleanup**

```bash
git add internal/runtime/bootstrap/manifest_runtime.go internal/runtime/bootstrap/manifest_runtime_test.go docs/services/runtime-service.md docs/system/observability.md docs/system/runtime-storage-model.md
git add -u internal/runtime/observer
git commit -m "refactor: hard delete manifest runtime legacy paths"
git push origin main
```

- [ ] **Step 5: Record deployment and compatibility notes in the closeout**

Use this closeout shape:

```md
Deployment note:
- `runtime-service` must be redeployed after the service repo push.

Compatibility note:
- runtime-side manifest writeback no longer retries `/api/v1/manifests/tekton/*`.
- any caller or test still depending on that fallback must migrate to `/api/v1/release/manifests/tekton/*`.

Verification note:
- `bash scripts/verify.sh` status: <pass or exact known unrelated failure>.
```
