# Image

## Ownership

- active owning boundary: `release-service`
- primary persisted record: `Manifest`
- domain packages:
  - `internal/manifest/domain`
  - `internal/release/domain`

## Current status

`Image` is not a standalone database-backed resource in the current repo-local contract.
The current system records build output on `Manifest`, then consumes that immutable image reference during release and runtime rendering.

## Source of truth

The current image-related fields live on `Manifest`:

- `image_ref`
- `image_tag`
- `image_digest`
- `repo_address`
- `commit_hash`
- `git_revision`

Release-time rendering consumes `manifest.image_ref` and does not read a separate `Image` resource.

## API surface

There is no standalone `Image` CRUD API in the current repo-local contract.
Clients should use:

- `docs/resources/manifest.md` for build output and image metadata
- `docs/resources/release.md` for deploy-time consumption of the built image

## Validation notes

- image metadata is produced by manifest build flow, not by direct user CRUD
- an image without digest or tag is not deployable
- release/runtime rendering should consume the frozen manifest snapshot rather than reconstructing image identity from ad-hoc inputs

## Source pointers

- manifest domain: `internal/manifest/domain/manifest.go`
- manifest service: `internal/manifest/service/manifest.go`
- manifest renderer: `internal/manifest/service/manifest_renderer.go`
- release bundle rendering: `internal/release/service/release_bundle.go`
