package observer

import (
	"testing"

	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
	runtimedomain "github.com/bsonger/devflow-service/internal/runtime/domain"
	"github.com/google/uuid"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDeriveReleaseRolloutContext(t *testing.T) {
	releaseID := uuid.New()
	applicationID := uuid.New()
	workload := &runtimedomain.RuntimeObservedWorkload{
		Environment:  "prod",
		Namespace:    "demo-ns",
		WorkloadName: "demo-api",
		Labels: map[string]string{
			releasedomain.ReleaseIDLabel:          releaseID.String(),
			releasedomain.ReleaseApplicationLabel: applicationID.String(),
			releasedomain.ReleaseEnvironmentLabel: "env-prod",
		},
	}

	ctx, reason := deriveReleaseRolloutContext(workload)
	if reason != "" {
		t.Fatalf("reason = %q", reason)
	}
	if ctx.ReleaseID != releaseID {
		t.Fatalf("releaseID = %s", ctx.ReleaseID)
	}
	if ctx.ApplicationID != applicationID {
		t.Fatalf("applicationID = %s", ctx.ApplicationID)
	}
	if ctx.EnvironmentID != "env-prod" {
		t.Fatalf("environmentID = %q", ctx.EnvironmentID)
	}
	if ctx.Namespace != "demo-ns" {
		t.Fatalf("namespace = %q", ctx.Namespace)
	}
	if ctx.DeploymentName != "demo-api" {
		t.Fatalf("deploymentName = %q", ctx.DeploymentName)
	}
}

func TestDeriveReleaseRolloutContextFallsBackToWorkloadEnvironment(t *testing.T) {
	releaseID := uuid.New()
	applicationID := uuid.New()
	workload := &runtimedomain.RuntimeObservedWorkload{
		Environment:  "prod",
		Namespace:    "demo-ns",
		WorkloadName: "demo-api",
		Labels: map[string]string{
			releasedomain.ReleaseIDLabel:          releaseID.String(),
			releasedomain.ReleaseApplicationLabel: applicationID.String(),
		},
	}

	ctx, reason := deriveReleaseRolloutContext(workload)
	if reason != "" {
		t.Fatalf("reason = %q", reason)
	}
	if ctx.EnvironmentID != "prod" {
		t.Fatalf("environmentID = %q", ctx.EnvironmentID)
	}
}

func TestDeriveReleaseRolloutContextMissingMetadata(t *testing.T) {
	workload := &runtimedomain.RuntimeObservedWorkload{
		Namespace:    "demo-ns",
		WorkloadName: "demo-api",
		Labels:       map[string]string{},
	}

	_, reason := deriveReleaseRolloutContext(workload)
	if reason != "missing_release_id_label" {
		t.Fatalf("reason = %q", reason)
	}
}

func TestDeriveReleaseRolloutStateMissingDeployment(t *testing.T) {
	status, message, progress, stateKey := deriveReleaseRolloutState("demo-ns", "demo-api", nil)
	if status != releasedomain.StepRunning {
		t.Fatalf("status = %q", status)
	}
	if progress != 10 {
		t.Fatalf("progress = %d", progress)
	}
	if stateKey != "missing" {
		t.Fatalf("stateKey = %q", stateKey)
	}
	if message == "" {
		t.Fatal("expected message")
	}
}

func TestDeriveReleaseRolloutStateSucceeded(t *testing.T) {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "demo-api", Generation: 3},
		Spec:       appsv1.DeploymentSpec{Replicas: int32Ptr(2)},
		Status: appsv1.DeploymentStatus{
			ObservedGeneration:  3,
			UpdatedReplicas:     2,
			ReadyReplicas:       2,
			AvailableReplicas:   2,
			UnavailableReplicas: 0,
		},
	}
	status, _, progress, stateKey := deriveReleaseRolloutState("demo-ns", "demo-api", deployment)
	if status != releasedomain.StepSucceeded {
		t.Fatalf("status = %q", status)
	}
	if progress != 100 {
		t.Fatalf("progress = %d", progress)
	}
	if stateKey == "" {
		t.Fatal("expected stateKey")
	}
}

func TestDeriveReleaseRolloutStateFailed(t *testing.T) {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "demo-api", Generation: 3},
		Spec:       appsv1.DeploymentSpec{Replicas: int32Ptr(2)},
		Status: appsv1.DeploymentStatus{
			ObservedGeneration: 2,
			Conditions: []appsv1.DeploymentCondition{
				{Type: appsv1.DeploymentProgressing, Status: "False", Reason: "ProgressDeadlineExceeded"},
			},
		},
	}
	status, _, progress, _ := deriveReleaseRolloutState("demo-ns", "demo-api", deployment)
	if status != releasedomain.StepFailed {
		t.Fatalf("status = %q", status)
	}
	if progress != 100 {
		t.Fatalf("progress = %d", progress)
	}
}

func TestDeriveReleaseRolloutStateRunning(t *testing.T) {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "demo-api", Generation: 3},
		Spec:       appsv1.DeploymentSpec{Replicas: int32Ptr(4)},
		Status: appsv1.DeploymentStatus{
			ObservedGeneration:  2,
			UpdatedReplicas:     2,
			ReadyReplicas:       1,
			AvailableReplicas:   1,
			UnavailableReplicas: 3,
			Conditions: []appsv1.DeploymentCondition{
				{Type: appsv1.DeploymentProgressing, Status: "True", Reason: "ReplicaSetUpdated"},
			},
		},
	}
	status, _, progress, _ := deriveReleaseRolloutState("demo-ns", "demo-api", deployment)
	if status != releasedomain.StepRunning {
		t.Fatalf("status = %q", status)
	}
	if progress <= 0 || progress >= 100 {
		t.Fatalf("progress = %d", progress)
	}
}

func TestPickPrimaryDeploymentPrefersNamedMatch(t *testing.T) {
	items := []appsv1.Deployment{
		{ObjectMeta: metav1.ObjectMeta{Name: "demo-api-canary"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "demo-api"}},
	}
	picked := pickPrimaryDeployment("demo-api", items)
	if picked == nil || picked.Name != "demo-api" {
		t.Fatalf("picked = %#v", picked)
	}
}

func TestProcessedKeysArePerReleaseAndState(t *testing.T) {
	observer := &ReleaseRolloutObserver{processed: map[string]string{}}
	releaseID := uuid.New().String()
	if observer.isProcessed(releaseID, "running|25") {
		t.Fatal("state should not be processed before mark")
	}
	observer.markProcessed(releaseID, "running|25")
	if !observer.isProcessed(releaseID, "running|25") {
		t.Fatal("expected processed state for same release/state key")
	}
	if observer.isProcessed(releaseID, "succeeded|100") {
		t.Fatal("different state key should not be treated as duplicate")
	}
	if observer.isProcessed(uuid.New().String(), "running|25") {
		t.Fatal("same state key on a different release should not be duplicate")
	}
}

func int32Ptr(v int32) *int32 {
	return &v
}
