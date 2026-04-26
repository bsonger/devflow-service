# Doc Synchronization Policy

Any code change that affects the public API surface, domain models, or service behavior must be reflected in the corresponding documentation before the change is considered complete.

## What must stay in sync

- `docs/resources/*.md` — resource field tables, API surface, validation rules
- `docs/services/*.md` — service-owned endpoints, upstream/downstream contracts, module boundaries
- `docs/system/*.md` — architecture, recovery, and system-level contracts
- `docs/policies/*.md` — if the change introduces a new policy or modifies an existing one
- `AGENTS.md` — if the change affects agent startup contracts, routing, or canonical read sets

## Trigger conditions

Review and update docs when the code change includes any of the following:

- new or removed HTTP endpoints
- new or removed request/response fields
- new or removed database tables or columns
- new or removed domain models
- changes to error codes or validation rules
- changes to service boundaries or ownership
- changes to the canonical verification stack

## How to review

Before marking a task complete:

1. run `git diff` and identify every file that changed
2. for each domain or service touched, open its matching `docs/resources/` or `docs/services/` file
3. check whether the API surface list, field table, or validation notes are still accurate
4. update the doc if anything drifted; do not leave stale field tables or missing endpoints
5. if the change spans multiple services, check `docs/system/architecture.md` for outdated references

## Authority

If a doc and the implementation disagree, the implementation is the current execution truth. The doc must be updated to match the code, not the other way around.
