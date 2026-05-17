package reconcile

import (
	"testing"

	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
	runtimedomain "github.com/bsonger/devflow-service/internal/runtime/domain"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestNormalizeReleaseObservedStateWorkloadBackedDeploymentRemainsRunningWithoutSpecGeneration(t *testing.T) {
	workload := &runtimedomain.RuntimeObservedWorkload{
		Namespace:           "devflow",
		WorkloadKind:        "Deployment",
		WorkloadName:        "demo-api",
		DesiredReplicas:     2,
		ReadyReplicas:       2,
		UpdatedReplicas:     2,
		AvailableReplicas:   2,
		UnavailableReplicas: 0,
		ObservedGeneration:  3,
	}

	state := NormalizeReleaseObservedState(workload)
	if state.Phase != releasedomain.StepRunning {
		t.Fatalf("phase = %q", state.Phase)
	}
	if state.Progress >= 100 {
		t.Fatalf("progress = %d", state.Progress)
	}
	if state.FinalizeState != nil {
		t.Fatalf("finalize = %#v", state.FinalizeState)
	}
	if len(state.StepWrites) != 1 {
		t.Fatalf("step writes = %#v", state.StepWrites)
	}
	if state.StepWrites[0].StepCode != "observe_rollout" {
		t.Fatalf("step writes = %#v", state.StepWrites)
	}
}

func TestNormalizeReleaseObservedStateDeploymentFailed(t *testing.T) {
	workload := &runtimedomain.RuntimeObservedWorkload{
		Namespace:          "devflow",
		WorkloadKind:       "Deployment",
		WorkloadName:       "demo-api",
		DesiredReplicas:    2,
		UpdatedReplicas:    1,
		ObservedGeneration: 2,
		Conditions: []runtimedomain.RuntimeObservedWorkloadCondition{
			{
				Type:   string(appsv1.DeploymentProgressing),
				Status: "False",
				Reason: "ProgressDeadlineExceeded",
			},
		},
	}

	state := NormalizeReleaseObservedState(workload)
	if state.Phase != releasedomain.StepFailed {
		t.Fatalf("phase = %q", state.Phase)
	}
	if state.FinalizeState == nil || state.FinalizeState.Status != releasedomain.StepFailed {
		t.Fatalf("finalize = %#v", state.FinalizeState)
	}
	if len(state.StepWrites) != 1 || state.StepWrites[0].Status != releasedomain.StepFailed {
		t.Fatalf("step writes = %#v", state.StepWrites)
	}
}

func TestNormalizeReleaseObservedStateCanarySucceeded(t *testing.T) {
	rollout := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Rollout",
		"metadata": map[string]any{
			"name":       "demo-api",
			"generation": int64(4),
		},
		"spec": map[string]any{
			"replicas": int64(2),
			"strategy": map[string]any{
				"canary": map[string]any{},
			},
		},
		"status": map[string]any{
			"phase":             "Healthy",
			"readyReplicas":     int64(2),
			"availableReplicas": int64(2),
		},
	}}

	state := NormalizeReleaseObservedStateFromRollout("demo-ns", "demo-api", "demo-api", rollout)
	if state.Phase != releasedomain.StepSucceeded {
		t.Fatalf("phase = %q", state.Phase)
	}
	if state.FinalizeState == nil || state.FinalizeState.Status != releasedomain.StepSucceeded {
		t.Fatalf("finalize = %#v", state.FinalizeState)
	}
	if got := state.StepWrites[len(state.StepWrites)-1]; got.StepCode != "canary_100" || got.Status != releasedomain.StepSucceeded {
		t.Fatalf("last step write = %#v", got)
	}
}

func TestNormalizeReleaseObservedStateBlueGreenSucceeded(t *testing.T) {
	rollout := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Rollout",
		"metadata": map[string]any{
			"name":       "demo-api",
			"generation": int64(4),
		},
		"spec": map[string]any{
			"replicas": int64(2),
			"strategy": map[string]any{
				"blueGreen": map[string]any{},
			},
		},
		"status": map[string]any{
			"phase":             "Healthy",
			"readyReplicas":     int64(2),
			"availableReplicas": int64(2),
		},
	}}

	state := NormalizeReleaseObservedStateFromRollout("demo-ns", "demo-api", "demo-api", rollout)
	if state.Phase != releasedomain.StepSucceeded {
		t.Fatalf("phase = %q", state.Phase)
	}
	if len(state.StepWrites) != 4 {
		t.Fatalf("step writes = %#v", state.StepWrites)
	}
	if state.StepWrites[0].StepCode != "deploy_preview" || state.StepWrites[3].StepCode != "verify_active" {
		t.Fatalf("step writes = %#v", state.StepWrites)
	}
}

