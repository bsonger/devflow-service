# Runtime Manifest Hard Delete Design

## Purpose

This design defines the final cleanup step for manifest runtime legacy code in
`runtime-service`.

The previous cleanup removed the legacy manifest observer startup wiring and the
`tekton_manifest_enabled` config flag. The remaining gap is that the repository
still contains:

- an unused legacy Tekton manifest observer implementation
- tests for that unused implementation
- runtime manifest writeback fallback from the current release-owned callback
  paths back to the older `/api/v1/manifests/tekton/*` paths

This slice removes those leftovers completely so the manifest runtime path has
one implementation and one callback contract.

## Scope

This design covers:

- deleting the unused legacy Tekton manifest observer code
- deleting its dedicated tests
- removing manifest writeback fallback to legacy HTTP paths from the active
  manifest runtime reconciler
- updating current runtime docs so they describe only the final manifest
  callback contract

This design does not cover:

- changing manifest business semantics
- changing the release-service manifest handlers themselves
- cleaning historical archived specs and plans unless they affect current truth
- redesigning Tekton task/result payload formats

## Current state assessment

After the earlier cleanup, the current runtime startup contract is already:

- `observer.manifest_runtime_enabled=true` starts the manifest runtime
  reconciler
- no startup path calls `StartTektonManifestObserver`
- live runtime-service config no longer depends on `tekton_manifest_enabled`

But three legacy residues still exist:

1. `internal/runtime/observer/tekton_manifest.go` still exists even though the
   active runtime startup path no longer references it
2. `internal/runtime/observer/tekton_manifest_test.go` still tests that dead
   implementation
3. `internal/runtime/bootstrap/manifest_runtime.go` still posts manifest
   writeback to the current release-owned paths and then falls back to the old
   `/api/v1/manifests/tekton/*` paths on not found

So the migration is operationally complete but not codebase-clean.

## User-approved direction

The user explicitly chose the strong-delete option:

- delete the dead observer implementation
- delete the dead observer tests
- delete the old writeback fallback paths
- keep only the current release-owned manifest callback paths

## Approaches considered

### Approach A — Hard delete all remaining manifest legacy runtime code

Remove the dead implementation and remove the old callback fallback.

This means:

- delete `internal/runtime/observer/tekton_manifest.go`
- delete `internal/runtime/observer/tekton_manifest_test.go`
- remove legacy manifest path constants from
  `internal/runtime/bootstrap/manifest_runtime.go`
- remove fallback posting to `/api/v1/manifests/tekton/*`
- keep only `/api/v1/release/manifests/tekton/status`
- keep only `/api/v1/release/manifests/tekton/tasks`
- keep only `/api/v1/release/manifests/tekton/result`

Pros:

- the runtime manifest lane becomes structurally honest
- no dead code remains to mislead future cleanup work
- one implementation path and one writeback contract are left

Cons:

- any caller still depending on old manifest callback paths will break
- tests and docs must be updated together

Recommendation: **accept**.

### Approach B — Delete dead observer code but keep old writeback fallback

Pros:

- lower immediate compatibility risk

Cons:

- runtime still carries two callback contracts
- does not satisfy the user's “delete completely” requirement
- preserves code ambiguity around the actual active manifest contract

Recommendation: reject.

## Selected design

Use **Approach A**.

## Design

### Final manifest callback contract

After this cleanup, runtime-side manifest writeback must use exactly these
paths:

- `/api/v1/release/manifests/tekton/status`
- `/api/v1/release/manifests/tekton/tasks`
- `/api/v1/release/manifests/tekton/result`

There is no runtime-side fallback to:

- `/api/v1/manifests/tekton/status`
- `/api/v1/manifests/tekton/tasks`
- `/api/v1/manifests/tekton/result`

If those older paths still exist elsewhere in the system, they are no longer
part of the runtime-service active callback contract.

### Code cleanup responsibilities

The runtime package should be reduced to the current manifest reconciler only.

That means:

- delete `internal/runtime/observer/tekton_manifest.go`
- delete `internal/runtime/observer/tekton_manifest_test.go`
- remove the `manifestTekton*LegacyPath` constants from
  `internal/runtime/bootstrap/manifest_runtime.go`
- change manifest writeback calls so they post only to the release-owned path
- delete any runtime-only helper logic that exists solely to support legacy
  fallback ordering

The runtime bootstrap package remains the only active manifest runtime
implementation surface after cleanup.

### Test cleanup responsibilities

Tests should prove the final single-contract behavior.

Keep tests that prove:

- manifest runtime reconciler posts to the release-owned manifest callback paths
- manifest runtime task, status, and result writeback still emit the current
  payload shape
- runtime bootstrap behavior remains correct when the release-owned path
  succeeds or fails

Delete tests that prove:

- legacy observer sync behavior
- legacy observer path ordering
- fallback from new manifest callback paths to old manifest callback paths

### Documentation cleanup responsibilities

Current truth docs should say:

- runtime-service no longer contains the legacy Tekton manifest observer
  implementation
- runtime manifest writeback targets only the release-owned callback paths
- old `/api/v1/manifests/tekton/*` fallback is gone from the active runtime
  contract

Current truth docs should stop saying:

- runtime may fall back to old manifest callback paths
- manifest observer legacy compatibility still exists in runtime-service

Historical specs/plans may still mention the old path as history; they are not
the active contract source.

## Risks

The main risk is compatibility exposure, not internal correctness.

The concrete risk is:

- some out-of-date caller, environment, or test may still assume runtime will
  retry against `/api/v1/manifests/tekton/*`

This risk is acceptable because:

- the user explicitly chose hard deletion
- the startup contract has already been migrated
- leaving the fallback in place keeps the codebase misleading

## Acceptance criteria

This design is complete when all of the following are true:

- `internal/runtime/observer/tekton_manifest.go` is deleted
- `internal/runtime/observer/tekton_manifest_test.go` is deleted
- `internal/runtime/bootstrap/manifest_runtime.go` no longer references
  `manifestTektonStatusLegacyPath`
- `internal/runtime/bootstrap/manifest_runtime.go` no longer references
  `manifestTektonTasksLegacyPath`
- `internal/runtime/bootstrap/manifest_runtime.go` no longer references
  `manifestTektonResultLegacyPath`
- active runtime code no longer posts manifest writeback to
  `/api/v1/manifests/tekton/*`
- current runtime truth docs no longer describe runtime-side fallback to the old
  manifest callback paths
- focused runtime tests still pass

## Non-goals for this slice

This slice does not:

- remove historical references from archived planning material
- change release-service route ownership
- remove old manifest HTTP handlers from non-runtime packages unless they are
  proven dead and brought into a separate approved cleanup
