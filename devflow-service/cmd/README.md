# cmd

This directory is reserved for runnable process entrypoints in `devflow-service`.

Future slices should place service mains and operational binaries here once real migration begins.
This bootstrap slice reserves the location only; it does not create placeholder binaries or fake service commands.

## Intended scope

Examples of future contents include:
- service main packages
- operational admin binaries
- gateway or sync helpers that must remain runnable entrypoints

## Out of scope

Do not place domain logic here.
`cmd/` is for process entrypoints only.
