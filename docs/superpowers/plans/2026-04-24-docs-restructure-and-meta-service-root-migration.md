# Docs Restructure And Meta-Service Root Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reorganize repo-local docs into a layered structure, update the repo contract to the new root layout, and migrate `meta-service` from `modules/meta-service` into root `cmd/` and `internal/` paths without changing service behavior.

**Architecture:** First move the documentation system from a flat layout into `docs/index`, `docs/system`, `docs/services`, and `docs/policies`, while making `AGENTS.md` a small routing contract. Then update verification, Docker, and CI contracts to the new `Go 1.26.2` and base-image-only installation model. Finally move `meta-service` code and imports into the root layout, keeping behavior stable and proving the result with formatting, vet, lint, tests, build, and Docker build.

**Tech Stack:** Go 1.26.2, Gin, golangci-lint, Docker, shell verification scripts, Markdown docs

---

## File map

### Documentation surfaces

- Modify: `AGENTS.md`
- Modify: `README.md`
- Replace: `docs/README.md`
- Move and rewrite: `docs/recovery.md` -> `docs/system/recovery.md`
- Move and rewrite: `docs/architecture.md` -> `docs/system/architecture.md`
- Move and rewrite: `docs/constraints.md` -> `docs/system/constraints.md`
- Move and rewrite: `docs/observability.md` -> `docs/system/observability.md`
- Move and rewrite: `docs/docker.md` -> `docs/policies/docker-baseline.md`
- Create: `docs/index/README.md`
- Create: `docs/index/getting-started.md`
- Create: `docs/index/agent-path.md`
- Create: `docs/services/meta-service.md`
- Create: `docs/policies/repo-layout.md`
- Create: `docs/policies/verification.md`
- Create: `docs/generated/README.md`
- Create: `docs/archive/README.md`

### Verification and packaging surfaces

- Modify: `scripts/README.md`
- Modify: `scripts/verify.sh`
- Modify: `scripts/check-docker-policy.sh`
- Modify: `go.mod`
- Modify: `docker/README.md`
- Modify: `docker/golang-builder.Dockerfile`
- Modify: `docker/service.Dockerfile.template`
- Modify: `modules/meta-service/Dockerfile` until the root `Dockerfile` replaces it
- Modify: `modules/meta-service/scripts/build.sh` until replaced or removed
- Create: `Dockerfile`
- Create or modify: `Makefile`
- Create or modify: CI config once the repo exposes one

### Service code movement

- Move: `modules/meta-service/cmd/main.go` -> `cmd/meta-service/main.go`
- Move and split: `modules/meta-service/pkg/api/*` -> `internal/<domain>/transport/http/*`
- Move and split: `modules/meta-service/pkg/app/*` -> `internal/<domain>/application/*`
- Move and split: `modules/meta-service/pkg/domain/*` -> `internal/<domain>/domain/*`
- Move and split: `modules/meta-service/pkg/store/*` and `modules/meta-service/pkg/infra/store/*` -> `internal/<domain>/repository/*`
- Move: `modules/meta-service/pkg/infra/config/*` -> `internal/platform/config/*`
- Move and split: `modules/meta-service/pkg/router/*` -> domain transport wiring plus root service assembly
- Move: `shared/httpx/*` -> `internal/platform/httpx/*`
- Move: `shared/loggingx/*` -> `internal/platform/logger/*`
- Move: `shared/otelx/*` -> `internal/platform/otel/*`
- Move: `shared/observability/*`, `shared/pyroscopex/*`, and `shared/bootstrap/*` -> `internal/platform/runtime/*`

### Test and proof surfaces

- Modify: tests that import old `modules/meta-service/...` or `shared/...` paths
- Verify: `gofmt ./...`
- Verify: `go vet ./...`
- Verify: `golangci-lint run`
- Verify: `go test ./...`
- Verify: `go build -o bin/meta-service ./cmd/meta-service`
- Verify: `docker build`

## Task 1: Restructure doc directories and startup surfaces

**Files:**
- Modify: `AGENTS.md`
- Modify: `README.md`
- Replace: `docs/README.md`
- Create: `docs/index/README.md`
- Create: `docs/index/getting-started.md`
- Create: `docs/index/agent-path.md`
- Create: `docs/system/recovery.md`
- Create: `docs/system/architecture.md`
- Create: `docs/system/constraints.md`
- Create: `docs/system/observability.md`
- Create: `docs/services/meta-service.md`
- Create: `docs/policies/repo-layout.md`
- Create: `docs/policies/docker-baseline.md`
- Create: `docs/policies/verification.md`
- Create: `docs/generated/README.md`
- Create: `docs/archive/README.md`

