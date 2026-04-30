package observer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	if workload.Labels[releasedomain.ReleaseEnvironmentLabel] != ctx.EnvironmentID {
		t.Fatalf("environment label must remain primary identity, labels=%#v ctx=%+v", workload.Labels, ctx)
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

func TestWriteReleaseStepsRollingObserverSkipsReleaseOwnedHandoffStep(t *testing.T) {
	releaseID := uuid.New()
	tests := []struct {
		name              string
		phase             releasedomain.StepStatus
		progress          int32
		message           string
		wantSteps         []string
		wantStatuses      []releasedomain.StepStatus
		wantProgresses    []int32
		wantMessages      []string
		forbiddenStepCode string
	}{
		{
			name:              "running writes observe only",
			phase:             releasedomain.StepRunning,
			progress:          45,
			message:           "deployment progressing",
			wantSteps:         []string{"observe_rollout"},
			wantStatuses:      []releasedomain.StepStatus{releasedomain.StepRunning},
			wantProgresses:    []int32{45},
			wantMessages:      []string{"deployment progressing"},
			forbiddenStepCode: "start_deployment",
		},
		{
			name:              "succeeded writes observe and finalize only",
			phase:             releasedomain.StepSucceeded,
			progress:          100,
			message:           "deployment healthy",
			wantSteps:         []string{"observe_rollout", "finalize_release"},
			wantStatuses:      []releasedomain.StepStatus{releasedomain.StepSucceeded, releasedomain.StepSucceeded},
			wantProgresses:    []int32{100, 100},
			wantMessages:      []string{"deployment healthy", "release finalized after deployment became healthy"},
			forbiddenStepCode: "start_deployment",
		},
		{
			name:              "failed writes observe and finalize only",
			phase:             releasedomain.StepFailed,
			progress:          100,
			message:           "deployment failed",
			wantSteps:         []string{"observe_rollout", "finalize_release"},
			wantStatuses:      []releasedomain.StepStatus{releasedomain.StepFailed, releasedomain.StepFailed},
			wantProgresses:    []int32{100, 100},
			wantMessages:      []string{"deployment failed", "release finalized after deployment failure"},
			forbiddenStepCode: "start_deployment",
		},
	}

	type stepPayload struct {
		ReleaseID string `json:"release_id"`
		StepCode  string `json:"step_code"`
		Status    string `json:"status"`
		Progress  int32  `json:"progress"`
		Message   string `json:"message"`
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []stepPayload
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer r.Body.Close()
				if r.Method != http.MethodPost {
					t.Fatalf("method = %s", r.Method)
				}
				if r.URL.Path != "/api/v1/verify/release/steps" {
					t.Fatalf("path = %s", r.URL.Path)
				}
				var payload stepPayload
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Fatalf("decode payload: %v", err)
				}
				got = append(got, payload)
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()

			observer := &ReleaseRolloutObserver{
				httpClient:  server.Client(),
				releaseBase: server.URL,
			}
			rollout := releaseRolloutContext{ReleaseID: releaseID}
			if err := observer.writeReleaseSteps(context.Background(), rollout, tt.phase, tt.progress, tt.message); err != nil {
				t.Fatalf("writeReleaseSteps failed: %v", err)
			}
			if len(got) != len(tt.wantSteps) {
				t.Fatalf("posted steps = %d want %d (%#v)", len(got), len(tt.wantSteps), got)
			}
			for i := range tt.wantSteps {
				if got[i].ReleaseID != releaseID.String() {
					t.Fatalf("payload[%d].release_id = %q", i, got[i].ReleaseID)
				}
				if got[i].StepCode != tt.wantSteps[i] {
					t.Fatalf("payload[%d].step_code = %q want %q", i, got[i].StepCode, tt.wantSteps[i])
				}
				if got[i].Status != string(tt.wantStatuses[i]) {
					t.Fatalf("payload[%d].status = %q want %q", i, got[i].Status, tt.wantStatuses[i])
				}
				if got[i].Progress != tt.wantProgresses[i] {
					t.Fatalf("payload[%d].progress = %d want %d", i, got[i].Progress, tt.wantProgresses[i])
				}
				if got[i].Message != tt.wantMessages[i] {
					t.Fatalf("payload[%d].message = %q want %q", i, got[i].Message, tt.wantMessages[i])
				}
				if got[i].StepCode == tt.forbiddenStepCode {
					t.Fatalf("payload[%d] unexpectedly wrote forbidden step %q", i, tt.forbiddenStepCode)
				}
			}
		})
	}
}

func int32Ptr(v int32) *int32 {
	return &v
}
