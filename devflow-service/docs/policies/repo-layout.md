# Repo Layout Policy

This document defines the durable repo-local layout policy for `devflow-service`.

## Root shape

The root layout is:
- `cmd/`
- `internal/`
- `api/`
- `deployments/`
- `test/`
- `docs/`
- `scripts/`

## Code layout rules

- `cmd/<service>/main.go` is entrypoint-only.
- `internal/platform/` is infrastructure-only.
- `internal/<domain>/domain` holds domain objects and rules.
- `internal/<domain>/service` holds use-case orchestration.
- `internal/<domain>/repository` holds persistence interfaces and implementations.
- `internal/<domain>/transport` holds protocol adapters.
- `internal/<domain>/module.go` wires one domain together.

## Explicit bans

- no catch-all `shared/`
- no catch-all `common/`
- no catch-all `util/`
- no `modules/` service-code home

Prefer explicit duplication over premature shared abstractions.
