#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SERVICE_NAME="meta-service"
BUILD_ROOT="$ROOT_DIR/.build"
STAGING_ROOT="$BUILD_ROOT/staging"
SERVICE_STAGE_DIR="$STAGING_ROOT/$SERVICE_NAME"
SHARED_STAGE_DIR="$STAGING_ROOT/_shared"
BIN_DIR="$ROOT_DIR/bin"
OUTPUT_BINARY="$BIN_DIR/$SERVICE_NAME"
OUTPUT_BINARY_REL="bin/$SERVICE_NAME"
SOURCE_BINARY="$SERVICE_STAGE_DIR/$SERVICE_NAME"
TEMP_SWAGGER_DIR="$BUILD_ROOT/swagger"
GO_CACHE_DIR="${GOCACHE:-$ROOT_DIR/.cache/go-build}"

log() {
  echo "INFO[meta-service-build]: $*"
}

copy_if_exists() {
  local source="$1"
  local target="$2"
  if [[ -e "$source" ]]; then
    mkdir -p "$(dirname "$target")"
    cp -R "$source" "$target"
    log "staged ${source#$ROOT_DIR/}"
  fi
}

log "preparing deterministic artifact directories"
rm -rf "$BUILD_ROOT" "$BIN_DIR"
mkdir -p "$SERVICE_STAGE_DIR" "$SHARED_STAGE_DIR/certs" "$BIN_DIR" "$GO_CACHE_DIR"
mkdir -p "$SERVICE_STAGE_DIR/docs" "$SERVICE_STAGE_DIR/config"

if [[ -f "$ROOT_DIR/scripts/regen-swagger.sh" ]]; then
  log "running optional Swagger regeneration into temporary staging"
  rm -rf "$TEMP_SWAGGER_DIR"
  mkdir -p "$TEMP_SWAGGER_DIR"
  if (
    cd "$ROOT_DIR"
    SWAGGER_OUTPUT_DIR="$TEMP_SWAGGER_DIR" bash scripts/regen-swagger.sh
  ); then
    copy_if_exists "$TEMP_SWAGGER_DIR" "$SERVICE_STAGE_DIR/docs/generated/swagger"
  else
    log "Swagger regeneration skipped or failed; continuing with existing route wiring"
  fi
else
  log "Swagger regeneration script not present; keeping existing route wiring without generated docs"
fi

CERT_SOURCE=""
for candidate in /etc/ssl/certs/ca-certificates.crt /etc/ssl/cert.pem; do
  if [[ -f "$candidate" ]]; then
    CERT_SOURCE="$candidate"
    break
  fi
done

if [[ -z "$CERT_SOURCE" ]]; then
  echo "ERROR[meta-service-build]: no CA certificate bundle found on the build host" >&2
  exit 1
fi
cp "$CERT_SOURCE" "$SHARED_STAGE_DIR/certs/ca-certificates.crt"
log "staged CA certificates from $CERT_SOURCE"

log "building linux/amd64 binary via root module"
(
  cd "$ROOT_DIR"
  GOCACHE="$GO_CACHE_DIR" CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o "$SOURCE_BINARY" ./cmd/meta-service
)
cp "$SOURCE_BINARY" "$OUTPUT_BINARY"
log "built binary at $OUTPUT_BINARY_REL"

copy_if_exists "$ROOT_DIR/docs/generated" "$SERVICE_STAGE_DIR/docs/generated"
copy_if_exists "$ROOT_DIR/config" "$SERVICE_STAGE_DIR/config"

log "artifact staging complete"
echo "  binary: $OUTPUT_BINARY"
echo "  staged service dir: $SERVICE_STAGE_DIR"
echo "  staged certs: $SHARED_STAGE_DIR/certs/ca-certificates.crt"
