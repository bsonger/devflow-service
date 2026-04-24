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
- `cmd/<service>/main.go` should only load config, initialize platform dependencies, assemble modules, and start the server.
- `internal/platform/` is infrastructure-only.
- `internal/shared/` is optional and must stay small, stable, and domain-agnostic.
- `internal/<domain>/domain` holds domain objects and rules.
- `internal/<domain>/service` holds use-case orchestration.
- `internal/<domain>/repository` holds persistence interfaces and implementations.
- `internal/<domain>/transport` holds protocol adapters.
- `internal/<domain>/module.go` wires one domain together.

## Shared code rules

- Allowed examples under `internal/shared/`: `errs`, `response`, `middleware`, `idgen`
- Shared code must not contain business-domain models or service-private behavior.
- Copy once before creating a shared abstraction.
- Only extract to `internal/shared/` after repeated, stable duplication is clear.

## Dependency direction

- `cmd` depends on module wiring and platform initialization.
- `transport` depends on `service`.
- `service` depends on `domain` and repository interfaces.
- `repository` depends on `domain` and `internal/platform`.
- `internal/platform` must not depend on business domains.
- `internal/shared` must not depend on business domains.

## Explicit bans

- no catch-all `common/`
- no catch-all `util/`
- no catch-all `base/`
- no `modules/` service-code home
- no direct cross-domain imports of another domain's internal implementation as a reuse shortcut

Prefer explicit duplication over premature shared abstractions.
