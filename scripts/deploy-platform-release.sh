#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  PLATFORM_BASE_URL=https://devflow.bei.com \
  APPLICATION_ID=<uuid> \
  ENVIRONMENT_ID=<uuid> \
  SERVICE_NAME=<service-name> \
  bash scripts/deploy-platform-release.sh

Required environment variables:
  PLATFORM_BASE_URL   Platform base URL, for example https://devflow.bei.com
  APPLICATION_ID      Target application UUID
  ENVIRONMENT_ID      Target environment UUID

Authentication options (optional but usually required):
  PLATFORM_AUTH_HEADER    Full header line, e.g. 'Authorization: Bearer <token>'
  PLATFORM_COOKIE_HEADER  Full header line, e.g. 'Cookie: session=<value>'

Optional environment variables:
  SERVICE_NAME             Validate that this application contains the target
                           service before creating the manifest/release
  GIT_REVISION            Git revision used to create the manifest, default main
  STRATEGY                rolling | blueGreen | canary, default rolling
  RELEASE_TYPE            Release type, default Upgrade
  WAIT_FOR_MANIFEST       true | false, default true
  WAIT_FOR_RELEASE        true | false, default true
  POLL_INTERVAL_SECONDS   Poll interval, default 10
  MANIFEST_TIMEOUT_SECONDS
                          Manifest wait timeout, default 900
  RELEASE_TIMEOUT_SECONDS
                          Release wait timeout, default 1800
  FLOW_DIR                Output directory, default scripts/.tmp/deploy-platform-release

Notes:
  - This script deploys through the platform release APIs. It does not apply
    Kubernetes YAML directly.
  - SERVICE_NAME is currently a validation and operator-safety selector. The
    active backend release contract remains application-scoped, so the manifest
    and release still freeze all application services/workload inputs together.
  - The flow is manifest-first: create manifest, wait for manifest availability,
    then create release for the target environment.
  - Existing manifests are reused when they already match APPLICATION_ID and
    GIT_REVISION and are in status Available.
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

json_get() {
  local file=$1
  local expr=$2
  jq -r "$expr // empty" "$file"
}