- [ ] **Step 1: Create the new docs directory tree**

Run:

```bash
mkdir -p docs/index docs/system docs/services docs/policies docs/generated docs/archive docs/superpowers/plans docs/superpowers/specs
```

Expected: directories exist with no output.

- [ ] **Step 2: Rewrite `AGENTS.md` as the canonical startup contract**

Update `AGENTS.md` so it keeps only:
- startup purpose
- minimal reading set
- authority ladder
- task-intent routing
- current migration focus
- short summary of target repo layout and verification expectations

It must point agents to `docs/index/*`, `docs/system/*`, `docs/services/*`, and `docs/policies/*` instead of restating all durable rules inline.

- [ ] **Step 3: Write the docs landing and navigation files**

Write:
- `docs/README.md` as the docs landing page
- `docs/index/README.md` as an index-only explainer
- `docs/index/getting-started.md` as the human-first navigation path
- `docs/index/agent-path.md` as the compact agent routing page back to `AGENTS.md`

These files must be navigation-only and must not become competing sources of implementation truth.

- [ ] **Step 4: Move flat system docs into `docs/system/`**

Rewrite these files under the new paths:
- `docs/system/recovery.md`
- `docs/system/architecture.md`
- `docs/system/constraints.md`
- `docs/system/observability.md`

Each file should describe current repo-local truth, not future-state aspirations.

- [ ] **Step 5: Split durable rules into policy docs**

Write:
- `docs/policies/repo-layout.md`
- `docs/policies/docker-baseline.md`
- `docs/policies/verification.md`

These files should own the stable rules that were previously mixed into flat docs and `AGENTS.md`.

- [ ] **Step 6: Create the service-level current-state doc**

Write `docs/services/meta-service.md` as the current service description covering:
- current service identity
- current build/package surfaces
- current migration target
- repo-local diagnostics

- [ ] **Step 7: Add generated/archive placeholders**

Write:
- `docs/generated/README.md`
- `docs/archive/README.md`

Both files should state that their directories are category-only and not current sources of truth.

- [ ] **Step 8: Remove or replace old flat docs**

Delete or reduce the old flat files after their replacements exist:
- `docs/recovery.md`
- `docs/architecture.md`
- `docs/constraints.md`
- `docs/observability.md`
- `docs/docker.md`

If keeping redirect stubs is necessary, make them point to the new owning files instead of duplicating content.

- [ ] **Step 9: Verify internal doc links and navigation**

Run:

```bash
rg -n "docs/(recovery|architecture|constraints|observability|docker)\\.md|modules/meta-service|shared/" AGENTS.md README.md docs scripts
```

Expected: only intentional historical references remain.

## Task 2: Update verification and Docker contract to the new baseline

**Files:**
- Modify: `scripts/README.md`
- Modify: `scripts/verify.sh`
- Modify: `scripts/check-docker-policy.sh`
- Modify: `go.mod`
- Modify: `docker/README.md`
- Modify: `docker/golang-builder.Dockerfile`
- Modify: `docker/service.Dockerfile.template`
- Create: `Dockerfile`
- Modify: `Makefile`

- [ ] **Step 1: Upgrade the Go baseline**

Update `go.mod` from `go 1.25.8` to `go 1.26.2`.

- [ ] **Step 2: Rewrite Docker policy docs**

Update Docker docs so they explicitly state:
- controlled builder/runtime base images use `Go 1.26.2`
- service Dockerfiles may not install packages or tools
- install behavior belongs in controlled base images only

- [ ] **Step 3: Update verification docs**

Rewrite `scripts/README.md` and the verification policy doc so the canonical proof stack is:

```bash
gofmt ./...
go vet ./...
golangci-lint run
go test ./...
go build -o bin/meta-service ./cmd/meta-service
docker build
```

- [ ] **Step 4: Update `scripts/verify.sh`**

Change the verifier to assert:
- new docs paths exist
- `AGENTS.md` references the new doc structure
- `Go 1.26.2` is declared
- `cmd/meta-service/main.go` exists
- `internal/platform/*` and domain directories exist once migrated
- service Dockerfiles have no install commands

