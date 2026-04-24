# API Compatibility Policy

This policy is adapted from `devflow-control` for use inside `devflow-service`.
If it conflicts with current implementation-facing docs in this repo, resolve that conflict instead of applying this file mechanically.

## Purpose

This policy defines how public HTTP APIs in `devflow-service` should evolve without silent compatibility breaks.

## When this applies

- when a public REST path changes
- when request or response fields change
- when enum or public status values change
- when writeback or callback contracts change
- when a change needs deprecation, migration notes, or a documented cutover plan

## Hard rules

1. Do not ship silent breaking changes to public paths, request fields, response fields, enum values, or callback contracts.
2. The human-authored source of current API truth in this repo is the affected `docs/services/*.md` file plus the handler and transport code.
3. If Swagger annotations or generated contract output are used, regenerate them in the same change cycle.
4. If a change is not fully backward-compatible, document the compatibility impact before calling the work done.

## Deprecation rules

Use deprecation instead of immediate removal when existing consumers could still rely on the current contract.

When deprecating:
- mark the deprecated path, field, or behavior in the affected service doc
- regenerate generated API artifacts when the repo uses them
- document the replacement path or field
- document the removal condition or expected removal window when known

## Breaking-change rules

A breaking change requires all of the following:
- explicit statement in active change artifacts or handoff notes
- migration note describing what consumers must change
- current repo-local docs updated in the same implementation cycle
- regenerated generated contract output when applicable

## Required follow-up docs

If an API change affects ownership, service boundaries, or runtime flow, also update:
- `docs/services/meta-service.md`
- the affected `docs/services/*.md` resource file
- `docs/system/architecture.md`
- `docs/policies/verification.md` when the proof surface changes
