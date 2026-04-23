# Constraints

These constraints apply to the bootstrap state of `devflow-service` and exist to keep later migration slices honest.

## Hard constraints for this bootstrap slice

- Do not create fake migrated service code just to make the monorepo look populated.
- Do not create `go.work` in this slice.
- Do not create per-service `go.mod` files in this slice.
- Do not introduce a hidden catch-all service or facade module to replace explicit ownership.
- Do not use `shared/` as a dumping ground for app, config, network, release, or runtime business logic.
- Do not use `gateway/` as a new source of truth for backend ownership.

## Repository-boundary constraints

The future monorepo is backend scope only.
The following remain outside this repository:
- `devflow-control` for current-state authority, target architecture, and migration governance
- `devflow-platform-web` for frontend behavior and browser-facing implementation

## Ownership constraints carried forward

Later migration slices must preserve these future-state rules:
- owner-service boundaries remain explicit under `modules/`
- common backend infrastructure absorbs into `shared/`
- verify-related migration lands with release-owned destinations
- observer and watch migration lands with runtime-owned destinations
- the old orchestrator shape must not be recreated as a hidden replacement module

## Documentation constraints

Root docs in this repository must stay honest about maturity.
They should describe what is already real, what is reserved, and what is intentionally deferred.
They must not claim that verification, APIs, or module internals exist before they are actually added.

## Verification constraint

Until repo-local verifier scripts land, structural verification is the only honest bootstrap verifier.
If a task needs stronger verification, it must add a real script or test entrypoint in the same change rather than implying one exists.