- [ ] **Step 5: Update `scripts/check-docker-policy.sh`**

Add or preserve checks for:
- banned install commands
- approved base images
- packaging-only Dockerfile behavior

- [ ] **Step 6: Add or update `Makefile`**

Expose at least:

```make
APP ?= meta-service

fmt:
	go fmt ./...

vet:
	go vet ./...

test:
	go test ./...

build:
	go build -o bin/$(APP) ./cmd/$(APP)

lint:
	golangci-lint run
```

- [ ] **Step 7: Create the root `Dockerfile` contract**

Write the root `Dockerfile` as the canonical packaging surface for `meta-service` once the service has moved to the root layout.

## Task 3: Move `meta-service` entrypoint and platform helpers to the root layout

**Files:**
- Create: `cmd/meta-service/main.go`
- Create: `internal/platform/config/*`
- Create: `internal/platform/logger/*`
- Create: `internal/platform/otel/*`
- Create: `internal/platform/httpx/*`
- Create: `internal/platform/runtime/*`
- Move or remove: `modules/meta-service/cmd/main.go`
- Move or remove: `shared/*`

- [ ] **Step 1: Move the main package**

Move `modules/meta-service/cmd/main.go` to `cmd/meta-service/main.go` and update imports to the new root package paths.

- [ ] **Step 2: Create `internal/platform` subpackages**

Create:
- `internal/platform/config`
- `internal/platform/logger`
- `internal/platform/otel`
- `internal/platform/httpx`
- `internal/platform/runtime`

Move infrastructure-only code into those packages with no business semantics mixed in.

- [ ] **Step 3: Fix all imports from `shared/*`**

Replace imports from:
- `shared/httpx`
- `shared/loggingx`
- `shared/otelx`
- `shared/pyroscopex`
- `shared/observability`
- `shared/bootstrap`

with the corresponding `internal/platform/*` packages.

- [ ] **Step 4: Run compile proof after platform migration**

Run:

```bash
go test ./...
```

Expected: compile errors should now point only at remaining old `modules/meta-service/pkg/*` references.

## Task 4: Split `meta-service` business code into explicit domains

**Files:**
- Create: `internal/project/**`
- Create: `internal/environment/**`
- Create: `internal/cluster/**`
- Move or remove: `modules/meta-service/pkg/api/*`
- Move or remove: `modules/meta-service/pkg/app/*`
- Move or remove: `modules/meta-service/pkg/domain/*`
- Move or remove: `modules/meta-service/pkg/store/*`
- Move or remove: `modules/meta-service/pkg/infra/store/*`
- Move or remove: `modules/meta-service/pkg/router/*`
- Move or remove: `modules/meta-service/pkg/model/*`

- [ ] **Step 1: Create domain directory skeletons**

Create for each of `project`, `environment`, and `cluster`:
- `application/`
- `domain/`
- `repository/`
- `transport/http/`
- `module.go`

- [ ] **Step 2: Move domain types and rules**

Move current business entities and domain errors into the correct `internal/<domain>/domain` packages.
Do not keep a generic `model` package.

- [ ] **Step 3: Move use-case orchestration**

Move current orchestration code into `internal/<domain>/application`.

- [ ] **Step 4: Move repository interfaces and implementations**

Move current store code into `internal/<domain>/repository`.
Keep storage concerns out of `application`.

- [ ] **Step 5: Move HTTP handlers and DTOs**

Move current API handlers into `internal/<domain>/transport/http`.
Move request and response DTOs there as well.

- [ ] **Step 6: Replace router assembly with domain modules**

Move routing and assembly responsibilities into:
- `internal/<domain>/module.go`
- root service composition that registers each domain's HTTP routes

- [ ] **Step 7: Delete old service package tree once imports are clean**

Remove the old `modules/meta-service/pkg` tree only after `go test ./...` proves there are no remaining imports from it.

## Task 5: Replace old build/package paths and land the final proof

**Files:**
- Modify: `scripts/verify.sh`
- Modify: `scripts/README.md`
- Modify: `README.md`
- Modify: `docs/system/recovery.md`
- Modify: `docs/system/observability.md`
- Modify: `docs/services/meta-service.md`
- Modify: `Dockerfile`
- Modify: CI files
- Delete: `modules/meta-service/scripts/build.sh` once replaced
- Delete: `modules/meta-service/Dockerfile` once replaced
- Delete: `modules/meta-service/README.md` once replaced

