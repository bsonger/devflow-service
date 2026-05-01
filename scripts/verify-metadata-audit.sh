#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

check() {
  local description="$1"
  local pattern="$2"
  shift 2
  local files=("$@")

  if rg -n --fixed-strings "$pattern" "${files[@]}" >/dev/null; then
    echo "[pass] $description"
  else
    echo "[fail] $description" >&2
    echo "        missing pattern: $pattern" >&2
    echo "        searched files: ${files[*]}" >&2
    exit 1
  fi
}

check \
  "release bundle writes canonical release/application/environment labels" \
  "model.ReleaseIDLabel:          release.ID.String()," \
  internal/release/service/release_bundle.go
check \
  "release bundle writes canonical application label" \
  "model.ReleaseApplicationLabel: release.ApplicationID.String()," \
  internal/release/service/release_bundle.go
check \
  "release bundle writes canonical environment label" \
  "model.ReleaseEnvironmentLabel: strings.TrimSpace(release.EnvironmentID)," \
  internal/release/service/release_bundle.go

check \
  "Argo Application metadata mirrors release identity labels" \
  "model.ReleaseIDLabel:          release.ID.String()," \
  internal/release/service/release.go
check \
  "Argo ignore-differences includes restartedAt annotation" \
  "/spec/template/metadata/annotations/kubectl.kubernetes.io~1restartedAt" \
  internal/release/service/release.go internal/release/service/release_argo_test.go

check \
  "runtime restart path patches kubectl restartedAt" \
  '"kubectl.kubernetes.io/restartedAt":"%s"' \
  internal/runtime/service/service.go
check \
  "runtime observer parses kubectl restartedAt from pod-template annotations" \
  'annotations["kubectl.kubernetes.io/restartedAt"]' \
  internal/runtime/observer/kubernetes_runtime.go

check \
  "runtime observer requires release application label" \
  "releasedomain.ReleaseApplicationLabel" \
  internal/runtime/observer/kubernetes_runtime.go
check \
  "runtime observer requires release environment label" \
  "releasedomain.ReleaseEnvironmentLabel" \
  internal/runtime/observer/kubernetes_runtime.go
check \
  "runtime observer requires non-empty release id label" \
  "if strings.TrimSpace(labels[releasedomain.ReleaseIDLabel]) == \"\" {" \
  internal/runtime/observer/kubernetes_runtime.go

check \
  "drift proof documents the meta-service out-of-sync resource" \
  "meta-service" \
  docs/resources/metadata-drift-proof.md
check \
  "drift proof documents the drifting restartedAt field" \
  "kubectl.kubernetes.io/restartedAt" \
  docs/resources/metadata-drift-proof.md
check \
  "drift proof names the runtime service writing path" \
  "internal/runtime/service/service.go" \
  docs/resources/metadata-drift-proof.md
check \
  "drift proof preserves the open Argo question" \
  "Open question" \
  docs/resources/metadata-drift-proof.md

check \
  "metadata audit links to the drift proof" \
  "docs/resources/metadata-drift-proof.md" \
  docs/resources/metadata-contract-audit.md
check \
  "metadata audit links to the focused verifier" \
  "bash scripts/verify-metadata-audit.sh" \
  docs/resources/metadata-contract-audit.md

echo "metadata audit verification passed"
