# Docker assets

This directory contains the repository-local Docker baseline for service packaging in `devflow-service`.
These assets are policy references, not alternative service homes.

## Purpose

Use these files when working on the active root `Dockerfile` or when a later slice adds another real service Dockerfile at an explicit service boundary.
They encode the approved builder/runtime image policy from `docs/docker.md` and provide a copyable multi-stage template that avoids inline package installation.

## Files

- `golang-builder.Dockerfile` — repo-local copy of the Go builder baseline (`golang:1.26.2-alpine3.22` plus required packages) for auditability and future local promotion
- `service.Dockerfile.template` — multi-stage template for future service boundaries; it builds the service in an approved builder image and copies only runtime artifacts into `scratch`

## Current policy

Active and future service Dockerfiles must:
- use approved FROM references only
- keep build and tool installation in the controlled builder stage
- copy only final runtime artifacts into `scratch` or another documented controlled runtime image
- avoid inline install commands such as `apk add`, `apk upgrade`, `apt-get`, `yum`, `dnf`, and `go install`

## How to use this in a future service

1. Copy `service.Dockerfile.template` into the service boundary.
2. Replace the placeholder service name, binary name, and copied source directories.
3. Keep the final image thin and runtime-only.
4. Keep the final Dockerfile thin: `FROM`, `WORKDIR`, `COPY`, `ENV`, `EXPOSE`, `USER`, `ENTRYPOINT`.
5. Run `bash scripts/verify.sh` from the repo root to confirm the contract files exist and the Docker policy checker accepts the new service Dockerfile.

## Relationship to verification

The canonical verifier remains:

```sh
bash scripts/verify.sh
```

That command now checks this directory, validates approved controlled-image references, and scans the active service Dockerfile surfaces outside `docker/` for banned inline-install patterns.
