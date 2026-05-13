#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  PREPROD_BASE_URL=https://devflow-pre-production.bei.com \
  MANIFEST_ID=<uuid> \
  ENVIRONMENT_ID=<uuid> \
  STRATEGY=rolling|blueGreen|canary \
  bash test/e2e/preprod_release_flow.sh

Required environment variables:
  PREPROD_BASE_URL   Base URL for the shared ingress
  MANIFEST_ID        Target manifest_id to deploy
  ENVIRONMENT_ID     Target environment_id
  STRATEGY           rolling | blueGreen | canary

Authentication options (optional):
  PREPROD_AUTH_HEADER    Full header line, e.g. 'Authorization: Bearer <token>'
  PREPROD_COOKIE_HEADER  Full header line, e.g. 'Cookie: session=<value>'

Optional environment variables:
  RELEASE_TYPE           Release type, default Upgrade
  FLOW_DIR               Output directory, default test/e2e/.tmp/preprod-release-flow
  POLL_INTERVAL_SECONDS  Poll interval, default 10
  RELEASE_TIMEOUT_SECONDS
                         End-to-end wait timeout, default 300
  EXPECTED_STATUS        Succeeded | Failed | Running | Pending | SyncFailed | RolledBack
                         default Succeeded
  EXPECTED_RUNNING_STEP  Optional step code expected to be Running when
                         EXPECTED_STATUS=Running, e.g. canary_30

Notes:
  - rolling and blueGreen are suitable success-path e2e flows
  - canary currently uses pause steps in the rendered rollout; treat it as a
    stage-observation flow unless the control plane adds automated promotion
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

assert_step_status() {
  local file=$1
  local step_code=$2
  local want=$3
  local got
  got=$(jq -r --arg code "$step_code" '.data.steps[] | select(.code == $code) | .status' "$file")
  if [[ "$got" != "$want" ]]; then
    echo "unexpected step status for $step_code: got $got want $want" >&2
    jq -r --arg code "$step_code" '.data.steps[] | select(.code == $code)' "$file" >&2
    exit 1
  fi
}

capture_request() {
  local method=$1
  local url=$2
  local body_file=$3
  local response_body=$4
  local response_status=$5

  local curl_args=(
    -sS
    -X "$method"
    -H 'Accept: application/json'
    -H 'Content-Type: application/json'
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
  printf '%s' "$status" > "$response_status"
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

require_cmd curl
require_cmd jq
require_env PREPROD_BASE_URL
require_env MANIFEST_ID
require_env ENVIRONMENT_ID
require_env STRATEGY

AUTH_HEADER=${PREPROD_AUTH_HEADER:-}
COOKIE_HEADER=${PREPROD_COOKIE_HEADER:-}
RELEASE_TYPE=${RELEASE_TYPE:-Upgrade}
FLOW_DIR=${FLOW_DIR:-test/e2e/.tmp/preprod-release-flow}
POLL_INTERVAL_SECONDS=${POLL_INTERVAL_SECONDS:-10}
RELEASE_TIMEOUT_SECONDS=${RELEASE_TIMEOUT_SECONDS:-300}
EXPECTED_STATUS=${EXPECTED_STATUS:-Succeeded}
EXPECTED_RUNNING_STEP=${EXPECTED_RUNNING_STEP:-}

case "$STRATEGY" in
  rolling|blueGreen|canary) ;;
  *)
    echo "unsupported STRATEGY: $STRATEGY" >&2
    exit 1
    ;;
esac

BASE_URL=${PREPROD_BASE_URL%/}
CREATE_URL="$BASE_URL/api/v1/release/releases"

mkdir -p "$FLOW_DIR"

cat > "$FLOW_DIR/create.payload.json" <<EOF
{
  "manifest_id": "$MANIFEST_ID",
  "environment_id": "$ENVIRONMENT_ID",
  "strategy": "$STRATEGY",
  "type": "$RELEASE_TYPE"
}
EOF

capture_request "POST" "$CREATE_URL" "$FLOW_DIR/create.payload.json" "$FLOW_DIR/create.body.json" "$FLOW_DIR/create.status"
create_status=$(<"$FLOW_DIR/create.status")
[[ "$create_status" == "201" ]] || {
  echo "release create failed with status $create_status" >&2
  cat "$FLOW_DIR/create.body.json" >&2
  exit 1
}

release_id=$(jq -r '.data.id' "$FLOW_DIR/create.body.json")
[[ -n "$release_id" && "$release_id" != "null" ]] || {
  echo "release id missing in create response" >&2
  cat "$FLOW_DIR/create.body.json" >&2
  exit 1
}

release_url="$CREATE_URL/$release_id"
start_ts=$(date +%s)

