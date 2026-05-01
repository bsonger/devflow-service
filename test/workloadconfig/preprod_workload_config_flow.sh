#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  PREPROD_BASE_URL=https://... \
  PREPROD_AUTH_HEADER='Authorization: Bearer <token>' \
  APPLICATION_ID=<uuid> \
  WORKLOAD_CONFIG_ID=<uuid> \
  bash test/workloadconfig/preprod_workload_config_flow.sh

Required environment variables:
  PREPROD_BASE_URL      Base URL for the shared ingress, e.g. https://preprod.example.com
  APPLICATION_ID        Target application_id for read-after-write verification
  WORKLOAD_CONFIG_ID    Target workload-config row id to update and re-read

Authentication options (choose one):
  PREPROD_AUTH_HEADER   Full header line, e.g. 'Authorization: Bearer <token>'
  PREPROD_COOKIE_HEADER Full cookie header line, e.g. 'Cookie: session=<value>'

Optional environment variables:
  FLOW_DIR              Output directory for captured status/body files
  EXPECTED_OUTCOME      clean_round_trip | invalid_argument | failed_precondition | missing_after_update
  SIZE_CLASS            Supported value: small | medium | large | xlarge (default: medium)
  REPLICAS              Integer replicas value (default: 1)
  SERVICE_ACCOUNT_NAME  Optional string to preserve/set service account name
  ENV_NAME              Repeated env row name used for the probe payload (default: WORKLOAD_FLOW_CHECK)
  ENV_VALUE             Repeated env row value used for the probe payload (default: roundtrip-ok)
  LIVENESS_PATH         Probe path (default: /healthz)
  LIVENESS_PORT         Probe port (default: http)
  READINESS_PATH        Probe path (default: /readyz)
  READINESS_PORT        Probe port (default: http)

This script captures:
  - list-by-application response
  - pre-update get-by-id response
  - update status/body
  - read-after-write get-by-id response

It intentionally uses only supported write fields:
  application_id, replicas, service_account_name, resources.size_class,
  probes.{liveness,readiness}, and repeated env rows.

If EXPECTED_OUTCOME is set, the script enforces one of:
  clean_round_trip      2xx update and read-after-write row still present
  invalid_argument      400 update with invalid_argument body
  failed_precondition   412 update with failed_precondition body
  missing_after_update  2xx/4xx update followed by 404 or list omission on reread
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
  python3 - "$file" "$expr" <<'PY'
import json, sys
path, expr = sys.argv[1], sys.argv[2]
with open(path, 'r', encoding='utf-8') as fh:
    data = json.load(fh)
value = data
for token in expr.split('.'):
    if token == '':
        continue
    if isinstance(value, list):
        value = value[int(token)]
    else:
        value = value[token]
if isinstance(value, (dict, list)):
    print(json.dumps(value, separators=(',', ':')))
else:
    print(value)
PY
}

