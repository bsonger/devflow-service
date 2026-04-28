# Image

## Ownership

- active service boundary: `release-service`
- runnable host process: `release-service`
- primary persisted record: `Manifest`
- domain packages:
  - `internal/manifest/domain`
  - `internal/release/domain`

## Purpose

`Image` is a derived contract, not a standalone database-backed resource in the current repo-local design.
The current system records build output on `Manifest`, then consumes that immutable image reference during release-time rendering and rollout.

## Field table

There is no standalone `Image` table or public CRUD payload today.
The current image-related fields live on `Manifest`:

| Field | Source | Description |
|---|---|---|
| `image_ref` | `Manifest` | workload image reference used for deployment |
| `image_tag` | `Manifest` | human-friendly image tag |
| `image_digest` | `Manifest` | immutable image digest |
| `repo_address` | `Manifest` | source repository address |
| `commit_hash` | `Manifest` | resolved source commit |
| `git_revision` | `Manifest` | requested source selector |

## API surface

There is no standalone `Image` CRUD API in the current repo-local contract.
Use these docs instead:

- `docs/resources/manifest.md` for build output and image metadata
- `docs/resources/release.md` for deploy-time consumption of the built image

## Create / update rules

- clients do not create or update `Image` directly
- image metadata is produced by manifest build execution
- release-time rendering consumes frozen manifest image output rather than a separate image resource

## Validation notes

- image metadata is produced by manifest build flow, not by direct user CRUD
- an image without digest or tag is not deployable
- release/runtime rendering should consume frozen manifest data rather than reconstructing image identity from ad-hoc inputs

## Source pointers

- manifest domain: `internal/manifest/domain/manifest.go`
- manifest service: `internal/manifest/service/manifest.go`
- manifest renderer: `internal/manifest/service/manifest_renderer.go`
- release bundle rendering: `internal/release/service/release_bundle.go`
