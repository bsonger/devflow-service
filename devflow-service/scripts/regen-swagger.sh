#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

if ! command -v swag >/dev/null 2>&1; then
  echo "INFO[meta-service-build]: swag CLI not installed; skipping Swagger regeneration." >&2
  exit 0
fi

OUTPUT_DIR="${SWAGGER_OUTPUT_DIR:-$ROOT_DIR/.build/swagger}"
mkdir -p "$OUTPUT_DIR"
export GOROOT="$(go env GOROOT)"
swag init -g cmd/meta-service/main.go -d . --exclude .build,.cache,bin --parseDependency --parseInternal -o "$OUTPUT_DIR"
