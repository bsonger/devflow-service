# Scripts

This directory is reserved for repo-level verification and support scripts.

## Current state

No repo-local scripts are required for the bootstrap slice.
The repository is still in documentation-and-structure setup, so the honest verifier is currently the root structural check documented in `AGENTS.md` and `docs/observability.md`.

## Expected future role

Later slices should add real scripts here for tasks such as:
- whole-repo verification
- workspace validation
- migration integrity checks
- build or generation helpers that are truly repo-wide

## Script contract

When scripts land here, they should:
- be runnable from the repo root
- be documented here in reader-first terms
- avoid pretending to verify behavior that the repository does not yet implement
- provide one preferred verifier path a fresh agent can run before handoff

## Current preferred verification

Until a real verifier script exists, use the structural root check:

```sh
test -f AGENTS.md && test -f README.md && test -f docs/architecture.md && test -f scripts/README.md && test -d cmd && test -d modules && test -d shared && test -d gateway
```
