# Docker contract

## Purpose

This document defines the controlled Docker baseline for `devflow-service` during M005/S03.
It exists so future service-migration work lands on one repo-local packaging policy instead of copying ad hoc patterns from sibling repositories.

This is the repository's **controlled Docker baseline** for future service Dockerfiles under `modules/`.
It applies before any migrated service exists here.

## Current repository truth

Today this repository still has:
- one root `go.mod`
- no migrated owner-service code under `modules/`
- no runnable service binaries under `cmd/`
- no root `go.work`
- no per-service `go.mod` files

If older notes or memories mention a multi-module `go.work` baseline, treat them as stale for this slice.
The active local execution contract remains the S02 single-root-module baseline until a later slice changes docs, verification, and code together.

## Scope

This contract reserves how future service packaging must work when S04 and later slices add real service Dockerfiles under `modules/`.
It does **not** claim those services already exist today.

## Controlled image catalog

Approved registry namespace:
- `registry.cn-hangzhou.aliyuncs.com/devflow`

Approved builder baseline:
- `registry.cn-hangzhou.aliyuncs.com/devflow/golang-builder:1.25.8`
- source of truth: `../devflow-control/docker/golang-builder.Dockerfile`
- supporting build script: `../devflow-control/scripts/build-golang-builder-image.sh`

Approved runtime-image choices for service packaging:
- `scratch` for fully static binaries plus staged CA certificates
- a controlled minimal runtime image published in the same Aliyun registry namespace when a service cannot run from `scratch`

If a later slice introduces another runtime base, it must be documented here and enforced by repo-local verification in the same change.

## Packaging policy

Future service packaging in this repository follows an **artifact-first** model:
1. build the service binary and any tracked runtime assets outside the final runtime image layer
2. stage those artifacts into a packaging directory
3. use a per-service Dockerfile that copies only staged artifacts into the approved runtime base

This matches the workspace pattern already used by `../devflow-control/scripts/build-staging-images.sh` together with sibling `Dockerfile.package` files.
For example, `../devflow-app-service/Dockerfile.package` shows the preferred shape: package prebuilt artifacts into `scratch` rather than compiling or installing tools inside the final image.

## Explicit bans for service Dockerfiles

Future Dockerfiles under `modules/**/Dockerfile*` must **not** perform inline package or tool installation.
That means service Dockerfiles must not contain commands such as:
- `apk add`
- `apk upgrade`
- `apt-get`
- `yum`
- `dnf`
- `go install`

The ban exists because inline installs make service packaging drift from the controlled builder/runtime catalog, hide supply-chain changes inside individual services, and break reproducible artifact-first packaging.

## Builder expectations

If a service needs Go compilation or tool generation, do that in one of these approved ways:
- inside the controlled `golang-builder:1.25.8` image
- in a repo-level build step that emits staged artifacts before packaging
- in an explicitly versioned reusable repo-local Docker asset added under `docker/` in this repository

Do **not** put service-specific `go install` or OS package installation steps directly inside a future service Dockerfile.
If a tool is needed, promote it into the controlled builder image or a shared packaging asset instead.

## Runtime expectations

A service Dockerfile should be a thin packaging layer:
- start from an approved runtime base
- copy in the prebuilt binary and tracked runtime assets
- set the non-root runtime user when required
- expose the service port
- define the final entrypoint

The Dockerfile should not become a second build system.

## Relationship to D021 and R031

S03 owns the repository-local Docker baseline required for the migration work tracked by this milestone.
This doc is the human-readable contract that supports D021's controlled-image direction and advances R031's requirement that future service packaging use repo-approved builder/runtime images and verifier-enforced policy.

## How S04 should consume this contract

When S04 adds the first real migrated service under `modules/`:
- keep the service code under its explicit owner boundary
- add a per-service Dockerfile that references the controlled image catalog from this document
- prefer artifact-first packaging, following the `Dockerfile.package` pattern from sibling repos
- do not introduce inline install commands into the service Dockerfile
- extend `scripts/verify.sh` and related Docker-policy checks if new approved image references or templates are added

## Verification surfaces

The repo-local verification entrypoint remains:

```sh
bash scripts/verify.sh
```

For S03, root docs should point readers here first when Docker policy, controlled-image references, or service-packaging expectations are in doubt.
Later in this slice, the verifier will also enforce the Docker contract files and banned inline-install patterns for any future service Dockerfiles under `modules/`.