capture_request() {
  local method=$1
  local url=$2
  local body_file=${3:-}
  local response_body=$4
  local response_status=$5
  local response_headers=$6

  local curl_args=(
    -sS
    -X "$method"
    -H 'Accept: application/json'
    -H 'Content-Type: application/json'
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
  if [[ -n "$body_file" ]]; then
    curl_args+=(--data @"$body_file")
  fi

  local status
  status=$(curl "${curl_args[@]}" "$url")
  printf '%s' "$status" >"$response_status"
}

extract_header_value() {
  local file=$1
  local name=$2
  awk -F': *' -v key="$(printf '%s' "$name" | tr '[:upper:]' '[:lower:]')" '
    BEGIN { IGNORECASE = 1 }
    tolower($1) == key {
      gsub(/\r/, "", $2)
      print $2
      exit
    }
  ' "$file"
}

print_request_context() {
  local body_file=$1
  local headers_file=$2
  local request_id trace_id
  request_id=$(json_get "$body_file" '.error.request_id')
  trace_id=$(json_get "$body_file" '.error.trace_id')
  if [[ -z "$request_id" ]]; then
    request_id=$(extract_header_value "$headers_file" "X-Request-Id")
  fi
  if [[ -z "$trace_id" ]]; then
    trace_id=$(extract_header_value "$headers_file" "X-Trace-Id")
  fi
  if [[ -n "$request_id" ]]; then
    echo "request_id=$request_id"
  fi
  if [[ -n "$trace_id" ]]; then
    echo "trace_id=$trace_id"
  fi
}

fail_with_response() {
  local message=$1
  local body_file=$2
  local headers_file=$3

  echo "$message" >&2
  print_request_context "$body_file" "$headers_file" >&2 || true
  if [[ -s "$body_file" ]]; then
    cat "$body_file" >&2
  fi
  exit 1
}

manifest_status_is_terminal() {
  case "$1" in
    Available|Unavailable)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

release_status_is_terminal() {
  case "$1" in
    Succeeded|Failed|RolledBack|SyncFailed)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

validate_service_name() {
  local list_url=$1
  local body_file=$2
  local status_file=$3
  local headers_file=$4
  local service_name=$5

  capture_request "GET" "$list_url" "" "$body_file" "$status_file" "$headers_file"
  local read_status
  read_status=$(<"$status_file")
  [[ "$read_status" == "200" ]] || fail_with_response "service list failed with status $read_status" "$body_file" "$headers_file"

  local matched_name
  matched_name=$(
    jq -r --arg service_name "$service_name" '
      [
        (.data // . // [])
        | .[]
        | select(.name == $service_name)
      ]
      | first
      | .name // empty
    ' "$body_file"
  )
  if [[ -z "$matched_name" ]]; then
    echo "service_name=$service_name was not found under application_id=$APPLICATION_ID" >&2
    jq -r '
      [
        (.data // . // [])
        | .[]
        | .name
      ]
      | .[]
    ' "$body_file" >&2 || true
    exit 1
  fi
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

require_cmd curl
require_cmd jq
require_env PLATFORM_BASE_URL
require_env APPLICATION_ID
require_env ENVIRONMENT_ID

AUTH_HEADER=${PLATFORM_AUTH_HEADER:-}
COOKIE_HEADER=${PLATFORM_COOKIE_HEADER:-}
GIT_REVISION=${GIT_REVISION:-main}
STRATEGY=${STRATEGY:-rolling}
RELEASE_TYPE=${RELEASE_TYPE:-Upgrade}
WAIT_FOR_MANIFEST=${WAIT_FOR_MANIFEST:-true}
WAIT_FOR_RELEASE=${WAIT_FOR_RELEASE:-true}
SERVICE_NAME=${SERVICE_NAME:-}
POLL_INTERVAL_SECONDS=${POLL_INTERVAL_SECONDS:-10}
MANIFEST_TIMEOUT_SECONDS=${MANIFEST_TIMEOUT_SECONDS:-900}
RELEASE_TIMEOUT_SECONDS=${RELEASE_TIMEOUT_SECONDS:-1800}
FLOW_DIR=${FLOW_DIR:-scripts/.tmp/deploy-platform-release}

case "$STRATEGY" in
  rolling|blueGreen|canary) ;;
  *)
    echo "unsupported STRATEGY: $STRATEGY" >&2
    exit 1
    ;;
esac

case "$WAIT_FOR_MANIFEST" in
  true|false) ;;
  *)
    echo "WAIT_FOR_MANIFEST must be true or false" >&2
    exit 1
    ;;
esac

case "$WAIT_FOR_RELEASE" in
  true|false) ;;
  *)
    echo "WAIT_FOR_RELEASE must be true or false" >&2
    exit 1
    ;;
esac

BASE_URL=${PLATFORM_BASE_URL%/}
MANIFESTS_URL="$BASE_URL/api/v1/release/manifests"
RELEASES_URL="$BASE_URL/api/v1/release/releases"
SERVICES_URL="$BASE_URL/api/v1/network/services?application_id=$APPLICATION_ID"
LIST_MANIFESTS_URL="$MANIFESTS_URL?application_id=$APPLICATION_ID"
LIST_RELEASES_URL="$RELEASES_URL?application_id=$APPLICATION_ID&environment_id=$ENVIRONMENT_ID"

mkdir -p "$FLOW_DIR"

if [[ -n "$SERVICE_NAME" ]]; then
  validate_service_name "$SERVICES_URL" "$FLOW_DIR/services.list.body.json" "$FLOW_DIR/services.list.status" "$FLOW_DIR/services.list.headers" "$SERVICE_NAME"
  echo "validated_service_name=$SERVICE_NAME"
fi