func TestNormalizeReleaseObservedStateWorkloadBackedCanarySucceededUsesRolloutContract(t *testing.T) {
	workload := &runtimedomain.RuntimeObservedWorkload{
		Namespace:          "demo-ns",
		WorkloadKind:       "Rollout",
		WorkloadName:       "demo-api",
		DesiredReplicas:    2,
		ReadyReplicas:      2,
		AvailableReplicas:  2,
		SummaryStatus:      "Healthy",
		ObservedGeneration: 4,
	}

	state := NormalizeReleaseObservedState(workload)
	if state.Phase != releasedomain.StepSucceeded {
		t.Fatalf("phase = %q", state.Phase)
	}
	if state.FinalizeState == nil || state.FinalizeState.Status != releasedomain.StepSucceeded {
		t.Fatalf("finalize = %#v", state.FinalizeState)
	}
	if got := state.StepWrites[len(state.StepWrites)-1]; got.StepCode != "canary_100" || got.Status != releasedomain.StepSucceeded {
		t.Fatalf("last step write = %#v", got)
	}
}

func TestNormalizeReleaseObservedStateWorkloadBackedBlueGreenSucceededUsesRolloutContract(t *testing.T) {
	workload := &runtimedomain.RuntimeObservedWorkload{
		Namespace:          "demo-ns",
		WorkloadKind:       "Rollout",
		WorkloadName:       "demo-api",
		DesiredReplicas:    2,
		ReadyReplicas:      2,
		AvailableReplicas:  2,
		SummaryStatus:      "Healthy",
		ObservedGeneration: 4,
		Conditions: []runtimedomain.RuntimeObservedWorkloadCondition{
			{
				Type:   "BlueGreen",
				Status: "True",
			},
		},
	}

	state := NormalizeReleaseObservedState(workload)
	if state.Phase != releasedomain.StepSucceeded {
		t.Fatalf("phase = %q", state.Phase)
	}
	if len(state.StepWrites) != 4 {
		t.Fatalf("step writes = %#v", state.StepWrites)
	}
	if state.StepWrites[0].StepCode != "deploy_preview" || state.StepWrites[3].StepCode != "verify_active" {
		t.Fatalf("step writes = %#v", state.StepWrites)
	}
}

func TestNormalizeReleaseObservedStateWorkloadBackedDeploymentDoesNotTreatObservedGenerationAsFreshSpecGeneration(t *testing.T) {
	workload := &runtimedomain.RuntimeObservedWorkload{
		Namespace:           "demo-ns",
		WorkloadKind:        "Deployment",
		WorkloadName:        "demo-api",
		DesiredReplicas:     2,
		ReadyReplicas:       2,
		UpdatedReplicas:     2,
		AvailableReplicas:   2,
		UnavailableReplicas: 0,
		ObservedGeneration:  3,
	}

	state := NormalizeReleaseObservedState(workload)
	if state.Phase != releasedomain.StepRunning {
		t.Fatalf("phase = %q", state.Phase)
	}
	if state.FinalizeState != nil {
		t.Fatalf("finalize = %#v", state.FinalizeState)
	}
	if state.Progress >= 100 {
		t.Fatalf("progress = %d", state.Progress)
	}
}

func TestNormalizeReleaseObservedStateUsesDistinctStateKeysForRunningAndSucceededDeployment(t *testing.T) {
	running := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "demo-api", Generation: 3},
		Spec:       appsv1.DeploymentSpec{Replicas: int32Ptr(3)},
		Status: appsv1.DeploymentStatus{
			ObservedGeneration:  2,
			UpdatedReplicas:     2,
			ReadyReplicas:       1,
			AvailableReplicas:   1,
			UnavailableReplicas: 2,
			Conditions: []appsv1.DeploymentCondition{
				{Type: appsv1.DeploymentProgressing, Status: "True", Reason: "ReplicaSetUpdated"},
			},
		},
	}
	succeeded := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "demo-api", Generation: 3},
		Spec:       appsv1.DeploymentSpec{Replicas: int32Ptr(3)},
		Status: appsv1.DeploymentStatus{
			ObservedGeneration: 3,
			UpdatedReplicas:    3,
			ReadyReplicas:      3,
			AvailableReplicas:  3,
		},
	}

	runningState := NormalizeReleaseObservedStateFromDeployment("demo-ns", "demo-api", running)
	succeededState := NormalizeReleaseObservedStateFromDeployment("demo-ns", "demo-api", succeeded)

	if runningState.Phase != releasedomain.StepRunning {
		t.Fatalf("running phase = %q", runningState.Phase)
	}
	if succeededState.Phase != releasedomain.StepSucceeded {
		t.Fatalf("succeeded phase = %q", succeededState.Phase)
	}
	if runningState.StateKey == succeededState.StateKey {
		t.Fatalf("expected distinct state keys, got %q", runningState.StateKey)
	}
}

func int32Ptr(v int32) *int32 {
	return &v
}
