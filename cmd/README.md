# cmd

This directory is reserved for runnable process entrypoints in `devflow-service`.

Current migrated entries include `cmd/meta-service` and `cmd/release-service`.

## Intended scope

Examples of future contents include:
- service main packages
- operational admin binaries
- gateway or sync helpers that must remain runnable entrypoints

## Out of scope

Do not place domain logic here.
`cmd/` is for process entrypoints only.
