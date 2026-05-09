package observer

import (
	"strings"
	"testing"

	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
	runtimedomain "github.com/bsonger/devflow-service/internal/runtime/domain"
	"github.com/google/uuid"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestReleaseOwnedSelector(t *testing.T) {
	appID := uuid.New()
	selector, err := releaseOwnedSelector(&runtimedomain.RuntimeSpec{ApplicationID: appID, Environment: "prod"})
	if err != nil {
		t.Fatalf("releaseOwnedSelector failed: %v", err)
	}
	wantParts := []string{
		releasedomain.ReleaseApplicationLabel + "=" + appID.String(),
		releasedomain.ReleaseEnvironmentLabel + "=prod",
	}
	for _, part := range wantParts {
		if !strings.Contains(selector, part) {
			t.Fatalf("selector %q missing %q", selector, part)
		}
	}
	if strings.Contains(selector, "otel.devflow.io/") {
		t.Fatalf("selector must stay label-only, got %q", selector)
	}
	if _, err := releaseOwnedSelector(&runtimedomain.RuntimeSpec{ApplicationID: appID}); err == nil {
		t.Fatal("expected environment-missing selector error")
	}
}

func TestLabelsMatchRuntimeSpec(t *testing.T) {
	appID := uuid.New()
	releaseID := uuid.New()
	spec := &runtimedomain.RuntimeSpec{ApplicationID: appID, Environment: "prod"}

	if !labelsMatchRuntimeSpec(spec, map[string]string{
		releasedomain.ReleaseApplicationLabel: appID.String(),
		releasedomain.ReleaseEnvironmentLabel: "prod",
		releasedomain.ReleaseIDLabel:          releaseID.String(),
	}) {
		t.Fatal("expected release-owned labels to match runtime spec")
	}

	if labelsMatchRuntimeSpec(spec, map[string]string{
		releasedomain.ReleaseApplicationLabel: appID.String(),
		releasedomain.ReleaseEnvironmentLabel: "prod",
	}) {
		t.Fatal("expected missing release id label to fail match")
	}
}

func TestLabelsMatchRuntimeSpecIgnoresAnnotationStyleMetadata(t *testing.T) {
	appID := uuid.New()
	releaseID := uuid.New()
	spec := &runtimedomain.RuntimeSpec{ApplicationID: appID, Environment: "prod"}
	deployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "demo-api",
			Labels: map[string]string{
				releasedomain.ReleaseApplicationLabel: appID.String(),
				releasedomain.ReleaseEnvironmentLabel: "prod",
			},
			Annotations: map[string]string{
				"otel.devflow.io/release-id": releaseID.String(),
			},
		},
	}

	if deploymentMatchesRuntimeSpec(spec, deployment) {
		t.Fatalf("expected deployment correlation to reject annotation-only release metadata: labels=%#v annotations=%#v", deployment.Labels, deployment.Annotations)
	}
}

func TestRolloutMatchesRuntimeSpec(t *testing.T) {
	appID := uuid.New()
	releaseID := uuid.New()
	spec := &runtimedomain.RuntimeSpec{ApplicationID: appID, Environment: "prod"}
	rollout := unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{
			"name": "demo-api",
			"labels": map[string]any{
				releasedomain.ReleaseApplicationLabel: appID.String(),
				releasedomain.ReleaseEnvironmentLabel: "prod",
				releasedomain.ReleaseIDLabel:          releaseID.String(),
			},
		},
	}}

	if !rolloutMatchesRuntimeSpec(spec, rollout) {
		t.Fatal("expected release-owned rollout labels to match runtime spec")
	}
}

func TestSelectReleaseOwnedRolloutPrefersNamedMatch(t *testing.T) {
	appID := uuid.New()
	spec := &runtimedomain.RuntimeSpec{ApplicationID: appID, Environment: "prod"}
	items := []unstructured.Unstructured{
		newTestRollout("demo-api-canary", appID, "prod"),
		newTestRollout("demo-api", appID, "prod"),
	}
	picked, err := selectReleaseOwnedRollout(spec, "demo-api", items)
	if err != nil {
		t.Fatalf("selectReleaseOwnedRollout failed: %v", err)
	}
	if picked == nil || picked.GetName() != "demo-api" {
		t.Fatalf("picked = %#v", picked)
	}
}

func TestSummarizeRolloutStatus(t *testing.T) {
	healthy := newTestRollout("demo-api", uuid.New(), "prod")
	healthy.Object["status"] = map[string]any{
		"phase":         "Healthy",
		"readyReplicas": int64(2),
	}
	healthy.Object["spec"] = map[string]any{"replicas": int64(2)}
	if got := summarizeRolloutStatus(&healthy); got != "Healthy" {
		t.Fatalf("healthy status = %q", got)
	}

	progressing := newTestRollout("demo-api", uuid.New(), "prod")
	progressing.Object["status"] = map[string]any{
		"phase":         "Progressing",
		"readyReplicas": int64(1),
	}
	progressing.Object["spec"] = map[string]any{"replicas": int64(2)}
	if got := summarizeRolloutStatus(&progressing); got != "Progressing" {
		t.Fatalf("progressing status = %q", got)
	}

	degraded := newTestRollout("demo-api", uuid.New(), "prod")
	degraded.Object["status"] = map[string]any{"phase": "Degraded"}
	if got := summarizeRolloutStatus(&degraded); got != "Degraded" {
		t.Fatalf("degraded status = %q", got)
	}
}

func newTestRollout(name string, appID uuid.UUID, environment string) unstructured.Unstructured {
	return unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Rollout",
		"metadata": map[string]any{
			"name": name,
			"labels": map[string]any{
				releasedomain.ReleaseApplicationLabel: appID.String(),
				releasedomain.ReleaseEnvironmentLabel: environment,
				releasedomain.ReleaseIDLabel:          uuid.New().String(),
			},
		},
	}}
}
