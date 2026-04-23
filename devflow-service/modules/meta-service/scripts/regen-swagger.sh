#!/usr/bin/env bash
set -euo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$DIR"

if ! command -v swag >/dev/null 2>&1; then
  echo "INFO[meta-service-build]: swag CLI not installed; skipping Swagger regeneration." >&2
  exit 0
fi

OUTPUT_DIR="${SWAGGER_OUTPUT_DIR:-$DIR/docs/generated/swagger}"
mkdir -p "$OUTPUT_DIR"
export GOROOT="$(go env GOROOT)"
swag init -g cmd/main.go --parseDependency -o "$OUTPUT_DIR"
