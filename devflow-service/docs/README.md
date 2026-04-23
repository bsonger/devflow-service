# Docs

This directory holds monorepo-wide documentation for `devflow-service`.
It is the root documentation layer for the backend monorepo, not the place for detailed per-module implementation docs.

## Read in this order

1. `recovery.md` — current milestone/slice ownership, root module path, Go baseline, pending work, and the next verifier command
2. `docker.md` — controlled Docker baseline, approved Aliyun image catalog, and the ban on inline install commands in service Dockerfiles
3. `architecture.md` — what the monorepo shape is and how the top-level areas are intended to work
4. `constraints.md` — what must not be collapsed, invented, or migrated prematurely
5. `observability.md` — how a fresh agent should inspect and verify the current repository state

## What belongs here

Keep root-level docs here when they apply across multiple future modules or explain the repo-wide contract:
- root-module build baseline
- controlled Docker baseline and approved image catalog
- monorepo structure
- migration-stage constraints
- shared observability and verification entrypoints
- navigation into future module-local docs

## What does not belong here

Do not put these at the root-doc layer:
- module-specific implementation details that belong under a future module-local docs area
- current production ownership truth that still belongs in `devflow-control`
- invented API contracts for services that have not been migrated yet

## Current scope

During M005/S02, these docs describe a repository with one real root `go.mod` and the first extracted shared packages.
During M005/S03, they also reserve the Docker contract future migrated services must follow.
They should stay honest about what is already real, what is still reserved, and what work remains for S03/S04/S05.
