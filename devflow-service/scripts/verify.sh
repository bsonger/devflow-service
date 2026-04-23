#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
META_SERVICE_DIR="$ROOT_DIR/modules/meta-service"

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
    go test ./...
  )
}

info "Checking repository-local root-module and recovery surfaces"
require_file "$ROOT_DIR/go.mod" "root go.mod"
require_file "$ROOT_DIR/README.md" "root README"
require_file "$ROOT_DIR/AGENTS.md" "root AGENTS"
require_file "$ROOT_DIR/docs/README.md" "docs index"
require_file "$ROOT_DIR/docs/architecture.md" "architecture doc"
require_file "$ROOT_DIR/docs/constraints.md" "constraints doc"
require_file "$ROOT_DIR/docs/observability.md" "observability doc"
require_file "$ROOT_DIR/docs/recovery.md" "recovery doc"
require_file "$ROOT_DIR/scripts/README.md" "scripts README"
require_file "$ROOT_DIR/scripts/verify.sh" "repo-local verifier"
require_file "$ROOT_DIR/scripts/check-docker-policy.sh" "Docker policy checker"
require_file "$ROOT_DIR/scripts/check-docker-policy_test.sh" "Docker policy checker test"
require_file "$ROOT_DIR/docs/docker.md" "Docker contract doc"
require_dir "$ROOT_DIR/docker" "docker assets directory"
require_file "$ROOT_DIR/docker/README.md" "docker assets README"
require_file "$ROOT_DIR/docker/golang-builder.Dockerfile" "repo-local golang builder Dockerfile"
require_file "$ROOT_DIR/docker/service.Dockerfile.template" "service Dockerfile template"

require_dir "$ROOT_DIR/cmd" "cmd directory"
require_dir "$ROOT_DIR/modules" "modules directory"
require_dir "$ROOT_DIR/shared" "shared directory"
require_dir "$ROOT_DIR/gateway" "gateway directory"

for pkg in httpx loggingx otelx pyroscopex observability routercore bootstrap; do
  require_dir "$ROOT_DIR/shared/$pkg" "shared/$pkg directory"
done

require_file "$ROOT_DIR/shared/httpx/pagination.go" "shared/httpx pagination helper"
require_file "$ROOT_DIR/shared/httpx/response.go" "shared/httpx response helper"
require_file "$ROOT_DIR/shared/httpx/httpx_test.go" "shared/httpx tests"
require_file "$ROOT_DIR/shared/loggingx/logging.go" "shared/loggingx logger helper"
require_file "$ROOT_DIR/shared/loggingx/logging_test.go" "shared/loggingx tests"
require_file "$ROOT_DIR/shared/otelx/metrics.go" "shared/otelx metrics helper"
require_file "$ROOT_DIR/shared/otelx/tracer.go" "shared/otelx tracer helper"
require_file "$ROOT_DIR/shared/pyroscopex/pyroscope.go" "shared/pyroscopex profiler helper"
require_file "$ROOT_DIR/shared/observability/runtime.go" "shared/observability runtime helper"
require_file "$ROOT_DIR/shared/observability/server.go" "shared/observability server helper"
require_file "$ROOT_DIR/shared/observability/dependency.go" "shared/observability dependency helper"
require_file "$ROOT_DIR/shared/routercore/gin.go" "shared/routercore gin helper"
require_file "$ROOT_DIR/shared/bootstrap/service.go" "shared/bootstrap service helper"

info "Checking migrated meta-service surfaces"
require_dir "$META_SERVICE_DIR" "meta-service directory"
require_file "$META_SERVICE_DIR/README.md" "meta-service README"
require_file "$META_SERVICE_DIR/scripts/build.sh" "meta-service build script"
require_file "$META_SERVICE_DIR/scripts/regen-swagger.sh" "meta-service swagger regen script"
require_file "$META_SERVICE_DIR/Dockerfile" "meta-service Dockerfile"
require_file "$META_SERVICE_DIR/cmd/main.go" "meta-service entrypoint"
require_file "$META_SERVICE_DIR/pkg/router/router_test.go" "meta-service router identity test"

info "Checking root module contract"
require_literal "$ROOT_DIR/go.mod" "module path" "module github.com/bsonger/devflow-service"
require_literal "$ROOT_DIR/go.mod" "go baseline" "go 1.25.8"

