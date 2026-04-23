# modules

This directory is reserved for explicit owner-service destinations inside the future backend monorepo.

The long-term target is for app, config, network, release, and runtime code to land here as distinct module areas.
This bootstrap slice reserves the directory only and intentionally does not create fake module trees.

## Rules

- keep ownership explicit
- do not create catch-all facade modules
- do not create per-service `go.mod` files in this bootstrap task
- use later M005 slices to add the real build and module contract
