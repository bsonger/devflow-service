package observer

import (
	"strings"
	"testing"

	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
	runtimedomain "github.com/bsonger/devflow-service/internal/runtime/domain"
	"github.com/google/uuid"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
