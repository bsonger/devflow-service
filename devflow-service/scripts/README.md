# Scripts

This directory contains repo-level verification and support scripts.

## Reader and outcome

This guide is for a fresh engineer or agent landing in `devflow-service`.
After reading it, the reader should know which repo-local script to run first and what that script proves.

## Canonical verifier

Run this from the repo root before handoff or after changing root docs, recovery guidance, or repository structure:

```sh
bash scripts/verify.sh
```

This is the canonical repo-local handoff check for the bootstrap repository.

## What `verify.sh` checks

The verifier fails fast and checks:
- required root docs exist and are non-empty
- reserved top-level directories still exist
- root entrypoints point to `docs/recovery.md` and `bash scripts/verify.sh`
- the upstream frozen-doc verifier scripts in `devflow-control` still pass

The final two checks are delegated to:
- `../devflow-control/scripts/verify-devflow-service-blueprint.sh`
- `../devflow-control/scripts/verify-devflow-service-migration-handoff.sh`

This keeps the repo-local verifier honest: it proves the local handoff surface exists and that upstream migration authority has not drifted.

## What this verifier does not claim

`verify.sh` does **not** claim that migrated services, build files, or runnable binaries already exist.
It verifies the repository-local bootstrap and recovery contract only.

## Expected future role

Later slices can extend this directory with real repo-wide helpers for:
- workspace validation once build files exist
- migration integrity checks
- whole-repo verification that composes module-level checks
- build or generation helpers that are truly repo-wide

Any future script added here should remain runnable from the repo root and should be documented in reader-first terms.
