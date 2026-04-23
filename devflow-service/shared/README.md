# shared

This directory holds infrastructure-only code shared across future backend modules.

As of M005/S02, the first real extracted packages live here:
- `httpx`
- `loggingx`
- `otelx`
- `pyroscopex`
- `observability`
- `routercore`
- `bootstrap`

These packages were absorbed from `devflow-service-common` so later service migrations can retarget imports to the monorepo instead of an external common repository.

## Boundary rule

`shared/` must not become a hidden owner layer.
If code carries app, config, network, release, or runtime ownership semantics, it belongs with the owning module instead.

The allowed use here is infrastructure: bootstrap, transport helpers, router middleware, and observability plumbing that multiple future modules can import without transferring business ownership.
