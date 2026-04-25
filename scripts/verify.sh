#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GO_CACHE_DIR="${GOCACHE:-$ROOT_DIR/.cache/go-build}"

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

info() {
  echo "INFO: $*"
}

run_docker_policy_check() {
  info "Running Docker policy checks"
  (
    cd "$ROOT_DIR"
    bash scripts/check-docker-policy.sh
  )
}

require_file() {
  local path="$1"
  local label="$2"
  [[ -f "$path" ]] || fail "$label is missing: $path"
  [[ -s "$path" ]] || fail "$label is empty: $path"
}

require_dir() {
  local path="$1"
  local label="$2"
  [[ -d "$path" ]] || fail "$label is missing: $path"
}

require_literal() {
  local path="$1"
  local label="$2"
  local needle="$3"
  grep -Fq "$needle" "$path" || fail "$label missing from $(basename "$path"): expected literal [$needle]"
}

run_go_test() {
  info "Running go test ./..."
  (
    cd "$ROOT_DIR"
    mkdir -p "$GO_CACHE_DIR"
    GOCACHE="$GO_CACHE_DIR" go test ./...
  )
}

run_go_build_targets() {
  info "Building root service entrypoints"
  (
    cd "$ROOT_DIR"
    mkdir -p bin
    GOCACHE="$GO_CACHE_DIR" go build -o bin/meta-service ./cmd/meta-service
    GOCACHE="$GO_CACHE_DIR" go build -o bin/release-service ./cmd/release-service
  )
}

info "Checking repository-local root-module and recovery surfaces"
require_file "$ROOT_DIR/go.mod" "root go.mod"
require_file "$ROOT_DIR/.gitignore" "root .gitignore"
require_file "$ROOT_DIR/README.md" "root README"
require_file "$ROOT_DIR/AGENTS.md" "root AGENTS"
require_file "$ROOT_DIR/.github/workflows/ci.yml" "repo CI workflow"
require_file "$ROOT_DIR/docs/README.md" "docs index"
require_file "$ROOT_DIR/docs/index/README.md" "docs index README"
require_file "$ROOT_DIR/docs/index/getting-started.md" "docs getting started"
require_file "$ROOT_DIR/docs/index/agent-path.md" "docs agent path"
require_file "$ROOT_DIR/docs/system/architecture.md" "system architecture doc"
require_file "$ROOT_DIR/docs/system/constraints.md" "system constraints doc"
require_file "$ROOT_DIR/docs/system/observability.md" "system observability doc"
require_file "$ROOT_DIR/docs/system/recovery.md" "system recovery doc"
require_file "$ROOT_DIR/docs/services/meta-service.md" "meta-service service doc"
require_file "$ROOT_DIR/docs/services/release-service.md" "release-service service doc"
require_file "$ROOT_DIR/docs/policies/go-monorepo-layout.md" "go monorepo layout policy doc"
require_file "$ROOT_DIR/docs/policies/docker-baseline.md" "docker baseline policy doc"
require_file "$ROOT_DIR/docs/policies/verification.md" "verification policy doc"
require_file "$ROOT_DIR/scripts/README.md" "scripts README"
require_file "$ROOT_DIR/scripts/verify.sh" "repo-local verifier"
require_file "$ROOT_DIR/scripts/docker-build.sh" "repo-local docker build wrapper"
require_file "$ROOT_DIR/scripts/regen-swagger.sh" "meta-service swagger regen script"
require_file "$ROOT_DIR/scripts/check-docker-policy.sh" "Docker policy checker"
require_file "$ROOT_DIR/scripts/check-docker-policy_test.sh" "Docker policy checker test"
require_file "$ROOT_DIR/docs/docker.md" "Docker redirect doc"
require_dir "$ROOT_DIR/docker" "docker assets directory"
require_file "$ROOT_DIR/docker/README.md" "docker assets README"
require_file "$ROOT_DIR/docker/golang-builder.Dockerfile" "repo-local golang builder Dockerfile"
require_file "$ROOT_DIR/docker/service.Dockerfile.template" "service Dockerfile template"
require_dir "$ROOT_DIR/docs/generated" "generated docs directory"
require_dir "$ROOT_DIR/docs/archive" "archive docs directory"