jq_mask_summary() {
  local file=$1
  jq '{
    id: .data.id,
    application_id: .data.application_id,
    replicas: .data.replicas,
    service_account_name: (.data.service_account_name // ""),
    size_class: (.data.resources.size_class // ""),
    liveness: (.data.probes.liveness // null),
    readiness: (.data.probes.readiness // null),
    env_names: ((.data.env // []) | map(.name)),
    labels: (.data.labels // {}),
    annotations: ((.data.annotations // {}) | keys)
  }' "$file"
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

assert_expected_outcome() {
  local expected=$1
  local update_status read_status list_file read_file update_body
  update_status=$(<"$FLOW_DIR/update.status")
  read_status=$(<"$FLOW_DIR/read_after_write.status")
  list_file="$FLOW_DIR/list_after_update.body.json"
  read_file="$FLOW_DIR/read_after_write.body.json"
  update_body="$FLOW_DIR/update.body.json"

  case "$expected" in
    clean_round_trip)
      [[ "$update_status" =~ ^20[04]$ ]] || { echo "expected clean round-trip update 2xx, got $update_status" >&2; exit 1; }
      [[ "$read_status" == "200" ]] || { echo "expected read-after-write 200, got $read_status" >&2; exit 1; }
      local read_id read_size read_app
      read_id=$(json_get "$read_file" data.id)
      read_app=$(json_get "$read_file" data.application_id)
      read_size=$(json_get "$read_file" data.resources.size_class)
      [[ "$read_id" == "$WORKLOAD_CONFIG_ID" ]] || { echo "read-after-write id mismatch: $read_id" >&2; exit 1; }
      [[ "$read_app" == "$APPLICATION_ID" ]] || { echo "read-after-write application_id mismatch: $read_app" >&2; exit 1; }
      [[ "$read_size" == "$SIZE_CLASS" ]] || { echo "read-after-write size_class mismatch: $read_size" >&2; exit 1; }
      ;;
    invalid_argument)
      [[ "$update_status" == "400" ]] || { echo "expected invalid_argument 400, got $update_status" >&2; exit 1; }
      jq -e '.error.code == "invalid_argument"' "$update_body" >/dev/null
      ;;
    failed_precondition)
      [[ "$update_status" == "412" ]] || { echo "expected failed_precondition 412, got $update_status" >&2; exit 1; }
      jq -e '.error.code == "failed_precondition"' "$update_body" >/dev/null
      ;;
    missing_after_update)
      if [[ "$read_status" == "404" ]]; then
        return 0
      fi
      jq -e --arg id "$WORKLOAD_CONFIG_ID" '.data | map(select(.id == $id)) | length == 0' "$list_file" >/dev/null
      ;;
    *)
      echo "unsupported EXPECTED_OUTCOME: $expected" >&2
      exit 1
      ;;
  esac
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

require_cmd curl
require_cmd jq
require_cmd python3
require_env PREPROD_BASE_URL
require_env APPLICATION_ID
require_env WORKLOAD_CONFIG_ID

AUTH_HEADER=${PREPROD_AUTH_HEADER:-}
COOKIE_HEADER=${PREPROD_COOKIE_HEADER:-}
if [[ -z "$AUTH_HEADER" && -z "$COOKIE_HEADER" ]]; then
  echo "provide PREPROD_AUTH_HEADER or PREPROD_COOKIE_HEADER" >&2
  usage >&2
  exit 1
fi

FLOW_DIR=${FLOW_DIR:-test/workloadconfig/.tmp/preprod-flow}
EXPECTED_OUTCOME=${EXPECTED_OUTCOME:-clean_round_trip}
SIZE_CLASS=${SIZE_CLASS:-medium}
REPLICAS=${REPLICAS:-1}
SERVICE_ACCOUNT_NAME=${SERVICE_ACCOUNT_NAME:-default}
ENV_NAME=${ENV_NAME:-WORKLOAD_FLOW_CHECK}
ENV_VALUE=${ENV_VALUE:-roundtrip-ok}
LIVENESS_PATH=${LIVENESS_PATH:-/healthz}
LIVENESS_PORT=${LIVENESS_PORT:-http}
READINESS_PATH=${READINESS_PATH:-/readyz}
READINESS_PORT=${READINESS_PORT:-http}

BASE_URL=${PREPROD_BASE_URL%/}
LIST_URL="$BASE_URL/api/v1/config/workload-configs?application_id=$APPLICATION_ID"
ITEM_URL="$BASE_URL/api/v1/config/workload-configs/$WORKLOAD_CONFIG_ID"

mkdir -p "$FLOW_DIR"

cat > "$FLOW_DIR/update.payload.json" <<EOF
{
  "application_id": "$APPLICATION_ID",
  "replicas": $REPLICAS,
  "service_account_name": "$SERVICE_ACCOUNT_NAME",
  "resources": {
    "size_class": "$SIZE_CLASS"
  },
  "probes": {
    "liveness": {
      "path": "$LIVENESS_PATH",
      "port": "$LIVENESS_PORT",
      "period_seconds": 10,
      "timeout_seconds": 5,
      "failure_threshold": 3
    },
    "readiness": {
      "path": "$READINESS_PATH",
      "port": "$READINESS_PORT",
      "period_seconds": 10,
      "timeout_seconds": 5,
      "failure_threshold": 3
    }
  },
  "env": [
    {
      "name": "$ENV_NAME",
      "value": "$ENV_VALUE"
    }
  ]
}
EOF

capture_request GET "$LIST_URL" "" "$FLOW_DIR/list_before_update.body.json" "$FLOW_DIR/list_before_update.status"
capture_request GET "$ITEM_URL" "" "$FLOW_DIR/read_before_update.body.json" "$FLOW_DIR/read_before_update.status"
capture_request PUT "$ITEM_URL" "$FLOW_DIR/update.payload.json" "$FLOW_DIR/update.body.json" "$FLOW_DIR/update.status"
capture_request GET "$ITEM_URL" "" "$FLOW_DIR/read_after_write.body.json" "$FLOW_DIR/read_after_write.status"
capture_request GET "$LIST_URL" "" "$FLOW_DIR/list_after_update.body.json" "$FLOW_DIR/list_after_update.status"

assert_expected_outcome "$EXPECTED_OUTCOME"

echo "=== workload-config pre-production flow ==="
echo "list-before-update status: $(<"$FLOW_DIR/list_before_update.status")"
echo "read-before-update status: $(<"$FLOW_DIR/read_before_update.status")"
echo "update status: $(<"$FLOW_DIR/update.status")"
echo "read-after-write status: $(<"$FLOW_DIR/read_after_write.status")"
echo "list-after-update status: $(<"$FLOW_DIR/list_after_update.status")"

echo "=== canonical row summary before update ==="
if [[ $(<"$FLOW_DIR/read_before_update.status") == "200" ]]; then
  jq_mask_summary "$FLOW_DIR/read_before_update.body.json"
else
  cat "$FLOW_DIR/read_before_update.body.json"
fi

echo "=== update payload (safe fields only) ==="
cat "$FLOW_DIR/update.payload.json"

echo "=== update response ==="
cat "$FLOW_DIR/update.body.json"

echo "=== read-after-write summary ==="
if [[ $(<"$FLOW_DIR/read_after_write.status") == "200" ]]; then
  jq_mask_summary "$FLOW_DIR/read_after_write.body.json"
else
  cat "$FLOW_DIR/read_after_write.body.json"
fi

echo "=== list-after-update ids ==="
jq '.data | map({id, application_id, size_class: .resources.size_class})' "$FLOW_DIR/list_after_update.body.json"

echo "Outcome check passed: $EXPECTED_OUTCOME"
