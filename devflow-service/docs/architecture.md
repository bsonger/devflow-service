# Architecture

## Role of this repository

`devflow-service` is the future backend monorepo for DevFlow.
In M005/S02 it stops being a docs-only skeleton and becomes a real Go repository with one root module plus the first infrastructure-only shared packages.
Service migrations under `modules/` and runnable entrypoints under `cmd/` remain intentionally deferred.

## Root structure

The top-level repository shape is:
- `go.mod` for the current repository-wide build contract
- `cmd/` for runnable entrypoints only
- `modules/` for explicit owner-service destinations
- `shared/` for infrastructure-only common code
- `gateway/` for edge and Kong-facing backend surfaces
- `docs/` for monorepo-wide documentation, including `docs/docker.md` for the controlled Docker baseline
- `scripts/` for repo-level verification and support scripts

These areas exist now so future work can land in a stable structure instead of inventing layout during each migration task.

## Current module model

The current local execution truth is a **single root Go module**:
- module path: `github.com/bsonger/devflow-service`
- Go baseline: `1.25.8`

This root module exists so extracted shared packages can compile and be tested immediately.
The choice intentionally supersedes older sibling-repo patch levels such as `1.25.6`, because the controlled builder image already ships `1.25.8`.

This is a staged migration contract, not a claim that the long-term workspace discussion is closed forever.
For S02, however, the repository must behave as one root module and must not introduce `go.work` or per-service `go.mod` files.
If older session memory or upstream notes still describe a root `go.work` or multi-module baseline, treat that as stale until a later slice changes the repo-local docs, verification, and code together.

## Boundary model

The monorepo keeps service ownership explicit.
Moving into one repository does **not** mean flattening backend domains into one shared package tree.

The intended ownership model remains:
- app concerns stay app-owned
- config concerns stay config-owned
- network concerns stay network-owned
- release concerns stay release-owned
- runtime concerns stay runtime-owned

`shared/` is for infrastructure such as bootstrap, transport helpers, router middleware, and observability plumbing.
`gateway/` is reserved for backend edge configuration and gateway-facing contracts.
Neither area is allowed to become a hidden business-logic owner.

## Docker packaging direction

S03 reserves a per-service Docker packaging model before any migrated service lands under `modules/`.
That Docker contract is documented in `docs/docker.md` and follows these repo-local rules:
- use the approved Aliyun registry namespace `registry.cn-hangzhou.aliyuncs.com/devflow`
- use the controlled builder baseline `golang-builder:1.25.8`
- prefer artifact-first packaging into a thin per-service runtime image
- keep inline install commands such as `apk add`, `apt-get`, `yum`, `dnf`, or `go install` out of future service Dockerfiles

This matches the sibling-repo staging pattern where build artifacts are prepared first and `Dockerfile.package` performs the final packaging step.
The contract exists now so S04 can add a real service Dockerfile without inventing Docker policy during migration.


This slice makes only a narrow set of things real:
- the root `go.mod`
- the root `go 1.25.8` baseline
- extracted infrastructure packages under `shared/httpx`, `shared/loggingx`, `shared/otelx`, `shared/pyroscopex`, `shared/observability`, `shared/routercore`, and `shared/bootstrap`
- repo-local docs and recovery/verifier surfaces that describe the current contract honestly

These extracted packages map cleanly to the infrastructure already evidenced in sibling repos:
- `httpx` covers response and pagination helpers already used by API handlers
- `loggingx` owns structured logger setup and request/trace context enrichment
- `otelx` and `pyroscopex` provide tracing, metrics, and profiling bootstrap
- `observability` composes runtime init plus metrics, pprof, and dependency-call signals
- `routercore` provides the Gin middleware seam services can share without sharing domain handlers
- `bootstrap` provides service startup orchestration without owning app-specific config or routing

It intentionally does **not** create:
- migrated owner-service code under `modules/`
- runnable binaries under `cmd/`
- fake APIs or fake runtime behavior
- the final multi-module workspace assembly files

## Relationship to upstream authority

This repo is the implementation destination.
`devflow-control` still holds the broader migration history and long-term target architecture, but this repository's docs define the active local contract for the current slice.
If those two sources diverge during M005/S02, treat the repo-local root-module contract as the current implementation truth and record the discrepancy for later reconciliation.

## Cold-reader outcome

A fresh reader should be able to tell from this repository alone:
- that the repo already has a real root build contract
- which shared packages are currently extracted
- what not to create yet
- what remains pending for later slices