require_dir "$ROOT_DIR/cmd" "cmd directory"
require_dir "$ROOT_DIR/internal" "internal directory"
require_dir "$ROOT_DIR/api" "api directory"
require_dir "$ROOT_DIR/deployments" "deployments directory"
require_dir "$ROOT_DIR/test" "test directory"
require_dir "$ROOT_DIR/gateway" "gateway directory"

require_dir "$ROOT_DIR/internal/app" "internal app assembly directory"
require_dir "$ROOT_DIR/internal/platform" "internal platform directory"
require_dir "$ROOT_DIR/internal/shared" "internal shared directory"
require_dir "$ROOT_DIR/internal/shared/downstreamhttp" "shared downstream http directory"
require_dir "$ROOT_DIR/internal/platform/config" "internal platform config directory"
require_dir "$ROOT_DIR/internal/platform/db" "internal platform db directory"
require_file "$ROOT_DIR/internal/app/router.go" "internal app router"
require_file "$ROOT_DIR/internal/app/router_test.go" "internal app router tests"
require_file "$ROOT_DIR/internal/platform/config/config.go" "internal platform config"
require_file "$ROOT_DIR/internal/platform/db/postgres.go" "internal platform db"
require_file "$ROOT_DIR/cmd/release-service/main.go" "release-service entrypoint"
require_dir "$ROOT_DIR/internal/image" "internal image directory"
require_file "$ROOT_DIR/internal/image/module.go" "image module"
require_dir "$ROOT_DIR/internal/image/service" "image service directory"
require_dir "$ROOT_DIR/internal/image/transport/http" "image http transport directory"
require_dir "$ROOT_DIR/internal/application/transport/downstream" "application downstream transport directory"
require_dir "$ROOT_DIR/internal/project/transport/downstream" "project downstream transport directory"
require_dir "$ROOT_DIR/internal/environment/transport/downstream" "environment downstream transport directory"
require_dir "$ROOT_DIR/internal/cluster/transport/downstream" "cluster downstream transport directory"
require_dir "$ROOT_DIR/internal/manifest" "internal manifest directory"
require_file "$ROOT_DIR/internal/manifest/module.go" "manifest module"
require_dir "$ROOT_DIR/internal/manifest/service" "manifest service directory"
require_dir "$ROOT_DIR/internal/manifest/transport/http" "manifest http transport directory"
require_dir "$ROOT_DIR/internal/appconfig/transport/downstream" "appconfig downstream transport directory"
require_dir "$ROOT_DIR/internal/appservice/transport/downstream" "appservice downstream transport directory"
require_dir "$ROOT_DIR/internal/intent" "internal intent directory"
require_file "$ROOT_DIR/internal/intent/module.go" "intent module"
require_dir "$ROOT_DIR/internal/intent/service" "intent service directory"
require_dir "$ROOT_DIR/internal/intent/transport/http" "intent http transport directory"
require_dir "$ROOT_DIR/internal/release" "internal release directory"
require_file "$ROOT_DIR/internal/release/module.go" "release module"
require_file "$ROOT_DIR/internal/release/transport/http/router.go" "release router"
require_file "$ROOT_DIR/internal/release/config/config.go" "release config"
require_dir "$ROOT_DIR/internal/release/domain" "release domain directory"
require_dir "$ROOT_DIR/internal/release/support" "release support directory"
require_dir "$ROOT_DIR/internal/release/service" "release service directory"
require_dir "$ROOT_DIR/internal/platform/k8s" "internal platform k8s directory"
require_dir "$ROOT_DIR/internal/release/transport" "release transport directory"
require_dir "$ROOT_DIR/internal/release/transport/argo" "release argo transport directory"
require_dir "$ROOT_DIR/internal/release/transport/downstream" "release downstream transport directory"
require_dir "$ROOT_DIR/internal/release/transport/runtime" "release runtime transport directory"
require_dir "$ROOT_DIR/internal/release/transport/tekton" "release tekton transport directory"

