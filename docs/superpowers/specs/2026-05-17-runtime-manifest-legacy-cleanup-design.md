# Runtime Manifest Legacy Cleanup Design

## Purpose

This design defines how `runtime-service` should finish the manifest runtime
migration by removing the remaining legacy Tekton manifest observer compatibility
path.

It addresses two linked questions:

1. whether the current manifest runtime migration is already functionally
   complete
2. whether the legacy `tekton_manifest_enabled` switch and legacy startup lane
   should now be removed entirely

The design goal is to move from “migration completed but compatibility still
present” to a single, clean manifest runtime contract.

## Scope

This design covers:

- how to judge the current manifest runtime migration state
- removal of the `tekton_manifest_enabled` config field
- removal of the legacy Tekton manifest observer startup branch
- required test, config, and documentation cleanup
- the final manifest runtime configuration contract

This design does not cover:

- release-runtime cleanup
- new manifest business logic
- Tekton API contract redesign
- a broader runtime observer architecture rewrite

## Constraints agreed during brainstorming

The user-approved constraints are:

- the current manifest runtime migration should be judged first before deleting
  compatibility code
- if migration is already complete, the cleanup should be strong rather than
  partial
- `tekton_manifest_enabled` should be removed from code, config, and docs
- the legacy Tekton manifest observer startup lane should be deleted rather than
  preserved as dormant compatibility code

## Current state assessment

The current code shows that manifest runtime migration is already
functionally complete.

The important current facts are:

- `observer.manifest_runtime_enabled` starts
  `bootstrap.StartManifestRuntimeReconciler`
- `startTektonManifestObserver(...)` is already skipped whenever
  `observer.manifest_runtime_enabled=true`
- live config intentionally sets `tekton_manifest_enabled: false`, which means
  the legacy Tekton manifest observer is explicitly disabled rather than relied
  upon for production behavior

Therefore the correct status judgment is:

- manifest runtime migration is behaviorally complete
- manifest runtime cleanup is not yet structurally complete

The remaining gap is cleanup, not missing functionality.

## Proposed approaches

### Approach A — Strong cleanup after confirming migration completion

Treat the migration as complete and delete the remaining compatibility layer.

Concretely:

- remove `observer.tekton_manifest_enabled`
- remove `startTektonManifestObserver(...)`
- remove the legacy observer startup hook and related tests
- keep `observer.manifest_runtime_enabled` as the only manifest lane switch
- remove the old key from live config and docs

Pros:

- code, config, and docs all align with the actual runtime architecture
- no more ambiguity about whether manifest runtime still depends on legacy
  polling
- easier future maintenance and fewer false signals in config reviews

Cons:

- touches runtime config, tests, docs, and live config together
- exposes any forgotten dependency on the legacy path immediately

Recommendation: **accept**.

### Approach B — Remove the config key but keep dormant legacy code

Delete `tekton_manifest_enabled` from live config while keeping the old startup
path in code.

Pros:

- smaller immediate code change
- lower short-term risk if someone later tries to re-enable the old path

Cons:

- preserves architecture drift between code and reality
- continues to mislead readers into thinking migration may still be incomplete
- defers cleanup work rather than finishing it

Recommendation: reject.

### Approach C — Keep both paths and only clarify docs

Leave code and config compatibility in place, but document that the new path is
the real one.

Pros:

- almost no implementation risk

Cons:

- does not satisfy the strong-cleanup goal
- leaves migration debt in the active runtime path
- keeps unnecessary testing and config surface alive

Recommendation: reject.

## Selected design

Use **Approach A**.

## Design

### Final manifest runtime contract

After cleanup, manifest runtime should have exactly one active startup contract:

- `observer.manifest_runtime_enabled=true` starts the manifest runtime lane
- `observer.manifest_runtime_enabled=false` does not start the manifest runtime
  lane

There should no longer be:

- `observer.tekton_manifest_enabled`
- a legacy Tekton manifest observer startup branch
- any compatibility language suggesting manifest runtime may fall back to the
  old observer path

This makes the current contract explicit: manifest runtime migration is done,
and the old lane is gone.

### Code cleanup responsibilities

The runtime config layer should be simplified so manifest startup only flows
through the manifest runtime reconciler path.

That means:

- delete `TektonManifestEnabled` from `ObserverConfig`
- delete `startTektonManifestObserverFn` test hook
- delete `startTektonManifestObserver(...)`
- remove the `StartTektonManifestObserver` wiring from runtime startup
- keep `startManifestRuntimeReconciler(...)` as the only manifest startup path

The startup sequence should still remain explicit and readable, but the
manifest-specific compatibility branch should disappear.

### Test cleanup responsibilities

Tests should be rewritten to prove the final contract rather than the migration
bridge contract.

Keep tests that prove:

- manifest runtime starts when `manifest_runtime_enabled=true`
- manifest runtime does not start when `manifest_runtime_enabled=false`
- startup safely handles missing in-cluster config where appropriate

Delete or rewrite tests that currently prove:

- legacy Tekton observer is disabled when manifest runtime is enabled
- `tekton_manifest_enabled` toggles the old observer path

Those tests become invalid once the legacy lane no longer exists.

### Configuration cleanup responsibilities

The live configuration contract should be reduced to the active fields only.

After cleanup:

- staging config should not contain `tekton_manifest_enabled`
- production config should not contain `tekton_manifest_enabled`
- config examples and docs should not contain `tekton_manifest_enabled`

Keeping the field with a fixed `false` value after cleanup would be misleading,
because it would describe a switch that no longer does anything.

### Documentation cleanup responsibilities

Docs should explicitly say:

- manifest runtime migration is complete
- `manifest_runtime_enabled` is the only manifest lane switch
- the legacy Tekton manifest observer has been removed

Docs should stop saying:

- legacy manifest polling remains a current compatibility fallback
- `tekton_manifest_enabled` is part of the active runtime contract

### Risk model

The main risks are cleanup risks, not product-behavior risks.

The important risks are:

- a forgotten test or helper may still expect the legacy startup hook
- a forgotten live environment may still rely on the removed config key for
  review or templating
- a doc or runbook may still mention the deleted flag and confuse operators

These are acceptable because they are exactly the inconsistencies the cleanup is
meant to surface and remove.

### Acceptance criteria

This design is complete when all of the following are true:

- `tekton_manifest_enabled` no longer appears in runtime code
- the legacy Tekton manifest observer startup path is deleted
- manifest runtime startup is controlled only by
  `observer.manifest_runtime_enabled`
- tests no longer assert legacy observer compatibility behavior
- staging and production config no longer contain `tekton_manifest_enabled`
- current docs no longer describe the legacy Tekton manifest observer as an
  active or fallback contract

## Non-goals for this slice

This design intentionally does not include:

- removing the release-runtime legacy cleanup path in the same change
- changing Tekton task status mapping
- changing manifest queue/informer behavior
- redesigning runtime-service configuration structure beyond this one legacy key
