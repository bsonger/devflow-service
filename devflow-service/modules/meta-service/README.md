# Meta Service

`modules/meta-service` is the first real owner-service migrated into `devflow-service` during M005/S04.
It preserves the former `devflow-app-service` boundary under the new runtime identity `meta-service` while importing shared monorepo infrastructure such as `shared/bootstrap`, `shared/observability`, `shared/routercore`, and `shared/loggingx`.

## What is real in this slice

This module now contains:
- the real Gin entrypoint at `cmd/main.go`
- the migrated API/app/domain/store/router packages under `pkg/`
- health and readiness endpoints that return `service: meta-service`
- a service-local build surface at `scripts/build.sh`
- a service-local artifact-first packaging surface at `Dockerfile`

## What is not yet promised

This slice does **not** claim that all historical runtime assets from `devflow-app-service` have already been migrated.
In particular:
- `scripts/build.sh` stages `docs/` and `config/` only if those tracked directories exist in this module
- Swagger regeneration is optional and staged into temporary build output so generated files do not change the default `go test ./...` package graph
- deployment/runtime rollout and any remaining config migration work still belong to S05 or later

## Build and package

Run the canonical service-local build from this directory:

```sh
bash scripts/build.sh
```

That command:
- builds a deterministic Linux amd64 binary at `bin/meta-service`
- stages packaging artifacts under `.build/staging/meta-service/`
- stages CA certificates under `.build/staging/_shared/certs/`
- regenerates Swagger into temporary build output only when `scripts/regen-swagger.sh` can actually run with a local `swag` CLI

The service Docker packaging surface is:

```sh
modules/meta-service/Dockerfile
```

It follows the repo-wide artifact-first contract from `docs/docker.md`: the Dockerfile starts from `scratch`, copies only staged artifacts, and uses no inline package installation.

## Diagnostics

For cold-start diagnosis, use these surfaces in order:
1. `bash scripts/build.sh`
2. `cd ../.. && bash scripts/check-docker-policy.sh`
3. `cd ../.. && bash scripts/verify.sh`
4. `pkg/router/router_test.go` for the runtime identity proof (`meta-service` on `/healthz` and `/readyz`)

If `scripts/build.sh` fails, inspect whether the root module still builds `./modules/meta-service/cmd`, whether CA certificates exist on the build host, and whether missing optional `docs/` or `config/` assets are expected for the current slice.
If Swagger generation is involved, inspect the staged output under `.build/swagger/` rather than expecting generated Go files to remain checked into the module tree.
