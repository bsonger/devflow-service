#!/usr/bin/env bash
set -euo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REPO_ROOT="$(cd "$DIR/../.." && pwd)"
cd "$REPO_ROOT"

if ! command -v swag >/dev/null 2>&1; then
  echo "INFO[meta-service-build]: swag CLI not installed; skipping Swagger regeneration." >&2
  exit 0
fi

OUTPUT_DIR="${SWAGGER_OUTPUT_DIR:-$DIR/.build/swagger}"
mkdir -p "$OUTPUT_DIR"
export GOROOT="$(go env GOROOT)"
swag init -g cmd/meta-service/main.go -d . --exclude modules/meta-service,.cache,bin --parseDependency --parseInternal -o "$OUTPUT_DIR"
