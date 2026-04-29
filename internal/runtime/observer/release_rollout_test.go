package observer

import (
	"testing"

	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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

func int32Ptr(v int32) *int32 {
	return &v
}
