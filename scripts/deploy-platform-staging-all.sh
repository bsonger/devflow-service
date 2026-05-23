#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  PLATFORM_BASE_URL=https://devflow.bei.com \
  PROJECT_ID=<project-uuid> \
  ENVIRONMENT_ID=<environment-uuid> \
  PLATFORM_AUTH_HEADER='Authorization: Bearer <token>' \
  bash scripts/deploy-platform-staging-all.sh

Required environment variables:
  PLATFORM_BASE_URL   Platform base URL, for example https://devflow.bei.com
  PROJECT_ID          Project UUID that contains the 5 service applications
  ENVIRONMENT_ID      Target staging environment UUID

Authentication options (optional but usually required):
  PLATFORM_AUTH_HEADER    Full header line, e.g. 'Authorization: Bearer <token>'
  PLATFORM_COOKIE_HEADER  Full header line, e.g. 'Cookie: session=<value>'

Optional environment variables:
  SERVICE_NAMES           Comma-separated application names, default:
                          meta-service,config-service,network-service,release-service,runtime-service,platform-web
  GIT_REVISION            Git revision for all deployments, default main
  STRATEGY                rolling | blueGreen | canary, default rolling
  WAIT_FOR_MANIFEST       true | false, default true
  WAIT_FOR_RELEASE        true | false, default true
  POLL_INTERVAL_SECONDS   Poll interval, default 10
  MANIFEST_TIMEOUT_SECONDS
                          Manifest wait timeout per service, default 900
  RELEASE_TIMEOUT_SECONDS
                          Release wait timeout per service, default 1800
  FLOW_DIR                Output directory, default scripts/.tmp/deploy-platform-staging-all

Notes:
  - This script discovers applications by project, then deploys each one through
    scripts/deploy-platform-release.sh.
  - The default target set is the 5 backend services plus the frontend:
    meta-service, config-service, network-service, release-service, runtime-service, platform-web.
EOF
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "missing required command: $1" >&2
    exit 1
  }
}

require_env() {
  local name=$1
  if [[ -z "${!name:-}" ]]; then
    echo "missing required env: $name" >&2
    usage >&2
    exit 1
  fi
}

capture_request() {
  local method=$1
  local url=$2
  local response_body=$3
  local response_status=$4
  local response_headers=$5

  local curl_args=(
    -sS
    -X "$method"
    -H 'Accept: application/json'
    -D "$response_headers"
    -o "$response_body"
    -w '%{http_code}'
  )

  if [[ -n "${AUTH_HEADER:-}" ]]; then
    curl_args+=(-H "$AUTH_HEADER")
  fi
  if [[ -n "${COOKIE_HEADER:-}" ]]; then
    curl_args+=(-H "$COOKIE_HEADER")
  fi

  local status
  status=$(curl "${curl_args[@]}" "$url")
  printf '%s' "$status" >"$response_status"
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

require_cmd curl
require_cmd jq
require_env PLATFORM_BASE_URL
require_env PROJECT_ID
require_env ENVIRONMENT_ID

AUTH_HEADER=${PLATFORM_AUTH_HEADER:-}
COOKIE_HEADER=${PLATFORM_COOKIE_HEADER:-}
SERVICE_NAMES=${SERVICE_NAMES:-meta-service,config-service,network-service,release-service,runtime-service,platform-web}
GIT_REVISION=${GIT_REVISION:-main}
STRATEGY=${STRATEGY:-rolling}
WAIT_FOR_MANIFEST=${WAIT_FOR_MANIFEST:-true}
WAIT_FOR_RELEASE=${WAIT_FOR_RELEASE:-true}
POLL_INTERVAL_SECONDS=${POLL_INTERVAL_SECONDS:-10}
MANIFEST_TIMEOUT_SECONDS=${MANIFEST_TIMEOUT_SECONDS:-900}
RELEASE_TIMEOUT_SECONDS=${RELEASE_TIMEOUT_SECONDS:-1800}
FLOW_DIR=${FLOW_DIR:-scripts/.tmp/deploy-platform-staging-all}

BASE_URL=${PLATFORM_BASE_URL%/}
APPLICATIONS_URL="$BASE_URL/api/v1/meta/applications?project_id=$PROJECT_ID"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
CHILD_SCRIPT="$SCRIPT_DIR/deploy-platform-release.sh"

mkdir -p "$FLOW_DIR"

capture_request "GET" "$APPLICATIONS_URL" "$FLOW_DIR/applications.body.json" "$FLOW_DIR/applications.status" "$FLOW_DIR/applications.headers"
applications_status=$(<"$FLOW_DIR/applications.status")
if [[ "$applications_status" != "200" ]]; then
  echo "failed to list project applications: status=$applications_status" >&2
  cat "$FLOW_DIR/applications.body.json" >&2
  exit 1
fi

IFS=',' read -r -a service_names <<<"$SERVICE_NAMES"
for i in "${!service_names[@]}"; do
  service_names[$i]=$(printf '%s' "${service_names[$i]}" | xargs)
done

for service_name in "${service_names[@]}"; do
  [[ -n "$service_name" ]] || continue

  application_id=$(
    jq -r --arg service_name "$service_name" '
      [
        (.data // . // [])
        | .[]
        | select(.name == $service_name)
      ]
      | first
      | .id // empty
    ' "$FLOW_DIR/applications.body.json"
  )

  if [[ -z "$application_id" ]]; then
    echo "application not found for service_name=$service_name in project_id=$PROJECT_ID" >&2
    jq -r '(.data // . // []) | .[] | .name' "$FLOW_DIR/applications.body.json" >&2 || true
    exit 1
  fi

  echo "deploying_service_name=$service_name application_id=$application_id environment_id=$ENVIRONMENT_ID"

  PLATFORM_BASE_URL="$PLATFORM_BASE_URL" \
  APPLICATION_ID="$application_id" \
  ENVIRONMENT_ID="$ENVIRONMENT_ID" \
  SERVICE_NAME="$service_name" \
  GIT_REVISION="$GIT_REVISION" \
  STRATEGY="$STRATEGY" \
  WAIT_FOR_MANIFEST="$WAIT_FOR_MANIFEST" \
  WAIT_FOR_RELEASE="$WAIT_FOR_RELEASE" \
  POLL_INTERVAL_SECONDS="$POLL_INTERVAL_SECONDS" \
  MANIFEST_TIMEOUT_SECONDS="$MANIFEST_TIMEOUT_SECONDS" \
  RELEASE_TIMEOUT_SECONDS="$RELEASE_TIMEOUT_SECONDS" \
  FLOW_DIR="$FLOW_DIR/$service_name" \
  PLATFORM_AUTH_HEADER="${AUTH_HEADER:-}" \
  PLATFORM_COOKIE_HEADER="${COOKIE_HEADER:-}" \
  bash "$CHILD_SCRIPT"
done

echo "project_id=$PROJECT_ID"
echo "environment_id=$ENVIRONMENT_ID"
echo "service_names=$SERVICE_NAMES"
echo "git_revision=$GIT_REVISION"
echo "strategy=$STRATEGY"
echo "flow_dir=$FLOW_DIR"