- [ ] **Step 1: Replace service build references**

Change all docs and scripts that still refer to:
- `modules/meta-service/scripts/build.sh`
- `modules/meta-service/Dockerfile`
- `modules/meta-service/README.md`

so they point to the new root layout.

- [ ] **Step 2: Replace or remove the old service-local build script**

Either:
- move the script contract into a root build command, or
- keep a renamed root-level helper under `scripts/`

Do not leave the old nested path as the canonical build entrypoint.

- [ ] **Step 3: Update CI**

Make CI run:

```bash
go fmt ./...
go vet ./...
golangci-lint run
go test ./...
go build -o bin/meta-service ./cmd/meta-service
docker build
```

- [ ] **Step 4: Run the full verification stack**

Run:

```bash
go fmt ./...
go vet ./...
golangci-lint run
go test ./...
go build -o bin/meta-service ./cmd/meta-service
docker build
bash scripts/verify.sh
```

Expected: all commands pass with the new root layout.

- [ ] **Step 5: Commit the migration in logical slices**

Suggested commit sequence:

```bash
git commit -m "docs: reorganize repo docs into layered structure"
git commit -m "build: upgrade go and docker contract to 1.26.2"
git commit -m "refactor: move meta-service entrypoint and platform packages"
git commit -m "refactor: split meta-service into root domain packages"
git commit -m "ci: align verification and packaging with root layout"
```

## Self-review

Spec coverage:
- repo root layout: covered by Tasks 1, 3, 4, and 5
- docs layering: covered by Task 1
- `meta-service` root migration: covered by Tasks 3, 4, and 5
- Go 1.26.2 and base-image installation rules: covered by Task 2
- verification and CI contract: covered by Tasks 2 and 5

Placeholder scan:
- no `TODO`, `TBD`, or deferred placeholders remain in the task list

Type consistency:
- service name is consistently `meta-service`
- root build target is consistently `./cmd/meta-service`
- infrastructure destination is consistently `internal/platform/*`

## Follow-up backlog

- manifest/image ownership migration:
  - move image packaging ownership into the manifest flow
  - after image handling is absorbed by manifest, remove the now-obsolete legacy manifest path/logic that existed before the migration
  - keep manifest responsible for freezing deployment inputs after image packaging completes
  - manifest snapshots must include at least:
    - `workload_config_snapshot`
    - `services_snapshot`

### Suggested implementation checklist

1. manifest becomes the packaging entry
   - make manifest creation the place that resolves the deployable image reference or artifact
   - define one authoritative manifest input contract for image selection and packaging output
   - ensure release creation consumes the manifest-owned packaged result instead of re-deriving image packaging state elsewhere

2. collapse legacy manifest logic
   - identify the pre-migration manifest path that overlaps with the new manifest-owned packaging flow
   - delete duplicate assembly logic once the manifest-owned path produces equivalent output
   - keep only one manifest rendering and artifact publishing path in `internal/manifest`

3. freeze snapshots during manifest creation
   - persist `workload_config_snapshot`
   - persist `services_snapshot`
   - keep existing route/app-config snapshots aligned with the same freeze point when still needed by rendering
   - make snapshot capture happen before release dispatch so later resource changes do not mutate the already-created manifest

4. move the remaining old manifest responsibilities to release where appropriate
   - migrate the legacy manifest-side freeze logic for:
     - `app_config_snapshot`
     - `routes_snapshot`
   - make `release` the owner of the final frozen deployment payload assembly boundary when that better matches the current runtime contract
   - keep manifest/release ownership explicit so snapshot freeze does not happen twice in competing paths

5. YAML packaging and OCI upload contract
   - the frozen rendered YAML must be packaged as an OCI artifact
   - the OCI upload step and resulting artifact reference must be part of the manifest/release-owned contract, not an implicit side effect
   - docs and code should clearly state which layer:
     - renders YAML
     - packages YAML
     - uploads OCI
     - stores the final artifact reference

### Proposed ownership boundary