capture_request "GET" "$LIST_MANIFESTS_URL" "" "$FLOW_DIR/manifests.list.body.json" "$FLOW_DIR/manifests.list.status" "$FLOW_DIR/manifests.list.headers"
list_manifest_status=$(<"$FLOW_DIR/manifests.list.status")
[[ "$list_manifest_status" == "200" ]] || fail_with_response "manifest list failed with status $list_manifest_status" "$FLOW_DIR/manifests.list.body.json" "$FLOW_DIR/manifests.list.headers"

existing_manifest_id=$(
  jq -r --arg git_revision "$GIT_REVISION" '
    [
      (.data // . // [])
      | .[]
      | select((.git_revision // "main") == $git_revision and .status == "Available")
    ]
    | sort_by(.updated_at // .created_at // "")
    | last
    | .id // empty
  ' "$FLOW_DIR/manifests.list.body.json"
)

manifest_id=${existing_manifest_id:-}
manifest_created=false

if [[ -z "$manifest_id" ]]; then
  cat >"$FLOW_DIR/manifest.create.payload.json" <<EOF
{
  "application_id": "$APPLICATION_ID",
  "git_revision": "$GIT_REVISION"
}
EOF

  capture_request "POST" "$MANIFESTS_URL" "$FLOW_DIR/manifest.create.payload.json" "$FLOW_DIR/manifest.create.body.json" "$FLOW_DIR/manifest.create.status" "$FLOW_DIR/manifest.create.headers"
  create_manifest_status=$(<"$FLOW_DIR/manifest.create.status")
  [[ "$create_manifest_status" == "201" ]] || fail_with_response "manifest create failed with status $create_manifest_status" "$FLOW_DIR/manifest.create.body.json" "$FLOW_DIR/manifest.create.headers"

  manifest_id=$(json_get "$FLOW_DIR/manifest.create.body.json" '.data.id')
  [[ -n "$manifest_id" ]] || fail_with_response "manifest id missing in create response" "$FLOW_DIR/manifest.create.body.json" "$FLOW_DIR/manifest.create.headers"
  manifest_created=true
  echo "created_manifest_id=$manifest_id"
else
  echo "reused_manifest_id=$manifest_id"
fi

manifest_url="$MANIFESTS_URL/$manifest_id"
manifest_body="$FLOW_DIR/manifest.get.body.json"
manifest_status_file="$FLOW_DIR/manifest.get.status"
manifest_headers="$FLOW_DIR/manifest.get.headers"

if [[ "$WAIT_FOR_MANIFEST" == "true" ]]; then
  manifest_start_ts=$(date +%s)
  while true; do
    capture_request "GET" "$manifest_url" "" "$manifest_body" "$manifest_status_file" "$manifest_headers"
    read_manifest_status=$(<"$manifest_status_file")
    [[ "$read_manifest_status" == "200" ]] || fail_with_response "manifest read failed with status $read_manifest_status" "$manifest_body" "$manifest_headers"

    manifest_status=$(json_get "$manifest_body" '.data.status')
    manifest_commit_hash=$(json_get "$manifest_body" '.data.commit_hash')
    manifest_image_ref=$(json_get "$manifest_body" '.data.image_ref')
    echo "manifest_id=$manifest_id status=${manifest_status:-unknown} commit_hash=${manifest_commit_hash:-} image_ref=${manifest_image_ref:-}"

    if [[ "$manifest_status" == "Available" ]]; then
      break
    fi
    if [[ "$manifest_status" == "Unavailable" ]]; then
      fail_with_response "manifest $manifest_id reached Unavailable" "$manifest_body" "$manifest_headers"
    fi

    now_ts=$(date +%s)
    if (( now_ts - manifest_start_ts >= MANIFEST_TIMEOUT_SECONDS )); then
      fail_with_response "timed out waiting for manifest $manifest_id to become Available" "$manifest_body" "$manifest_headers"
    fi

    sleep "$POLL_INTERVAL_SECONDS"
  done
else
  capture_request "GET" "$manifest_url" "" "$manifest_body" "$manifest_status_file" "$manifest_headers"
  read_manifest_status=$(<"$manifest_status_file")
  [[ "$read_manifest_status" == "200" ]] || fail_with_response "manifest read failed with status $read_manifest_status" "$manifest_body" "$manifest_headers"
fi

cat >"$FLOW_DIR/release.create.payload.json" <<EOF
{
  "manifest_id": "$manifest_id",
  "environment_id": "$ENVIRONMENT_ID",
  "strategy": "$STRATEGY",
  "type": "$RELEASE_TYPE"
}
EOF

capture_request "POST" "$RELEASES_URL" "$FLOW_DIR/release.create.payload.json" "$FLOW_DIR/release.create.body.json" "$FLOW_DIR/release.create.status" "$FLOW_DIR/release.create.headers"
create_release_status=$(<"$FLOW_DIR/release.create.status")
[[ "$create_release_status" == "201" ]] || fail_with_response "release create failed with status $create_release_status" "$FLOW_DIR/release.create.body.json" "$FLOW_DIR/release.create.headers"

release_id=$(json_get "$FLOW_DIR/release.create.body.json" '.data.id')
if [[ -z "$release_id" ]]; then
  release_id=$(json_get "$FLOW_DIR/release.create.body.json" '.resource_id')
fi
[[ -n "$release_id" ]] || fail_with_response "release id missing in create response" "$FLOW_DIR/release.create.body.json" "$FLOW_DIR/release.create.headers"

echo "release_id=$release_id"
print_request_context "$FLOW_DIR/release.create.body.json" "$FLOW_DIR/release.create.headers" || true

release_url="$RELEASES_URL/$release_id"
release_body="$FLOW_DIR/release.get.body.json"
release_status_file="$FLOW_DIR/release.get.status"
release_headers="$FLOW_DIR/release.get.headers"

if [[ "$WAIT_FOR_RELEASE" == "true" ]]; then
  release_start_ts=$(date +%s)
  while true; do
    capture_request "GET" "$release_url" "" "$release_body" "$release_status_file" "$release_headers"
    read_release_status=$(<"$release_status_file")
    [[ "$read_release_status" == "200" ]] || fail_with_response "release read failed with status $read_release_status" "$release_body" "$release_headers"

    release_status=$(json_get "$release_body" '.data.status')
    current_step=$(jq -r '
      (
        (.data.steps // [])
        | map(select(.status == "Running"))
        | first
        | .code
      ) // empty
    ' "$release_body")
    echo "release_id=$release_id status=${release_status:-unknown} running_step=${current_step:-}"

    if release_status_is_terminal "$release_status"; then
      break
    fi

    now_ts=$(date +%s)
    if (( now_ts - release_start_ts >= RELEASE_TIMEOUT_SECONDS )); then
      fail_with_response "timed out waiting for release $release_id to reach a terminal status" "$release_body" "$release_headers"
    fi

    sleep "$POLL_INTERVAL_SECONDS"
  done

  final_status=$(json_get "$release_body" '.data.status')
  case "$final_status" in
    Succeeded)
      echo "release_result=Succeeded"
      ;;
    Failed|RolledBack|SyncFailed)
      fail_with_response "release $release_id finished with terminal status $final_status" "$release_body" "$release_headers"
      ;;
    *)
      echo "release_result=$final_status"
      ;;
  esac
else
  capture_request "GET" "$LIST_RELEASES_URL" "" "$FLOW_DIR/releases.list.body.json" "$FLOW_DIR/releases.list.status" "$FLOW_DIR/releases.list.headers"
  list_release_status=$(<"$FLOW_DIR/releases.list.status")
  [[ "$list_release_status" == "200" ]] || fail_with_response "release list failed with status $list_release_status" "$FLOW_DIR/releases.list.body.json" "$FLOW_DIR/releases.list.headers"
  echo "release_result=Created"
fi

echo "application_id=$APPLICATION_ID"
echo "environment_id=$ENVIRONMENT_ID"
if [[ -n "$SERVICE_NAME" ]]; then
  echo "service_name=$SERVICE_NAME"
fi
echo "git_revision=$GIT_REVISION"
echo "strategy=$STRATEGY"
echo "manifest_created=$manifest_created"
echo "flow_dir=$FLOW_DIR"
