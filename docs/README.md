# Docs

## Purpose

Use this page as the repo-local docs landing page.
It is navigation-first and does not replace `AGENTS.md` as the startup contract.

## Canonical starts

- canonical agent start -> `AGENTS.md`
- canonical docs navigation start -> `docs/index/getting-started.md`

## Navigation order

Use these surfaces in roughly this order when navigating the doc set:

- `docs/index/getting-started.md` — human-first navigation path
- `AGENTS.md` — canonical agent startup contract
- `docs/index/agent-path.md` — compact agent route map back to `AGENTS.md`
- `docs/index/agent-recipes.md` — compact task recipes for common repo-local changes
- `docs/system/` — current repo-local execution truth
- `docs/services/` — current service ownership, ingress boundaries, diagnostics, and verification
- `docs/resources/` — current resource contracts, API behavior, and validation rules
- `docs/policies/` — durable repo rules such as layout, Docker, and verification
- `docs/generated/` — generated artifacts only
- `docs/archive/` — historical material only
- `docs/superpowers/README.md` — design specs, plans, and other pre-implementation artifacts

## Common starting points by topic

For the detailed Go monorepo directory and dependency rules used by this repo, start with:

- `docs/policies/go-monorepo-layout.md`

For structured logging, metric-label, and trace-correlation rules, start with:

- `docs/policies/observability-logging.md`

For stable HTTP error envelopes and handler error-code mapping, start with:

- `docs/policies/error-handling.md`

For shared Gin handler conventions such as pagination, response helpers, and HTTP-edge parsing, start with:

- `docs/policies/http-handler.md`

For use-case orchestration boundaries inside `internal/*/service`, start with:

- `docs/policies/service-layer.md`

For downstream runtime-boundary clients, shared HTTP client reuse, and typed downstream status handling, start with:

- `docs/policies/downstream-client.md`

For persistence ownership, repository constructor shape, and storage boundary rules, start with:

- `docs/policies/repository-layer.md`

For background execution, lease-driven worker semantics, and runtime helper boundaries, start with:

- `docs/policies/worker-runtime.md`

For resource CRUD behavior, list/filter/pagination rules, and resource-doc contract shape, start with:

- `docs/policies/resource-api.md`

For architecture and service/resource flow diagrams, start with:

- `docs/system/diagrams.md`
- `docs/system/flow-overview.md`

For runtime workload / pod display model, observer writeback, and the read-vs-action split, start with:

- `docs/system/runtime-observer.md`

## Notes

- directory `README.md` files under `docs/index/`, `docs/services/`, `docs/resources/`, `docs/policies/`, `docs/generated/`, and `docs/archive/` are orientation aids only
- the owning docs in `docs/system/`, `docs/services/`, `docs/resources/`, and `docs/policies/` hold the current facts
