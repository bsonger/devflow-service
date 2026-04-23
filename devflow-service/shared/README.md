# shared

This directory is reserved for infrastructure-only code shared across future backend modules.

It is the future landing zone for common bootstrap, transport, routing, and observability helpers absorbed from existing shared backend repos.
This bootstrap slice reserves the location only.

## Boundary rule

`shared/` must not become a hidden owner layer.
If code carries app, config, network, release, or runtime ownership semantics, it belongs with the owning module instead.
