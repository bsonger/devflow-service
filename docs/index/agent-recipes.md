# Agent Recipes

This page is index-only.

Use it as a compact task map after reading `AGENTS.md`.
If any recipe conflicts with current repo-local docs, trust `AGENTS.md`, `docs/system/`, `docs/services/`, `docs/resources/`, and `docs/policies/` first.

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
3. the affected resource doc under `docs/resources/`
4. `docs/policies/verification.md`

Verify:
- handler and service code changed together
- the affected `docs/resources/*.md` file reflects the current request fields, routes, and validation shape
- `bash scripts/verify.sh` still passes

## Recipe: Release writeback or observer callback change

Read:
1. `AGENTS.md`
2. `docs/services/release-service.md`
3. `docs/system/release-writeback.md`
4. `docs/system/release-steps.md`
5. `docs/resources/release.md`
6. `docs/policies/verification.md`

Verify:
- token-gated writeback behavior still matches `observer.shared_token` wiring
- callback routes, accepted headers, and status normalization still match the current handlers
- step semantics still match `docs/system/release-steps.md`
- release writeback docs and resource docs were updated in the same change
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
- current facts stay in `docs/system/`, `docs/services/`, `docs/resources/`, or `docs/policies/`
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


## Recipe: Runtime read model or runtime action change

Read:
1. `AGENTS.md`
2. `docs/services/runtime-service.md`
3. `docs/resources/runtime-spec.md`
4. `docs/system/runtime-observer.md`
5. `docs/system/flow-overview.md`
6. `docs/policies/verification.md`

Verify:
- read routes still use runtime-owned observer/index state where expected
- action routes still map to explicit Kubernetes mutations only
- runtime docs and runtime service behavior changed together
- `bash scripts/verify.sh` still passes
