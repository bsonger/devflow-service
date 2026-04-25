# Constraints

These constraints apply to the current `devflow-service` migration baseline.

## Hard constraints

- Do not create fake migrated code, fake binaries, or pretend verification output.
- Do not introduce `go.work` or per-service `go.mod` files during this migration.
- Do not create catch-all `common/`, `util/`, `base/`, or business-heavy `shared/` directories.
- Do not reintroduce `modules/` as a service-code location.
- Do not bury business semantics inside `internal/platform`.
- Do not let `cmd/` absorb domain logic or large amounts of module-internal wiring.
- Do not add install commands such as `apk add`, `apt-get`, `yum`, `dnf`, or `go install` to service Dockerfiles.

## Ownership constraints

- `meta-service` remains the current active service name.
- Business code should be organized by explicit domains under `internal/`.
- Infrastructure-only code belongs under `internal/platform/`.
- `internal/shared/` is optional and must stay narrow, stable, and domain-agnostic.
- Repo-local implementation truth belongs in this repo.
- Cross-repo future-state governance remains in `devflow-control`.

## Build constraints

- Keep `go.mod` at the repo root.
- Move the repo to `Go 1.26.2`.
- Align local verification, CI, builder images, and packaging rules to the same Go baseline.
- Keep Docker policy and verification aligned in the same change.

## Documentation constraints

- Keep navigation docs in `docs/index/` only.
- Keep current repo-local truth in `docs/system/`, `docs/services/`, and `docs/policies/`.
- Keep generated output in `docs/generated/` only.
- Keep historical material in `docs/archive/` only.
- Do not leave stale flat docs around as competing authorities once the layered docs exist.
