#!/usr/bin/env bash
set -euo pipefail

PROMETHEUS_URL="${PROMETHEUS_URL:-https://prometheus.bei.com}"
QUERY="${QUERY:-http_server_requests_total{deployment_environment_name=\"pre-production\",http_response_status_class=\"5xx\"}}"
START="${START:-$(date -u -v-1H +%s 2>/dev/null || date -u -d '1 hour ago' +%s)}"
END="${END:-$(date -u +%s)}"

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

response="$(
  curl -fsS --get "$PROMETHEUS_URL/api/v1/query_exemplars" \
    --data-urlencode "query=$QUERY" \
    --data-urlencode "start=$START" \
    --data-urlencode "end=$END"
)"

printf '%s\n' "$response" | grep -q '"status":"success"' || fail "Prometheus exemplar query failed"
printf '%s\n' "$response" | grep -q '"trace_id"' || fail "no trace_id exemplar found for query: $QUERY"
printf '%s\n' "$response" | grep -q '"span_id"' || fail "no span_id exemplar found for query: $QUERY"

echo "INFO: Prometheus exemplars include trace_id/span_id for 5xx HTTP metrics."