[[ ! -d "$ROOT_DIR/shared" ]] || fail "catch-all shared directory must not exist: $ROOT_DIR/shared"
[[ ! -d "$ROOT_DIR/common" ]] || fail "catch-all common directory must not exist: $ROOT_DIR/common"
[[ ! -d "$ROOT_DIR/util" ]] || fail "catch-all util directory must not exist: $ROOT_DIR/util"
[[ ! -d "$ROOT_DIR/modules" ]] || fail "legacy modules directory must not exist: $ROOT_DIR/modules"
[[ ! -d "$ROOT_DIR/internal/release/argoclient" ]] || fail "legacy release argoclient directory must not exist: $ROOT_DIR/internal/release/argoclient"
[[ ! -d "$ROOT_DIR/internal/release/downstream" ]] || fail "legacy release downstream directory must not exist: $ROOT_DIR/internal/release/downstream"
[[ ! -d "$ROOT_DIR/internal/release/runtimeclient" ]] || fail "legacy release runtimeclient directory must not exist: $ROOT_DIR/internal/release/runtimeclient"
[[ ! -d "$ROOT_DIR/internal/release/infra" ]] || fail "legacy release infra directory must not exist: $ROOT_DIR/internal/release/infra"
[[ ! -d "$ROOT_DIR/internal/release/router" ]] || fail "legacy release router directory must not exist: $ROOT_DIR/internal/release/router"
[[ ! -d "$ROOT_DIR/internal/release/api" ]] || fail "legacy release api directory must not exist: $ROOT_DIR/internal/release/api"
[[ ! -d "$ROOT_DIR/internal/release/model" ]] || fail "legacy release model directory must not exist: $ROOT_DIR/internal/release/model"
[[ ! -d "$ROOT_DIR/internal/release/store" ]] || fail "legacy release store directory must not exist: $ROOT_DIR/internal/release/store"
[[ ! -f "$ROOT_DIR/internal/release/transport/downstream/app.go" ]] || fail "legacy release app downstream client must not exist: $ROOT_DIR/internal/release/transport/downstream/app.go"
[[ ! -f "$ROOT_DIR/internal/release/transport/downstream/config_manifest.go" ]] || fail "legacy release config downstream client must not exist: $ROOT_DIR/internal/release/transport/downstream/config_manifest.go"
[[ ! -f "$ROOT_DIR/internal/release/transport/downstream/network_manifest.go" ]] || fail "legacy release network downstream client must not exist: $ROOT_DIR/internal/release/transport/downstream/network_manifest.go"
[[ ! -f "$ROOT_DIR/internal/release/transport/downstream/client.go" ]] || fail "legacy release shared downstream client must not exist: $ROOT_DIR/internal/release/transport/downstream/client.go"

info "Checking root module contract"
require_literal "$ROOT_DIR/go.mod" "module path" "module github.com/bsonger/devflow-service"
require_literal "$ROOT_DIR/go.mod" "go baseline" "go 1.26.2"

info "Checking root entrypoint wiring"
require_literal "$ROOT_DIR/README.md" "README recovery link" "docs/system/recovery.md"
require_literal "$ROOT_DIR/README.md" "README verifier command" "bash scripts/verify.sh"
require_literal "$ROOT_DIR/AGENTS.md" "AGENTS recovery link" "docs/system/recovery.md"
require_literal "$ROOT_DIR/docs/README.md" "docs index agent start" "AGENTS.md"
require_literal "$ROOT_DIR/docs/index/README.md" "index README canonical agent start" "AGENTS.md"
require_literal "$ROOT_DIR/scripts/README.md" "scripts README verifier command" "bash scripts/verify.sh"
require_literal "$ROOT_DIR/docs/policies/docker-baseline.md" "Docker baseline install ban" "go install"
require_literal "$ROOT_DIR/docs/policies/docker-baseline.md" "Docker baseline go version" "1.26.2"
require_literal "$ROOT_DIR/docker/README.md" "docker assets README verifier command" "bash scripts/verify.sh"
require_literal "$ROOT_DIR/docker/README.md" "docker assets README policy reference" "approved FROM references"
require_literal "$ROOT_DIR/docs/policies/verification.md" "verification policy make ci" "make ci"
require_literal "$ROOT_DIR/scripts/README.md" "scripts README make ci" "make ci"
require_literal "$ROOT_DIR/README.md" "README release-service build target" "./cmd/release-service"
require_literal "$ROOT_DIR/docs/system/recovery.md" "system recovery release-service build target" "./cmd/release-service"
require_literal "$ROOT_DIR/docs/policies/verification.md" "verification policy release-service build target" "./cmd/release-service"
require_literal "$ROOT_DIR/scripts/README.md" "scripts README release-service build target" "./cmd/release-service"
require_literal "$ROOT_DIR/README.md" "README split release domains" "internal/image"
require_literal "$ROOT_DIR/docs/system/architecture.md" "system architecture split release domains" "internal/image"
require_literal "$ROOT_DIR/docs/services/release-service.md" "release-service split release domains" "internal/image"
require_literal "$ROOT_DIR/docs/services/release-service.md" "release-service release support area" "internal/release/support"
require_literal "$ROOT_DIR/docs/services/release-service.md" "release-service migrated downstream domains" "internal/appconfig/transport/downstream"
require_literal "$ROOT_DIR/docs/services/release-service.md" "release-service migrated owner readers" "internal/application/transport/downstream"

