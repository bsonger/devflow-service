# Scripts

This directory contains repo-level verification and support scripts.

## Reader and outcome

This guide is for a fresh engineer or agent landing in `devflow-service`.
After reading it, the reader should know which repo-local script to run first and what that script proves.

## Canonical verifier

Run this from the repo root before handoff or after changing root docs, recovery guidance, shared package surfaces, or repository structure:

```sh
bash scripts/verify.sh
```

This is the canonical repo-local handoff check for the current root-module baseline.

## What `verify.sh` checks

The verifier fails fast and checks:
- root `go.mod` exists and is non-empty
- required root docs exist and are non-empty
- root entrypoints point to `docs/recovery.md` and `bash scripts/verify.sh`
- repo-local docs mention the root-module contract
- expected shared baseline packages exist under `shared/httpx` and `shared/loggingx`
- `go test ./...` passes as the authoritative compile/test proof for the code currently landed here

This keeps the repo-local verifier honest: it proves the local handoff surface exists and that the root module plus extracted shared packages still compile.

## What this verifier does not claim

`verify.sh` does **not** claim that migrated owner-service modules, runnable binaries, or gateway implementations already exist.
It verifies the repository-local root-module and shared-baseline contract only.

## Expected future role

Later slices can extend this directory with real repo-wide helpers for:
- migration integrity checks once owner modules land
- whole-repo verification that composes module-level checks
- build or generation helpers that are truly repo-wide

Any future script added here should remain runnable from the repo root and should be documented in reader-first terms.
