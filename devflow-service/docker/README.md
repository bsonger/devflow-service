# Docker assets

This directory contains the repository-local Docker baseline for future service packaging in `devflow-service`.
These assets are shared contract surfaces, not a claim that migrated services already exist under `modules/` today.

## Purpose

Use these files when a later slice adds a real service Dockerfile under `modules/`.
They encode the approved builder/runtime image policy from `docs/docker.md` and provide a copyable artifact-first template that avoids inline package installation.

## Files

- `golang-builder.Dockerfile` — repo-local copy of the controlled Go builder baseline (`registry.cn-hangzhou.aliyuncs.com/devflow/golang-builder:1.25.8` source shape) for auditability and future local promotion
- `service.Dockerfile.template` — packaging-only template for future service boundaries; it copies prebuilt artifacts into an approved runtime base and bans inline install steps by structure

## Current policy

Future service Dockerfiles under `modules/**/Dockerfile*` must:
- use approved FROM references only
- keep build and tool installation in the controlled builder or an earlier staged build pipeline
- package prebuilt artifacts into `scratch` or another documented controlled runtime image
- avoid inline install commands such as `apk add`, `apk upgrade`, `apt-get`, `yum`, `dnf`, and `go install`

## How to use this in a future service

1. Build the service binary outside the final packaging image.
2. Stage the binary and any tracked runtime assets under a predictable artifacts directory.
3. Copy `service.Dockerfile.template` into the service boundary and replace the placeholder artifact paths and entrypoint.
4. Keep the final Dockerfile thin: `FROM`, `WORKDIR`, `COPY`, `ENV`, `EXPOSE`, `USER`, `ENTRYPOINT`.
5. Run `bash scripts/verify.sh` from the repo root to confirm the contract files exist and the Docker policy checker accepts the new service Dockerfile.

## Relationship to verification

The canonical verifier remains:

```sh
bash scripts/verify.sh
```

That command now checks this directory, validates approved controlled-image references, and scans service Dockerfiles under `modules/` for banned inline-install patterns.
