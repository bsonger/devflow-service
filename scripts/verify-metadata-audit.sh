#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

check() {
  local description="$1"
  local pattern="$2"
  shift 2
  local files=("$@")

  if rg -n --fixed-strings -- "$pattern" "${files[@]}" >/dev/null; then
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
  "release bundle routes workload labels through the release-owned helper" \
  "labels := releaseWorkloadLabels(selectorName, workload.Labels, release)" \
  internal/release/service/release_bundle.go
check \
  "release bundle routes workload annotations through the supplementary filter" \
  "annotations := releaseSupplementaryAnnotations(workload.Annotations)" \
  internal/release/service/release_bundle.go
check \
  "release bundle defines restartedAt as a drift-prone filtered annotation" \
  '"kubectl.kubernetes.io/restartedAt": {}' \
  internal/release/service/release_bundle.go
check \
  "release supplementary annotation filter skips blocked keys" \
  "if _, blocked := releaseDriftProneAnnotationKeys[trimmedKey]; blocked {" \
  internal/release/service/release_bundle.go

check \
  "Argo Application metadata mirrors release identity labels" \
  "model.ReleaseIDLabel:          release.ID.String()," \
  internal/release/service/release.go
check \
  "Argo Application keeps trace annotations supplementary" \
  "oci.TraceIDAnnotation: sc.TraceID().String()," \
  internal/release/service/release.go
check \
  "Argo Application keeps parent span annotations supplementary" \
  "oci.SpanAnnotation:    sc.SpanID().String()," \
  internal/release/service/release.go
check \
  "Argo ignore-differences includes restartedAt annotation for rolling deployments" \
  "assertRestartedAtIgnoreDifference(t, app.Spec.IgnoreDifferences, \"apps\", \"Deployment\")" \
  internal/release/service/release_argo_test.go
check \
  "Argo ignore-differences retargets restartedAt annotation to rollout workloads when strategy requires it" \
  "assertRestartedAtIgnoreDifference(t, app.Spec.IgnoreDifferences, \"argoproj.io\", \"Rollout\")" \
  internal/release/service/release_argo_test.go

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
  "runtime rollout context derives release identity from workload labels" \
  "uuid.Parse(strings.TrimSpace(workload.Labels[releasedomain.ReleaseIDLabel]))" \
  internal/runtime/observer/release_rollout.go
check \
  "runtime rollout context derives application identity from workload labels" \
  "uuid.Parse(strings.TrimSpace(workload.Labels[releasedomain.ReleaseApplicationLabel]))" \
  internal/runtime/observer/release_rollout.go
check \
  "runtime rollout context derives environment identity from workload labels" \
  "environmentID := strings.TrimSpace(workload.Labels[releasedomain.ReleaseEnvironmentLabel])" \
  internal/runtime/observer/release_rollout.go

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
  "metadata audit documents the desired-state annotation filter seam" \
  "release rendering now treats workload annotations as an **explicitly filtered supplementary surface**" \
  docs/resources/metadata-contract-audit.md
check \
  "metadata audit documents restartedAt as filtered from desired state" \
  "Explicitly filtered from rendered workload metadata because it is runtime-mutated and drift-prone." \
  docs/resources/metadata-contract-audit.md
check \
  "metadata audit links to the drift proof" \
  "docs/resources/metadata-drift-proof.md" \
  docs/resources/metadata-contract-audit.md
check \
  "metadata audit links to the focused verifier" \
  "bash scripts/verify-metadata-audit.sh" \
  docs/resources/metadata-contract-audit.md

check \
  "release doc documents the desired-state workload annotation filter" \
  "- **Rendered workload and pod-template annotations** are filtered before publication so drift-prone runtime-mutated keys do not become desired-state contract by accident." \
  docs/resources/release.md
check \
  "release doc documents restartedAt exclusion from rendered desired state" \
  "are intentionally excluded from rendered desired state by default" \
  docs/resources/release.md
check \
  "release doc documents trace annotations as supplementary" \
  "treat them as supplementary diagnostics rather than business identity" \
  docs/resources/release.md

echo "metadata audit verification passed"
