#!/usr/bin/env bash
set -euo pipefail

GRAFANA_URL="${GRAFANA_URL:-https://grafana.bei.com}"
GRAFANA_USER="${GRAFANA_USER:-admin}"
GRAFANA_PASS="${GRAFANA_PASS:-admin}"
DRY_RUN=false

if [[ "${1:-}" == "--dry-run" ]]; then
  DRY_RUN=true
fi

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
prod_dir="$repo_root/deployments/devflow/grafana/dashboards"
pre_dir="$repo_root/deployments/pre-production/grafana/dashboards"
old_folder_uid="ef9tqdkc8vtoge"

curl_api() {
  local method="$1" path="$2" data_file="${3:-}"
  if [[ -n "$data_file" ]]; then
    curl -ksu "$GRAFANA_USER:$GRAFANA_PASS" -H 'Content-Type: application/json' -X "$method" "$GRAFANA_URL$path" --data-binary "@$data_file"
  else
    curl -ksu "$GRAFANA_USER:$GRAFANA_PASS" -X "$method" "$GRAFANA_URL$path"
  fi
}

folder_json() {
  local uid="$1" title="$2"
  python3 - <<'PY' "$uid" "$title"
import json,sys
print(json.dumps({"uid":sys.argv[1],"title":sys.argv[2]}, ensure_ascii=False))
PY
}

ensure_folder() {
  local uid="$1" title="$2"
  local tmp="$(mktemp)"
  if curl -ksfu "$GRAFANA_USER:$GRAFANA_PASS" "$GRAFANA_URL/api/folders/$uid" > /dev/null; then
    echo "reuse folder: $uid ($title)"
    rm -f "$tmp"
    return
  fi
  folder_json "$uid" "$title" > "$tmp"
  if $DRY_RUN; then
    echo "create folder: $uid ($title)"
  else
    curl_api POST "/api/folders" "$tmp" > /dev/null
    echo "created folder: $uid ($title)"
  fi
  rm -f "$tmp"
}

import_dashboards() {
  local folder_uid="$1" dashboard_dir="$2" label="$3"
  for path in "$dashboard_dir"/*.json; do
    local tmp="$(mktemp)"
    python3 - <<'PY' "$path" "$folder_uid" > "$tmp"
import json,sys
path,folder_uid=sys.argv[1],sys.argv[2]
dash=json.load(open(path))
print(json.dumps({"dashboard":dash,"folderUid":folder_uid,"overwrite":True}, ensure_ascii=False))
PY
    if $DRY_RUN; then
      echo "import [$label]: $(basename "$path") -> folder $folder_uid"
    else
      local resp
      resp="$(curl_api POST "/api/dashboards/db" "$tmp")"
      echo "imported [$label]: $(basename "$path") :: $resp"
    fi
    rm -f "$tmp"
  done
}

list_folder_dashboards() {
  local folder_uid="$1"
  local tmp="$(mktemp)"
  curl -ksu "$GRAFANA_USER:$GRAFANA_PASS" "$GRAFANA_URL/api/search?folderIds=" > /dev/null 2>&1 || true
  curl -ksu "$GRAFANA_USER:$GRAFANA_PASS" "$GRAFANA_URL/api/search" > "$tmp"
  python3 - <<'PY' "$tmp" "$folder_uid"
import json,sys
items=json.load(open(sys.argv[1]))
folder_uid=sys.argv[2]
rows=[i for i in items if i.get('folderUid')==folder_uid]
print(json.dumps(rows, ensure_ascii=False))
PY
  rm -f "$tmp"
}

delete_old_folder_if_empty() {
  local tmp="$(mktemp)"
  curl -ksu "$GRAFANA_USER:$GRAFANA_PASS" "$GRAFANA_URL/api/search" > "$tmp"
  local count
  count="$(python3 - <<'PY' "$tmp" "$old_folder_uid"
import json,sys
items=json.load(open(sys.argv[1]))
folder_uid=sys.argv[2]
print(sum(1 for i in items if i.get('folderUid')==folder_uid and i.get('type')=='dash-db'))
PY
)"
  rm -f "$tmp"
  if $DRY_RUN; then
    echo "old folder dashboard count: $count"
    if [[ "$count" == "0" ]]; then
      echo "delete empty old folder: $old_folder_uid"
    fi
    return
  fi
  if [[ "$count" == "0" ]]; then
    curl_api DELETE "/api/folders/$old_folder_uid" > /dev/null || true
    echo "deleted empty old folder: $old_folder_uid"
  else
    echo "old folder not empty; leaving in place: $count dashboards remain"
  fi
}

print_summary() {
  local tmp="$(mktemp)"
  curl -ksu "$GRAFANA_USER:$GRAFANA_PASS" "$GRAFANA_URL/api/search" > "$tmp"
  python3 - <<'PY' "$tmp"
import json,sys
items=json.load(open(sys.argv[1]))
for uid in ('devflow-production','devflow-pre-production'):
    folder=next((i for i in items if i.get('type')=='dash-folder' and i.get('uid')==uid), None)
    if folder:
        print(f"folder {uid}: {folder.get('url')}")
        for d in sorted([i for i in items if i.get('folderUid')==uid], key=lambda x:x.get('title','')):
            print(f"  - {d.get('title')}: {d.get('url')}")
PY
  rm -f "$tmp"
}

echo "Mode: $([[ "$DRY_RUN" == true ]] && echo DRY_RUN || echo APPLY)"
echo "Grafana: $GRAFANA_URL"

essure_prod_uid="devflow-production"
essure_pre_uid="devflow-pre-production"
ensure_folder "$essure_prod_uid" "devflow-production"
ensure_folder "$essure_pre_uid" "devflow-pre-production"
import_dashboards "$essure_prod_uid" "$prod_dir" "prod"
import_dashboards "$essure_pre_uid" "$pre_dir" "preprod"
delete_old_folder_if_empty
print_summary
