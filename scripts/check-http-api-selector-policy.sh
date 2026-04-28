#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

fail() {
  echo "ERROR[api-selector-policy]: $*" >&2
  exit 1
}

info() {
  echo "INFO[api-selector-policy]: $*"
}

DOC_PATTERN='(POST|DELETE)[[:space:]]+/api/v1/[^[:space:]]*\?(application_id|environment_id|project_id|cluster_id|manifest_id|release_id|runtime_spec_id|resource_id|service_id|route_id)='

info "Checking docs for POST/DELETE selector query usage"
doc_matches="$(
  cd "$ROOT_DIR"
  rg -n "$DOC_PATTERN" docs README.md AGENTS.md || true
)"
[[ -z "$doc_matches" ]] || fail "POST/DELETE business selectors must not be carried in query strings in docs; use JSON body instead:\n$doc_matches"

info "Checking handlers for POST/DELETE selector query parsing"
code_matches="$(
  cd "$ROOT_DIR"
  python3 - <<'PY'
from pathlib import Path
import re
ROOT = Path('.').resolve()
files = sorted(ROOT.glob('internal/*/transport/http/*.go'))
selector_patterns = [
    'c.Query("application_id")',
    'c.Query("environment_id")',
    'c.Query("project_id")',
    'c.Query("cluster_id")',
    'c.Query("manifest_id")',
    'c.Query("release_id")',
    'c.Query("runtime_spec_id")',
    'c.Query("resource_id")',
    'httpx.ParseUUIDQuery(c, "application_id")',
    'httpx.ParseUUIDQuery(c, "project_id")',
    'httpx.ParseUUIDQuery(c, "cluster_id")',
    'httpx.ParseUUIDQuery(c, "manifest_id")',
    'httpx.ParseUUIDQuery(c, "release_id")',
    'httpx.ParseUUIDQuery(c, "runtime_spec_id")',
    'httpx.ParseUUIDQuery(c, "resource_id")',
    'httpx.ParseUUIDString(c, c.Query("application_id")',
    'httpx.ParseUUIDString(c, c.Query("environment_id")',
]
name_re = re.compile(r'func \(h \*Handler\) ([A-Za-z0-9_]+)\(c \*gin\.Context\) \{')
write_prefixes = ('Create', 'Delete', 'Update', 'Patch', 'Restart', 'Sync', 'Trigger', 'Validate', 'Rollout')
violations = []
for path in files:
    text = path.read_text()
    lines = text.splitlines()
    i = 0
    while i < len(lines):
        m = name_re.search(lines[i])
        if not m:
            i += 1
            continue
        name = m.group(1)
        start = i
        brace = lines[i].count('{') - lines[i].count('}')
        i += 1
        body_lines = []
        while i < len(lines):
            body_lines.append(lines[i])
            brace += lines[i].count('{') - lines[i].count('}')
            if brace <= 0:
                break
            i += 1
        body = '\n'.join(body_lines)
        if name.startswith(write_prefixes):
            for pat in selector_patterns:
                if pat in body:
                    violations.append(f"{path.relative_to(ROOT)}:{start+1}: {name} uses query selector pattern `{pat}`")
                    break
        i += 1
print('\n'.join(violations))
PY
)"
[[ -z "$code_matches" ]] || fail "non-GET handlers must not read business selectors from query strings; use JSON body instead:\n$code_matches"

info "HTTP API selector policy passed"
