#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DOCKER_SCAN_ROOT="${1:-$ROOT_DIR}"

BANNED_PATTERNS=(
  'apk[[:space:]]+add'
  'apk[[:space:]]+upgrade'
  'apt-get'
  'yum'
  'dnf'
  'go[[:space:]]+install'
)

APPROVED_FROM_PATTERNS=(
  '^FROM[[:space:]]+registry\.cn-hangzhou\.aliyuncs\.com/devflow/golang-builder:1\.26\.2([[:space:]]+AS[[:space:]]+[A-Za-z0-9._-]+)?$'
  '^FROM[[:space:]]+scratch([[:space:]]+AS[[:space:]]+[A-Za-z0-9._-]+)?$'
  '^FROM[[:space:]]+registry\.cn-hangzhou\.aliyuncs\.com/devflow/[A-Za-z0-9._/-]+(:[A-Za-z0-9._-]+)?([[:space:]]+AS[[:space:]]+[A-Za-z0-9._-]+)?$'
)

info() {
  echo "INFO[docker-policy]: $*"
}

fail() {
  echo "ERROR[docker-policy]: $*" >&2
  exit 1
}

matches_any() {
  local value="$1"
  shift
  local pattern
  for pattern in "$@"; do
    if [[ "$value" =~ $pattern ]]; then
      return 0
    fi
  done
  return 1
}

dockerfiles=()
while IFS= read -r dockerfile; do
  [[ -n "$dockerfile" ]] || continue
  dockerfiles+=("$dockerfile")
done < <(
  find "$DOCKER_SCAN_ROOT" \
    -type f \
    \( -name 'Dockerfile' -o -name 'Dockerfile.*' \) \
    ! -path "$DOCKER_SCAN_ROOT/docker/*" \
    ! -path "$DOCKER_SCAN_ROOT/.build/*" \
    2>/dev/null | sort -u
)

if [[ ${#dockerfiles[@]} -eq 0 ]]; then
  info "No service Dockerfiles found under $DOCKER_SCAN_ROOT; static policy check passed."
  exit 0
fi

info "Scanning ${#dockerfiles[@]} Dockerfile(s) under $DOCKER_SCAN_ROOT"

violations=0

for dockerfile in "${dockerfiles[@]}"; do
  info "Checking $dockerfile"

  while IFS=':' read -r line_number line_content; do
    [[ -n "$line_number" ]] || continue
    echo "ERROR[docker-policy]: banned inline install pattern in $dockerfile:$line_number -> $line_content" >&2
    violations=1
  done < <(grep -Eni "$(IFS='|'; echo "${BANNED_PATTERNS[*]}")" "$dockerfile" || true)

  from_found=0
  while IFS= read -r from_line; do
    [[ -n "$from_line" ]] || continue
    from_found=1
    if ! matches_any "$from_line" "${APPROVED_FROM_PATTERNS[@]}"; then
      echo "ERROR[docker-policy]: unapproved FROM reference in $dockerfile -> $from_line" >&2
      violations=1
    fi
  done < <(grep -E '^[[:space:]]*FROM[[:space:]]+' "$dockerfile" | sed -E 's/^[[:space:]]+//')

  if [[ "$from_found" -eq 0 ]]; then
    echo "ERROR[docker-policy]: Dockerfile has no FROM instruction: $dockerfile" >&2
    violations=1
  fi
done

if [[ "$violations" -ne 0 ]]; then
  fail "Policy violations detected. See errors above."
fi

info "Docker policy passed for ${#dockerfiles[@]} Dockerfile(s)."
