# Agent Recipes

This page is index-only.

Use it as a compact task map after reading `AGENTS.md`.
If any recipe conflicts with current repo-local docs, trust `AGENTS.md`, `docs/system/`, `docs/services/`, and `docs/policies/` first.

## Recipe: Repo-local implementation change

Read:
1. `AGENTS.md`
2. `docs/system/recovery.md`
3. `docs/system/architecture.md`
4. the smallest affected service or policy doc

Verify:
- code and docs still point at the same root layout
- no catch-all `common/`, `util/`, or business-heavy `shared/` directories were reintroduced

## Recipe: API or handler change

Read:
1. `AGENTS.md`
2. `docs/services/meta-service.md`
3. the affected resource doc under `docs/services/`
4. `docs/policies/verification.md`

Verify:
- handler and service code changed together
- the affected `docs/services/*.md` file reflects the current request fields, routes, and validation shape
- `bash scripts/verify.sh` still passes

## Recipe: Docker, image, or CI change

Read:
1. `AGENTS.md`
2. `docs/policies/docker-baseline.md`
3. `docs/policies/verification.md`
4. `scripts/README.md`

Verify:
- service Dockerfiles remain thin multi-stage builds
- install behavior stays in controlled base images, not service Dockerfiles
- `bash scripts/check-docker-policy.sh`
- `bash scripts/verify.sh`

## Recipe: Doc governance change

Read:
1. `docs/README.md`
2. `docs/index/README.md`
3. the smallest current authoritative doc implicated by the change

Verify:
- index docs stay navigation-only
- current facts stay in `docs/system/`, `docs/services/`, or `docs/policies/`
- no migrated content overrides current repo-local truth

## Recipe: Handoff or completion

Read:
1. `AGENTS.md`
2. `docs/system/recovery.md`
3. `docs/policies/verification.md`

Verify:
- rerun the repo-local verification stack
- confirm docs and code still describe the same layout
- confirm the active service still builds as `meta-service`
