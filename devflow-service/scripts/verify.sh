#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONTROL_DIR="$ROOT_DIR/../devflow-control"
BLUEPRINT_VERIFIER="$CONTROL_DIR/scripts/verify-devflow-service-blueprint.sh"
HANDOFF_VERIFIER="$CONTROL_DIR/scripts/verify-devflow-service-migration-handoff.sh"

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

run_checked() {
  local label="$1"
  local script_path="$2"

  require_file "$script_path" "$label"

  info "Running $label"
  bash "$script_path"
}

info "Checking repository-local bootstrap and recovery surfaces"
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

info "Checking root entrypoint wiring"
require_literal "$ROOT_DIR/README.md" "README recovery link" "docs/recovery.md"
require_literal "$ROOT_DIR/README.md" "README verifier command" "bash scripts/verify.sh"
require_literal "$ROOT_DIR/AGENTS.md" "AGENTS recovery link" "docs/recovery.md"
require_literal "$ROOT_DIR/AGENTS.md" "AGENTS verifier command" "bash scripts/verify.sh"
require_literal "$ROOT_DIR/docs/README.md" "docs index recovery link" "recovery.md"
require_literal "$ROOT_DIR/scripts/README.md" "scripts README verifier command" "bash scripts/verify.sh"

info "Checking upstream frozen-doc verifier availability"
require_dir "$CONTROL_DIR" "devflow-control repository"
run_checked "upstream blueprint verifier" "$BLUEPRINT_VERIFIER"
run_checked "upstream migration handoff verifier" "$HANDOFF_VERIFIER"

info "Repository-local verification passed."
echo "  repo: $ROOT_DIR"
echo "  recovery: $ROOT_DIR/docs/recovery.md"
echo "  verifier: $ROOT_DIR/scripts/verify.sh"
