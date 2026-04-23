#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

info() {
  echo "INFO: $*"
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

info "Checking repo-local root-module documentation"
require_literal "$ROOT_DIR/README.md" "README root module contract" "single root module"
require_literal "$ROOT_DIR/docs/architecture.md" "architecture root module contract" "single root Go module"
require_literal "$ROOT_DIR/docs/recovery.md" "recovery root module contract" "single root module"
require_literal "$ROOT_DIR/docs/recovery.md" "recovery go baseline" "1.25.8"
require_literal "$ROOT_DIR/docs/recovery.md" "recovery extracted seam" "shared/routercore"
require_literal "$ROOT_DIR/docs/observability.md" "observability go test proof" 'go test ./...'

run_go_test

info "Repository-local verification passed."
echo "  repo: $ROOT_DIR"
echo "  recovery: $ROOT_DIR/docs/recovery.md"
echo "  verifier: $ROOT_DIR/scripts/verify.sh"
