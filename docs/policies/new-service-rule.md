# New Service Rule

This policy is migrated from `devflow-control` as a local reference.
It does not override the current repository scope: the active migration in this repo is still `meta-service`, not a broad multi-service expansion.

## Purpose

This policy captures the default rule set for creating or admitting a new service boundary into the DevFlow backend layout.

## When this applies

- when a new backend service is proposed
- when reviewing whether a proposed repo or service shape matches the DevFlow baseline
- when deciding whether a new ownership boundary belongs in this repo or somewhere else

## Hard rules

- classify ownership before writing implementation code
- define purpose, ownership, and non-goals before scaffolding code
- keep repo-local implementation detail in the owning repo docs rather than in copied control summaries
- prefer the current root repo layout contract in `AGENTS.md`, `docs/system/`, and `docs/policies/`
- do not use this policy as justification to broaden the current `meta-service` migration scope

## Baseline repo shape

For backend services, the preferred local structure is:

```text
cmd/<service>/main.go
internal/<domain>/domain/
internal/<domain>/service/
internal/<domain>/repository/
internal/<domain>/transport/
internal/<domain>/module.go
internal/platform/
docs/
scripts/
```

## Required baseline docs

- `README.md`
- `AGENTS.md`
- `docs/system/architecture.md`
- `docs/system/constraints.md`
- `docs/system/observability.md`
- `docs/system/recovery.md`
- relevant `docs/services/*.md`
- relevant `docs/resources/*.md` when the service exposes resource contracts
- `docs/policies/*.md`

## Verification expectation

Any new service baseline is not complete until the repo-local verification contract is wired and documented:

```sh
bash scripts/verify.sh
```
