package reconcile

import (
	"context"
	"testing"
	"time"

	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
	runtimedomain "github.com/bsonger/devflow-service/internal/runtime/domain"
	runtimerepo "github.com/bsonger/devflow-service/internal/runtime/repository"
	"github.com/google/uuid"
)

func TestReleaseReconcilerWritesStepsForMatchingRunningRelease(t *testing.T) {
	releaseID := uuid.New()
	applicationID := uuid.New()
	store := runtimerepo.NewMemoryStore()
	spec := &runtimedomain.RuntimeSpec{
		ID:            uuid.New(),
		ApplicationID: applicationID,
		Environment:   "env-1",
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	if err := store.CreateRuntimeSpec(context.Background(), spec); err != nil {
		t.Fatalf("CreateRuntimeSpec failed: %v", err)
	}
	if err := store.UpsertObservedWorkload(context.Background(), &runtimedomain.RuntimeObservedWorkload{
		ID:            uuid.New(),
		RuntimeSpecID: spec.ID,
		ApplicationID: applicationID,
		Environment:   "env-1",
		Namespace:     "devflow",
		WorkloadKind:  "Deployment",
		WorkloadName:  "demo-api",
		Labels: map[string]string{
			releasedomain.ReleaseIDLabel: releaseID.String(),
		},
		ObservedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("UpsertObservedWorkload failed: %v", err)
	}

	writer := &stubReleaseStepsWriter{}
	updater := &stubReleaseStatusLabelUpdater{}
	reconciler := NewReleaseReconciler(stubReleaseStateSource{
		item: &RunningRelease{
			ReleaseID:      releaseID,
			ApplicationID:  applicationID,
			EnvironmentID:  "env-1",
			ControlPlaneID: "cp-1",
			Status:         "running",
		},
	}, store, writer, updater, "cp-1")

	if err := reconciler.Reconcile(context.Background(), releaseID.String()); err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}
	if writer.input == nil {
		t.Fatal("expected steps writer to be invoked")
	}
	if writer.input.ReleaseID != releaseID {
		t.Fatalf("ReleaseID = %s", writer.input.ReleaseID)
	}
	if writer.input.Phase != releasedomain.StepRunning {
		t.Fatalf("Phase = %q", writer.input.Phase)
	}
	if len(writer.input.StepWrites) != 1 {
		t.Fatalf("StepWrites len = %d, want 1", len(writer.input.StepWrites))
	}
	if writer.input.StepWrites[0].StepCode != "observe_rollout" {
		t.Fatalf("StepCode = %q", writer.input.StepWrites[0].StepCode)
	}
	if updater.called {
		t.Fatal("did not expect terminal status updater for running workload")
	}
}

func TestReleaseReconcilerSkipsNonRunningRelease(t *testing.T) {
	releaseID := uuid.New()
	applicationID := uuid.New()
	store := runtimerepo.NewMemoryStore()
	spec := &runtimedomain.RuntimeSpec{
		ID:            uuid.New(),
		ApplicationID: applicationID,
		Environment:   "env-1",
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	if err := store.CreateRuntimeSpec(context.Background(), spec); err != nil {
		t.Fatalf("CreateRuntimeSpec failed: %v", err)
	}
	if err := store.UpsertObservedWorkload(context.Background(), &runtimedomain.RuntimeObservedWorkload{
		ID:            uuid.New(),
		RuntimeSpecID: spec.ID,
		ApplicationID: applicationID,
		Environment:   "env-1",
		Namespace:     "devflow",
		WorkloadKind:  "Deployment",
		WorkloadName:  "demo-api",
		Labels: map[string]string{
			releasedomain.ReleaseIDLabel: releaseID.String(),
		},
		ObservedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("UpsertObservedWorkload failed: %v", err)
	}

	writer := &stubReleaseStepsWriter{}
	updater := &stubReleaseStatusLabelUpdater{}
	reconciler := NewReleaseReconciler(stubReleaseStateSource{
		item: &RunningRelease{
			ReleaseID:      releaseID,
			ApplicationID:  applicationID,
			EnvironmentID:  "env-1",
			ControlPlaneID: "cp-1",
			Status:         "Succeeded",
		},
	}, store, writer, updater, "cp-1")

	if err := reconciler.Reconcile(context.Background(), releaseID.String()); err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}
	if writer.input != nil {
		t.Fatalf("writer input = %#v, want nil", writer.input)
	}
	if updater.called {
		t.Fatal("did not expect status updater for non-running release")
	}
}

func TestReleaseReconcilerConvergesTerminalReleaseStatusLabel(t *testing.T) {
	releaseID := uuid.New()
	applicationID := uuid.New()
	store := runtimerepo.NewMemoryStore()
	spec := &runtimedomain.RuntimeSpec{
		ID:            uuid.New(),
		ApplicationID: applicationID,
		Environment:   "env-1",
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	if err := store.CreateRuntimeSpec(context.Background(), spec); err != nil {
		t.Fatalf("CreateRuntimeSpec failed: %v", err)
	}
	if err := store.UpsertObservedWorkload(context.Background(), &runtimedomain.RuntimeObservedWorkload{
		ID:                  uuid.New(),
		RuntimeSpecID:       spec.ID,
		ApplicationID:       applicationID,
		Environment:         "env-1",
		Namespace:           "devflow",
		WorkloadKind:        "Deployment",
		WorkloadName:        "demo-api",
		DesiredReplicas:     3,
		ReadyReplicas:       3,
		UnavailableReplicas: 0,
		Labels: map[string]string{
			releasedomain.ReleaseIDLabel:     releaseID.String(),
			releasedomain.ControlPlaneLabel:  "cp-1",
			releasedomain.ReleaseStatusLabel: string(releasedomain.ReleaseRunning),
		},
		ObservedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("UpsertObservedWorkload failed: %v", err)
	}

	writer := &stubReleaseStepsWriter{}
	updater := &stubReleaseStatusLabelUpdater{}
	reconciler := NewReleaseReconciler(stubReleaseStateSource{
		item: &RunningRelease{
			ReleaseID:      releaseID,
			ApplicationID:  applicationID,
			EnvironmentID:  "env-1",
			ControlPlaneID: "cp-1",
			Status:         "running",
		},
	}, store, writer, updater, "cp-1")

	if err := reconciler.Reconcile(context.Background(), releaseID.String()); err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}
	if !updater.called {
		t.Fatal("expected terminal status updater to be called")
	}
	if updater.status != releasedomain.ReleaseSucceeded {
		t.Fatalf("status = %q", updater.status)
	}
	if updater.workload == nil || updater.workload.WorkloadName != "demo-api" {
		t.Fatalf("workload = %#v", updater.workload)
	}
}

type stubReleaseStateSource struct {
	item *RunningRelease
}

func (s stubReleaseStateSource) GetRunningRelease(context.Context, uuid.UUID) (*RunningRelease, error) {
	return s.item, nil
}

type stubReleaseStepsWriter struct {
	input *WriteReleaseStepsInput
}

func (s *stubReleaseStepsWriter) WriteReleaseSteps(_ context.Context, input WriteReleaseStepsInput) error {
	s.input = &input
	return nil
}

type stubReleaseStatusLabelUpdater struct {
	called   bool
	workload *runtimedomain.RuntimeObservedWorkload
	status   releasedomain.ReleaseStatus
}

func (s *stubReleaseStatusLabelUpdater) UpdateReleaseStatusLabel(_ context.Context, workload *runtimedomain.RuntimeObservedWorkload, status releasedomain.ReleaseStatus) error {
	s.called = true
	s.workload = workload
	s.status = status
	return nil
}
