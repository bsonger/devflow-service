# Recovery

## Reader and outcome

This document is for a fresh engineer or agent landing in `devflow-service` without prior session memory.
After reading it, the reader should know:
- what M005/S01 already established
- what remains intentionally pending for S02 and later slices
- which command to run next to verify the repository-local handoff surface
- which docs to read in order before changing the repo

## Current phase ownership

- Milestone: `M005`
- Slice: `S01`
- Slice goal: create the real `devflow-service` repository skeleton with repository-local startup, recovery, and verification entrypoints
- Current task status in this slice:
  - `T01` complete — root repository skeleton and root entrypoint docs exist
  - `T02` complete when `bash scripts/verify.sh` passes from the repo root

## What S01 established

S01 created the real sibling repository at `devflow-service` and made the monorepo root navigable before service migration begins.
The slice established these repository-local surfaces:
- root `README.md` describing repo purpose and current bootstrap state
- root `AGENTS.md` describing startup order and the preferred repo-local verification path
- root docs covering architecture, constraints, and observability
- reserved top-level landing zones for `cmd/`, `modules/`, `shared/`, and `gateway/`
- one repo-local verifier entrypoint at `scripts/verify.sh`

This means a fresh reader can now determine the current migration phase from inside this repo instead of relying only on upstream target docs in `devflow-control`.

## What is intentionally pending for S02+

The following are intentionally **not** part of S01 and should be introduced only by later migration slices:
- the real root `go.mod`
- any `go.work` file
- per-service `go.mod` files
- migrated backend service code under `modules/`
- runnable binaries under `cmd/`
- generated assets, fake APIs, or placeholder runtime behavior

Near-term M005 work is expected to add the real root build/workspace contract in later slices before the repository converges on its final multi-module state.

## Read this next

If you are landing here cold, read in this order:
1. `README.md` — repo purpose and current bootstrap state
2. `AGENTS.md` — startup rules and canonical verifier command
3. `docs/README.md` — docs map
4. `docs/architecture.md` — monorepo shape and ownership boundaries
5. `docs/constraints.md` — what must not be created yet
6. `docs/observability.md` — inspection and verification surfaces
7. `scripts/README.md` — repo-local verifier contract

If ownership, migration authority, or future-state boundary questions remain after that, consult the upstream frozen authority in `../devflow/devflow-control/docs/target-architecture/`.

## Canonical verification command

Run this from the `devflow-service` repo root:

```sh
bash scripts/verify.sh
```

This verifier is the canonical repo-local handoff check for S01.
It fails fast and reports which required local surfaces are missing before rerunning the upstream frozen-doc verifiers from `devflow-control`.

## What `scripts/verify.sh` proves

A passing run means:
- the required root docs and reserved top-level directories still exist
- the repository-local recovery surface is present and wired from the root entrypoints
- the repo-local verifier is present and executable
- upstream blueprint and migration-handoff docs in `devflow-control` still pass their own frozen-contract verification

A passing run does **not** mean migrated services or workspace/build files exist yet.
It only proves the repository-local bootstrap and recovery contract is intact and still aligned with upstream authority.

## If verification fails

Use the first failing line to decide where to inspect next:
- missing root files or directories → restore the local repository skeleton and root docs
- missing recovery/verifier references → rewire `README.md`, `AGENTS.md`, or `scripts/README.md`
- upstream verifier failure → inspect the referenced `devflow-control` target-architecture docs and update this repo only if the local handoff surface drifted from upstream truth

Do not add fake code or fake build files just to satisfy the verifier.
If the verifier reveals a real contract change, update the docs and script so the repository stays honest.