while true; do
  capture_request "GET" "$release_url" "" "$FLOW_DIR/release.body.json" "$FLOW_DIR/release.status"
  read_status=$(<"$FLOW_DIR/release.status")
  [[ "$read_status" == "200" ]] || {
    echo "release read failed with status $read_status" >&2
    cat "$FLOW_DIR/release.body.json" >&2
    exit 1
  }

  current_status=$(jq -r '.data.status' "$FLOW_DIR/release.body.json")
  echo "release_id=$release_id strategy=$STRATEGY status=$current_status"

  if [[ "$current_status" == "$EXPECTED_STATUS" ]]; then
    break
  fi

  case "$current_status" in
    Failed|SyncFailed|RolledBack)
      echo "release reached unexpected terminal status $current_status" >&2
      jq . "$FLOW_DIR/release.body.json" >&2
      exit 1
      ;;
  esac

  now_ts=$(date +%s)
  if (( now_ts - start_ts >= RELEASE_TIMEOUT_SECONDS )); then
    echo "timed out waiting for release $release_id to reach $EXPECTED_STATUS" >&2
    jq . "$FLOW_DIR/release.body.json" >&2
    exit 1
  fi

  sleep "$POLL_INTERVAL_SECONDS"
done

case "$EXPECTED_STATUS" in
  Succeeded)
    assert_step_status "$FLOW_DIR/release.body.json" "freeze_inputs" "Succeeded"
    assert_step_status "$FLOW_DIR/release.body.json" "render_deployment_bundle" "Succeeded"
    assert_step_status "$FLOW_DIR/release.body.json" "publish_bundle" "Succeeded"
    assert_step_status "$FLOW_DIR/release.body.json" "create_argocd_application" "Succeeded"
    case "$STRATEGY" in
      rolling)
        assert_step_status "$FLOW_DIR/release.body.json" "start_deployment" "Succeeded"
        assert_step_status "$FLOW_DIR/release.body.json" "observe_rollout" "Succeeded"
        ;;
      blueGreen)
        assert_step_status "$FLOW_DIR/release.body.json" "deploy_preview" "Succeeded"
        assert_step_status "$FLOW_DIR/release.body.json" "observe_preview" "Succeeded"
        assert_step_status "$FLOW_DIR/release.body.json" "switch_traffic" "Succeeded"
        assert_step_status "$FLOW_DIR/release.body.json" "verify_active" "Succeeded"
        ;;
      canary)
        assert_step_status "$FLOW_DIR/release.body.json" "deploy_canary" "Succeeded"
        assert_step_status "$FLOW_DIR/release.body.json" "canary_100" "Succeeded"
        ;;
    esac
    assert_step_status "$FLOW_DIR/release.body.json" "finalize_release" "Succeeded"
    ;;
  Running)
    assert_step_status "$FLOW_DIR/release.body.json" "freeze_inputs" "Succeeded"
    assert_step_status "$FLOW_DIR/release.body.json" "render_deployment_bundle" "Succeeded"
    assert_step_status "$FLOW_DIR/release.body.json" "publish_bundle" "Succeeded"
    assert_step_status "$FLOW_DIR/release.body.json" "create_argocd_application" "Succeeded"
    case "$STRATEGY" in
      rolling)
        assert_step_status "$FLOW_DIR/release.body.json" "start_deployment" "Succeeded"
        assert_step_status "$FLOW_DIR/release.body.json" "observe_rollout" "Running"
        ;;
      blueGreen)
        assert_step_status "$FLOW_DIR/release.body.json" "deploy_preview" "Succeeded"
        assert_step_status "$FLOW_DIR/release.body.json" "observe_preview" "Running"
        ;;
      canary)
        assert_step_status "$FLOW_DIR/release.body.json" "deploy_canary" "Succeeded"
        if [[ -n "$EXPECTED_RUNNING_STEP" ]]; then
          assert_step_status "$FLOW_DIR/release.body.json" "$EXPECTED_RUNNING_STEP" "Running"
        else
          jq -e '
            [
              .data.steps[]
              | select(.code == "canary_10" or .code == "canary_30" or .code == "canary_60" or .code == "canary_100")
              | select(.status == "Running" or .status == "Pending")
            ] | length >= 1
          ' "$FLOW_DIR/release.body.json" >/dev/null || {
            echo "expected at least one pending or running canary promotion step" >&2
            jq '{id:.data.id,status:.data.status,steps:.data.steps}' "$FLOW_DIR/release.body.json" >&2
            exit 1
          }
        fi
        ;;
    esac
    assert_step_status "$FLOW_DIR/release.body.json" "finalize_release" "Pending"
    ;;
esac

echo "release flow verified successfully: release_id=$release_id strategy=$STRATEGY expected_status=$EXPECTED_STATUS"
jq '{id:.data.id,status:.data.status,strategy:.data.strategy,type:.data.type,steps:.data.steps}' "$FLOW_DIR/release.body.json"
