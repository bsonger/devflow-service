# Architecture

## Role of this repository

`devflow-service` is the future backend monorepo for DevFlow.
In M005/S01, it exists as the real repository root and documentation surface that later migration slices will populate.
This slice reserves the monorepo shape without claiming that backend services have already been moved.

## Root structure

The top-level repository shape is reserved as follows:
- `cmd/` for runnable entrypoints only
- `modules/` for explicit owner-service destinations
- `shared/` for infrastructure-only common code
- `gateway/` for edge and Kong-facing backend surfaces
- `docs/` for monorepo-wide documentation
- `scripts/` for repo-level verification and support scripts

These areas exist now so future work can land in a stable structure instead of inventing layout during each migration task.

## Boundary model

The monorepo keeps service ownership explicit.
Moving into one repository does **not** mean flattening backend domains into one shared package tree.

The intended ownership model remains:
- app concerns stay app-owned
- config concerns stay config-owned
- network concerns stay network-owned
- release concerns stay release-owned
- runtime concerns stay runtime-owned

`shared/` is reserved for infrastructure such as bootstrap, transport helpers, and observability plumbing.
`gateway/` is reserved for backend edge configuration and gateway-facing contracts.
Neither area is allowed to become a hidden business-logic owner.

## Migration-stage contract

This bootstrap slice is intentionally narrow.
It creates:
- the repository itself
- root docs and agent entrypoints
- empty-but-real top-level landing zones with README placeholders

It intentionally does **not** create:
- migrated service code
- generated assets
- fake binaries or fake service layouts
- the final workspace contract files

Per the M005 target architecture and migration handoff, later slices will introduce the actual Go workspace setup.
For the near-term M005 path, use a **single root `go.mod` in later slices** rather than adding per-service `go.mod` files during this bootstrap task.
This task therefore reserves module boundaries in prose and directory shape, not in build metadata yet.

## Relationship to upstream authority

This repo is the future implementation destination.
Current-state ownership truth and future-state migration authority still live in `devflow-control`.
If this root architecture ever conflicts with the target architecture documents there, the upstream target docs win until intentionally updated.

## Cold-reader outcome

A fresh reader should be able to tell from this repository alone:
- why `devflow-service` exists
- which top-level areas are already reserved
- what not to create yet
- where to go for the authoritative future-state migration contract
