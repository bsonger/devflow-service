# Policies

This directory is policy-only.

Read files here when the current task needs a durable repo rule rather than a service-specific or recovery-specific explanation.

Current policy docs:
- `go-monorepo-layout.md` — detailed Go monorepo structure and naming rules for this repo
- `docker-baseline.md` — Docker and image-build rules
- `verification.md` — canonical verification stack
- `observability-logging.md` — structured logging, metrics-label, and trace-correlation rules
- `error-handling.md` — HTTP error envelope, code vocabulary, and handler mapping rules
- `http-handler.md` — Gin handler responsibilities, shared response helpers, and pagination rules
- `service-layer.md` — service orchestration boundaries, cross-domain coordination, and non-HTTP rules
- `downstream-client.md` — runtime-boundary client rules, shared downstream HTTP reuse, and typed status handling
- `repository-layer.md` — persistence ownership, repository constructor shape, and storage boundary rules
- `worker-runtime.md` — background execution, lease/claim semantics, and runtime helper boundaries
- `resource-api.md` — list/filter/pagination, soft-delete semantics, and resource doc contract rules
- `api-compatibility.md` — API evolution and compatibility expectations
- `api-directory.md` — `api/` contract layer rules and OpenAPI generation policy
- `new-service-rule.md` — new service and repo-baseline reference rules

If a policy description conflicts with current implementation-facing docs, resolve that conflict instead of guessing.