info "Checking repo-local documentation alignment"
require_literal "$ROOT_DIR/README.md" "README docs layout" "docs/index/"
require_literal "$ROOT_DIR/README.md" "README recovery contract" "docs/system/recovery.md"
require_literal "$ROOT_DIR/docs/system/architecture.md" "system architecture cmd layout" "cmd/"
require_literal "$ROOT_DIR/docs/system/architecture.md" "system architecture internal layout" "internal/"
require_literal "$ROOT_DIR/docs/system/architecture.md" "system architecture app assembly" "internal/platform/"
require_literal "$ROOT_DIR/docs/system/recovery.md" "system recovery verifier command" "bash scripts/verify.sh"
require_literal "$ROOT_DIR/docs/system/recovery.md" "system recovery migration target" "repository root layout"
require_literal "$ROOT_DIR/docs/system/observability.md" "system observability build proof" './cmd/meta-service'
require_literal "$ROOT_DIR/docs/policies/verification.md" "verification policy verifier command" "bash scripts/verify.sh"
require_literal "$ROOT_DIR/scripts/README.md" "scripts README meta-service build target" "./cmd/meta-service"
require_literal "$ROOT_DIR/docs/services/meta-service.md" "meta-service internal platform target" "internal/platform/..."

info "Checking meta-service documentation and packaging contract"
require_literal "$ROOT_DIR/docs/services/meta-service.md" "meta-service service name" "meta-service"
require_literal "$ROOT_DIR/docs/services/meta-service.md" "meta-service root build target" "./cmd/meta-service"
require_literal "$ROOT_DIR/docs/services/release-service.md" "release-service service name" "release-service"
require_literal "$ROOT_DIR/docs/services/release-service.md" "release-service verify merge note" "verify-service"
require_literal "$ROOT_DIR/Dockerfile" "meta-service Docker builder base" "FROM registry.cn-hangzhou.aliyuncs.com/devflow/golang-builder:1.26.2-alpine3.22 AS builder"
require_literal "$ROOT_DIR/Dockerfile" "meta-service Docker scratch base" "FROM scratch"
require_literal "$ROOT_DIR/Dockerfile" "meta-service Docker build target" "go build -o /out/meta-service ./cmd/meta-service"
require_literal "$ROOT_DIR/Dockerfile" "meta-service Docker copied binary" "COPY --from=builder /out/meta-service ./meta-service"
require_literal "$ROOT_DIR/Dockerfile" "meta-service Docker copied certs" "COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt"
require_literal "$ROOT_DIR/scripts/regen-swagger.sh" "meta-service optional swag guard" "swag CLI not installed; skipping Swagger regeneration"
require_literal "$ROOT_DIR/internal/app/router_test.go" "meta-service identity assertion" 'payload.Service != "meta-service"'

run_docker_policy_check
run_go_test
run_go_build_targets

info "Repository-local verification passed."
echo "  repo: $ROOT_DIR"
echo "  recovery: $ROOT_DIR/docs/system/recovery.md"
echo "  verifier: $ROOT_DIR/scripts/verify.sh"
