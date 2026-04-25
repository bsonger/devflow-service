#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE_TAG="${1:-devflow-service:local}"
DOCKERFILE_PATH="${2:-Dockerfile}"

run_build() {
  docker build -t "$IMAGE_TAG" -f "$DOCKERFILE_PATH" "$ROOT_DIR"
}

docker_config_dir() {
  printf '%s\n' "${DOCKER_CONFIG:-$HOME/.docker}"
}

docker_config_has_missing_desktop_helper() {
  local config_path
  config_path="$(docker_config_dir)/config.json"
  if [[ ! -f "$config_path" ]]; then
    return 1
  fi
  grep -q '"credsStore"[[:space:]]*:[[:space:]]*"desktop"' "$config_path" &&
    ! command -v docker-credential-desktop >/dev/null 2>&1
}

if docker_config_has_missing_desktop_helper; then
  temp_dir="$(mktemp -d)"
  trap 'rm -rf "$temp_dir"' EXIT
  if [[ -d "$(docker_config_dir)/contexts" ]]; then
    mkdir -p "$temp_dir/contexts"
    cp -R "$(docker_config_dir)/contexts"/. "$temp_dir/contexts"/
  fi
  if [[ -d "$(docker_config_dir)/cli-plugins" ]]; then
    mkdir -p "$temp_dir/cli-plugins"
    cp -R "$(docker_config_dir)/cli-plugins"/. "$temp_dir/cli-plugins"/
  fi
  sed '/"credsStore"[[:space:]]*:[[:space:]]*"desktop"/d' "$(docker_config_dir)/config.json" > "$temp_dir/config.json"
  DOCKER_CONFIG="$temp_dir" run_build
  exit 0
fi

run_build
