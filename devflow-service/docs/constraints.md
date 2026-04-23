# Constraints

These constraints apply to the current `devflow-service` baseline and exist to keep later migration slices honest.

## Hard constraints for S02/S03

- Do not create fake migrated service code just to make the monorepo look populated.
- Do not create `go.work` in this slice.
- Do not create per-service `go.mod` files in this slice.
- Do not introduce a hidden catch-all service or facade module to replace explicit ownership.
- Do not use `shared/` as a dumping ground for app, config, network, release, or runtime business logic.
- Do not use `gateway/` as a new source of truth for backend ownership.
- Do not add runnable binaries under `cmd/` until a later slice lands real service behavior.
- Do not add inline install commands such as `apk add`, `apk upgrade`, `apt-get`, `yum`, `dnf`, or `go install` to future service Dockerfiles under `modules/`.

## Repository-boundary constraints

The future monorepo is backend scope only.
The following remain outside this repository:
- `devflow-control` for migration governance and target architecture history
- `devflow-platform-web` for frontend behavior and browser-facing implementation

## Ownership constraints carried forward

Later migration slices must preserve these future-state rules:
- owner-service boundaries remain explicit under `modules/`
- common backend infrastructure absorbs into `shared/`
- verify-related migration lands with release-owned destinations
- observer and watch migration lands with runtime-owned destinations
- the old orchestrator shape must not be recreated as a hidden replacement module

## Build-contract constraints

The active build contract is one root module:
- keep `go.mod` at the repo root
- keep the baseline on `go 1.25.8` unless the controlled builder image changes
- add shared dependencies only when a real extracted package in this repo needs them
- keep the controlled Docker baseline aligned with `docs/docker.md` and the approved Aliyun image catalog

If later slices change the workspace model, they must update docs, recovery guidance, and verification in the same change.
If later slices change approved builder/runtime images, they must update the Docker contract and verification in the same change.

## Documentation constraints

Root docs in this repository must stay honest about maturity.
They should describe what is already real, what is reserved, and what is intentionally deferred.
They must not claim that migrated modules, binaries, or APIs exist before they are actually added.

## Verification constraint

`bash scripts/verify.sh` is the root contract check.
If a task strengthens the repo contract, it must extend the verifier and/or tests in the same change rather than only documenting the expectation.
This includes Docker policy changes: future slices must enforce controlled-image references and the ban on inline install commands instead of leaving them as prose-only guidance.