| Concern | Owner | Notes |
|---|---|---|
| resolve image input | `manifest` | manifest entry decides which deployable image/artifact is being packaged |
| freeze `workload_config_snapshot` | `manifest` | frozen at manifest creation time |
| freeze `services_snapshot` | `manifest` | frozen at manifest creation time |
| freeze `app_config_snapshot` | `release` | migrated from old manifest-side logic |
| freeze `routes_snapshot` | `release` | migrated from old manifest-side logic |
| render deployment YAML | `manifest` | rendering contract should stay singular |
| package rendered YAML | `manifest` | produces the OCI payload boundary |
| upload OCI artifact | `manifest` | upload should not be hidden in a second path |
| persist artifact reference/digest | `manifest` | stored on the manifest record |
| consume frozen artifact for rollout | `release` | release should consume, not rebuild |
| execute runtime rollout / writeback | `release` | release remains rollout owner |

Boundary rule:
- `manifest` should own packaging-time assembly and OCI artifact publication
- `release` should own rollout-time freeze responsibilities that remain outside the manifest packaging boundary
- no resource should be frozen twice by both layers for the same purpose

### Likely code touchpoints

Manifest-owned work will likely land in:
- `internal/manifest/service/manifest.go`
- `internal/manifest/service/manifest_artifact.go`
- `internal/manifest/service/manifest_renderer.go`
- `internal/manifest/domain/manifest.go`
- `internal/manifest/repository/repository.go`
- `internal/manifest/transport/http/manifest_handler.go`
- `docs/resources/manifest.md`

Release-owned migrated freeze work will likely land in:
- `internal/release/service/release.go`
- `internal/release/domain/release.go`
- `internal/release/repository/repository.go`
- `internal/release/transport/http/release_handler.go`
- `docs/resources/release.md`
- `docs/services/release-service.md`

Downstream readers affected by the boundary split will likely land in:
- `internal/appconfig/transport/downstream/config_manifest.go`
- `internal/appservice/transport/downstream/service.go`
- `internal/release/support/deploy_target.go`

Verification and contract sync will likely land in:
- `internal/manifest/service/*_test.go`
- `internal/release/service/*_test.go`
- `internal/release/transport/http/*_test.go`
- `api/openapi/docs.go`
- `api/openapi/swagger.json`
- `api/openapi/swagger.yaml`

### Suggested execution phases

#### Phase 1 — make ownership explicit without changing rollout behavior
- rename and document the intended boundaries first
- ensure manifest remains the only YAML render + OCI publish path
- add or update tests that lock current packaged artifact behavior

Exit gate:
- one code path renders YAML
- one code path publishes OCI
- docs clearly say who owns what

#### Phase 2 — migrate freeze responsibilities
- move `app_config_snapshot` freeze logic into `release`
- move `routes_snapshot` freeze logic into `release`
- keep `workload_config_snapshot` and `services_snapshot` in `manifest`
- prove no duplicate freeze happens in both layers

Exit gate:
- manifest and release each freeze only their assigned resources
- test coverage shows deterministic snapshot capture

#### Phase 3 — remove obsolete legacy manifest logic
- delete superseded assembly branches
- remove stale fields/helpers that only existed for the old path
- simplify docs and handlers to the new single flow

Exit gate:
- no dead alternate manifest flow remains
- code and docs describe the same single path

#### Phase 4 — final contract hardening
- regenerate OpenAPI
- re-check docs/resources and service docs
- verify release consumes frozen manifest artifact instead of rebuilding mutable state

Exit gate:
- `go test ./...`
- `bash scripts/verify.sh`
- no doc/API drift remains

6. tighten API and docs contract
   - document that manifest owns the packaged deployment artifact
   - document which upstream resources are snapshotted into the manifest
   - document which legacy freeze responsibilities moved to `release`
   - document that rendered YAML is packaged and uploaded as OCI
   - remove docs that imply image packaging and manifest freezing are separate user-facing workflows if that is no longer true

7. verification expectations
   - add tests proving manifest creation captures the expected snapshots
   - add tests proving release-side migrated freeze logic still captures `appconfig` and `route` deterministically
   - add tests proving rendered YAML is packaged and uploaded as OCI with the stored artifact reference
   - add tests proving release uses manifest-frozen data rather than reloading mutable live config
   - keep `docs/resources/manifest.md`, `internal/manifest/service`, and OpenAPI generated artifacts in sync

### Acceptance target

- one clear flow: image input -> manifest packaging -> snapshot freeze -> release consume
- no duplicated old/new manifest assembly paths remain
- manifest record is sufficient to replay or inspect the frozen deployment payload without re-querying mutable live config for the core packaged result
- release-owned migrated freeze responsibilities are explicit and non-duplicated
- rendered YAML OCI packaging/upload ownership is explicit in both code and docs
