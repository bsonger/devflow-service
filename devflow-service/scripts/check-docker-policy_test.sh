#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TEST_ROOT="$ROOT_DIR/scripts/testdata/docker-policy"
VALID_DIR="$TEST_ROOT/valid/modules/meta-service"
INVALID_DIR="$TEST_ROOT/invalid/modules/meta-service"
CHECKER="$ROOT_DIR/scripts/check-docker-policy.sh"

cleanup() {
  rm -rf "$TEST_ROOT"
}
trap cleanup EXIT

mkdir -p "$VALID_DIR" "$INVALID_DIR"

cat > "$VALID_DIR/Dockerfile.package" <<'EOF_VALID'
FROM registry.cn-hangzhou.aliyuncs.com/devflow/golang-builder:1.26.2 AS builder
WORKDIR /workspace
COPY . .
RUN go build -o /out/meta-service ./cmd/meta-service

FROM scratch
WORKDIR /app
COPY --from=builder /out/meta-service ./meta-service
ENTRYPOINT ["/app/meta-service"]
EOF_VALID

cat > "$INVALID_DIR/Dockerfile" <<'EOF_INVALID'
FROM alpine:3.22
RUN apk add --no-cache curl
ENTRYPOINT ["/bin/sh"]
EOF_INVALID

echo "INFO[test]: expecting pass for approved builder/runtime references"
bash "$CHECKER" "$TEST_ROOT/valid/modules"

echo "INFO[test]: expecting failure for banned install pattern and unapproved runtime"
if bash "$CHECKER" "$TEST_ROOT/invalid/modules"; then
  echo "ERROR[test]: expected invalid fixture to fail policy check" >&2
  exit 1
fi

echo "INFO[test]: docker policy checker fixtures passed"