info "Checking root entrypoint wiring"
require_literal "$ROOT_DIR/README.md" "README recovery link" "docs/recovery.md"
require_literal "$ROOT_DIR/README.md" "README verifier command" "bash scripts/verify.sh"
require_literal "$ROOT_DIR/AGENTS.md" "AGENTS recovery link" "docs/recovery.md"
require_literal "$ROOT_DIR/AGENTS.md" "AGENTS verifier command" "bash scripts/verify.sh"
require_literal "$ROOT_DIR/docs/README.md" "docs index recovery link" "recovery.md"
require_literal "$ROOT_DIR/scripts/README.md" "scripts README verifier command" "bash scripts/verify.sh"
require_literal "$ROOT_DIR/docs/docker.md" "Docker contract banner" "controlled Docker baseline"
require_literal "$ROOT_DIR/docs/docker.md" "Docker contract inline install ban" "go install"
require_literal "$ROOT_DIR/docker/README.md" "docker assets README verifier command" "bash scripts/verify.sh"
require_literal "$ROOT_DIR/docker/README.md" "docker assets README policy reference" "approved FROM references"

info "Checking repo-local root-module documentation"
require_literal "$ROOT_DIR/README.md" "README root module contract" "single root module"
require_literal "$ROOT_DIR/README.md" "README meta-service contract" "modules/meta-service"
require_literal "$ROOT_DIR/docs/architecture.md" "architecture root module contract" "single root Go module"
require_literal "$ROOT_DIR/docs/architecture.md" "architecture meta-service contract" "modules/meta-service"
require_literal "$ROOT_DIR/docs/recovery.md" "recovery root module contract" "single root module"
require_literal "$ROOT_DIR/docs/recovery.md" "recovery go baseline" "1.25.8"
require_literal "$ROOT_DIR/docs/recovery.md" "recovery extracted seam" "shared/routercore"
require_literal "$ROOT_DIR/docs/recovery.md" "recovery meta-service build command" "bash modules/meta-service/scripts/build.sh"
require_literal "$ROOT_DIR/docs/observability.md" "observability go test proof" 'go test ./...'
require_literal "$ROOT_DIR/docs/observability.md" "observability meta-service surfaces" 'modules/meta-service'
require_literal "$ROOT_DIR/scripts/README.md" "scripts README meta-service build script" 'scripts/build.sh'

info "Checking meta-service documentation and packaging contract"
require_literal "$META_SERVICE_DIR/README.md" "meta-service shared adoption" "shared/bootstrap"
require_literal "$META_SERVICE_DIR/README.md" "meta-service build command" "bash scripts/build.sh"
require_literal "$META_SERVICE_DIR/README.md" "meta-service deferred rollout note" "S05 or later"
require_literal "$META_SERVICE_DIR/Dockerfile" "meta-service Docker scratch base" "FROM scratch"
require_literal "$META_SERVICE_DIR/Dockerfile" "meta-service Docker staged binary" "COPY .build/staging/meta-service/meta-service ./meta-service"
require_literal "$META_SERVICE_DIR/Dockerfile" "meta-service Docker staged certs" "COPY .build/staging/_shared/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt"
require_literal "$META_SERVICE_DIR/scripts/build.sh" "meta-service build go build target" "./modules/meta-service/cmd"
require_literal "$META_SERVICE_DIR/scripts/build.sh" "meta-service build binary contract" 'OUTPUT_BINARY_REL="bin/$SERVICE_NAME"'
require_literal "$META_SERVICE_DIR/scripts/build.sh" "meta-service staging contract" 'STAGING_ROOT="$BUILD_ROOT/staging"'
require_literal "$META_SERVICE_DIR/scripts/regen-swagger.sh" "meta-service optional swag guard" "swag CLI not installed; skipping Swagger regeneration"
require_literal "$META_SERVICE_DIR/pkg/router/router_test.go" "meta-service identity assertion" 'payload.Service != "meta-service"'

run_docker_policy_check
run_go_test

info "Repository-local verification passed."
echo "  repo: $ROOT_DIR"
echo "  migrated service: $META_SERVICE_DIR"
echo "  recovery: $ROOT_DIR/docs/recovery.md"
echo "  verifier: $ROOT_DIR/scripts/verify.sh"
