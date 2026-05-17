package reconcile

import (
	"context"
	"errors"
	"strings"
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
		item: &ReleaseRecord{
			ReleaseID:      releaseID,
			ApplicationID:  applicationID,
			EnvironmentID:  "env-1",
			ControlPlaneID: "cp-1",
			Status:         "running",
		},
	}, store, writer, updater, "cp-1")

	err := reconciler.Reconcile(context.Background(), releaseID.String())
	var requeueErr interface{ RequeueAfter() time.Duration }
	if !errors.As(err, &requeueErr) {
		t.Fatalf("Reconcile error = %v, want requeue-after error", err)
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

func TestReleaseReconcilerCompensatesNonRunningRelease(t *testing.T) {
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
		item: &ReleaseRecord{
			ReleaseID:      releaseID,
			ApplicationID:  applicationID,
			EnvironmentID:  "env-1",
			ControlPlaneID: "cp-1",
			Status:         "Succeeded",
		},
	}, store, writer, updater, "cp-1")

	err := reconciler.Reconcile(context.Background(), releaseID.String())
	var requeueErr interface{ RequeueAfter() time.Duration }
	if !errors.As(err, &requeueErr) {
		t.Fatalf("Reconcile error = %v, want requeue-after error", err)
	}
	assertReleaseWritePayload(t, writer.input, releaseWriteExpectation{
		releaseID:            releaseID,
		applicationID:        applicationID,
		environmentID:        "env-1",
		namespace:            "devflow",
		observedWorkloadKind: "Deployment",
		observedWorkloadName: "demo-api",
		phase:                releasedomain.StepRunning,
		progress:             25,
		stepWrites: []stepWriteExpectation{
			{
				stepCode: "observe_rollout",
				status:   releasedomain.StepRunning,
				progress: 25,
				message: messageExpectation{
					contains: "deployment progressing",
				},
			},
		},
	})
	if updater.called {
		t.Fatal("did not expect status updater for running workload state")
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
		ObservedGeneration:  1,
		DesiredReplicas:     3,
		UpdatedReplicas:     3,
		ReadyReplicas:       3,
		AvailableReplicas:   3,
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
		item: &ReleaseRecord{
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
	if writer.input == nil {
		t.Fatal("expected steps writer to be invoked")
	}
	if len(writer.input.StepWrites) != 2 {
		t.Fatalf("StepWrites len = %d, want 2", len(writer.input.StepWrites))
	}
	if writer.input.StepWrites[1].StepCode != "finalize_release" {
		t.Fatalf("final step = %q", writer.input.StepWrites[1].StepCode)
	}
}

func TestReleaseReconcilerFindsMatchingObservedWorkloadWhenEarlierSpecBelongsToAnotherRelease(t *testing.T) {
	releaseID := uuid.New()
	staleReleaseID := uuid.New()
	applicationID := uuid.New()
	store := runtimerepo.NewMemoryStore()

	firstSpec := &runtimedomain.RuntimeSpec{
		ID:            uuid.New(),
		ApplicationID: applicationID,
		Environment:   "env-1",
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	if err := store.CreateRuntimeSpec(context.Background(), firstSpec); err != nil {
		t.Fatalf("CreateRuntimeSpec(first) failed: %v", err)
	}
	if err := store.UpsertObservedWorkload(context.Background(), &runtimedomain.RuntimeObservedWorkload{
		ID:            uuid.New(),
		RuntimeSpecID: firstSpec.ID,
		ApplicationID: applicationID,
		Environment:   "env-1",
		Namespace:     "devflow",
		WorkloadKind:  "Deployment",
		WorkloadName:  "demo-api-old",
		Labels: map[string]string{
			releasedomain.ReleaseIDLabel:     staleReleaseID.String(),
			releasedomain.ControlPlaneLabel:  "cp-1",
			releasedomain.ReleaseStatusLabel: string(releasedomain.ReleaseRunning),
		},
		ObservedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("UpsertObservedWorkload(first) failed: %v", err)
	}

	secondSpec := &runtimedomain.RuntimeSpec{
		ID:            uuid.New(),
		ApplicationID: applicationID,
		Environment:   "env-1",
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	if err := store.CreateRuntimeSpec(context.Background(), secondSpec); err != nil {
		t.Fatalf("CreateRuntimeSpec(second) failed: %v", err)
	}
	if err := store.UpsertObservedWorkload(context.Background(), &runtimedomain.RuntimeObservedWorkload{
		ID:                  uuid.New(),
		RuntimeSpecID:       secondSpec.ID,
		ApplicationID:       applicationID,
		Environment:         "env-1",
		Namespace:           "devflow",
		WorkloadKind:        "Deployment",
		WorkloadName:        "demo-api",
		ObservedGeneration:  1,
		DesiredReplicas:     2,
		UpdatedReplicas:     2,
		ReadyReplicas:       2,
		AvailableReplicas:   2,
		UnavailableReplicas: 0,
		Labels: map[string]string{
			releasedomain.ReleaseIDLabel:     releaseID.String(),
			releasedomain.ControlPlaneLabel:  "cp-1",
			releasedomain.ReleaseStatusLabel: string(releasedomain.ReleaseRunning),
		},
		ObservedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("UpsertObservedWorkload(second) failed: %v", err)
	}

	writer := &stubReleaseStepsWriter{}
	updater := &stubReleaseStatusLabelUpdater{}
	reconciler := NewReleaseReconciler(stubReleaseStateSource{
		item: &ReleaseRecord{
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
	if writer.input.ObservedWorkloadName != "demo-api" {
		t.Fatalf("ObservedWorkloadName = %q, want %q", writer.input.ObservedWorkloadName, "demo-api")
	}
	if !updater.called {
		t.Fatal("expected terminal status updater to be called")
	}
	if updater.status != releasedomain.ReleaseSucceeded {
		t.Fatalf("status = %q, want %q", updater.status, releasedomain.ReleaseSucceeded)
	}
}

func TestReleaseReconcilerCompensatesTerminalSucceededDeployment(t *testing.T) {
	releaseID := uuid.New()
	applicationID := uuid.New()
	store := runtimerepo.NewMemoryStore()
	createRuntimeSpecAndObservedWorkload(t, store, applicationID, &runtimedomain.RuntimeObservedWorkload{
		ID:                  uuid.New(),
		ApplicationID:       applicationID,
		Environment:         "env-1",
		Namespace:           "devflow",
		WorkloadKind:        "Deployment",
		WorkloadName:        "demo-api",
		ObservedGeneration:  1,
		DesiredReplicas:     3,
		UpdatedReplicas:     3,
		ReadyReplicas:       3,
		AvailableReplicas:   3,
		UnavailableReplicas: 0,
		Labels: map[string]string{
			releasedomain.ReleaseIDLabel:     releaseID.String(),
			releasedomain.ControlPlaneLabel:  "cp-1",
			releasedomain.ReleaseStatusLabel: string(releasedomain.ReleaseRunning),
		},
		ObservedAt: time.Now().UTC(),
	})

	writer := &stubReleaseStepsWriter{}
	updater := &stubReleaseStatusLabelUpdater{}
	reconciler := NewReleaseReconciler(stubReleaseStateSource{
		item: &ReleaseRecord{
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
	assertReleaseWritePayload(t, writer.input, releaseWriteExpectation{
		releaseID:            releaseID,
		applicationID:        applicationID,
		environmentID:        "env-1",
		namespace:            "devflow",
		observedWorkloadKind: "Deployment",
		observedWorkloadName: "demo-api",
		phase:                releasedomain.StepSucceeded,
		progress:             100,
		stepWrites: []stepWriteExpectation{
			{
				stepCode: "observe_rollout",
				status:   releasedomain.StepSucceeded,
				progress: 100,
				message: messageExpectation{
					nonEmpty: true,
				},
			},
			{
				stepCode: "finalize_release",
				status:   releasedomain.StepSucceeded,
				progress: 100,
				message: messageExpectation{
					contains: "finalized",
				},
			},
		},
	})
	if !updater.called {
		t.Fatal("expected terminal status updater to be called")
	}
	if updater.status != releasedomain.ReleaseSucceeded {
		t.Fatalf("status = %q, want %q", updater.status, releasedomain.ReleaseSucceeded)
	}
}

func TestReleaseReconcilerCompensatesTerminalFailedDeployment(t *testing.T) {
	releaseID := uuid.New()
	applicationID := uuid.New()
	store := runtimerepo.NewMemoryStore()
	createRuntimeSpecAndObservedWorkload(t, store, applicationID, &runtimedomain.RuntimeObservedWorkload{
		ID:                  uuid.New(),
		ApplicationID:       applicationID,
		Environment:         "env-1",
		Namespace:           "devflow",
		WorkloadKind:        "Deployment",
		WorkloadName:        "demo-api",
		DesiredReplicas:     3,
		ReadyReplicas:       1,
		UnavailableReplicas: 2,
		SummaryStatus:       "Failed",
		Conditions: []runtimedomain.RuntimeObservedWorkloadCondition{
			{
				Type:    "Progressing",
				Status:  "False",
				Reason:  "ProgressDeadlineExceeded",
				Message: "deployment exceeded its progress deadline",
			},
		},
		Labels: map[string]string{
			releasedomain.ReleaseIDLabel:     releaseID.String(),
			releasedomain.ControlPlaneLabel:  "cp-1",
			releasedomain.ReleaseStatusLabel: string(releasedomain.ReleaseRunning),
		},
		ObservedAt: time.Now().UTC(),
	})

	writer := &stubReleaseStepsWriter{}
	updater := &stubReleaseStatusLabelUpdater{}
	reconciler := NewReleaseReconciler(stubReleaseStateSource{
		item: &ReleaseRecord{
			ReleaseID:      releaseID,
			ApplicationID:  applicationID,
			EnvironmentID:  "env-1",
			ControlPlaneID: "cp-1",
			Status:         "Failed",
		},
	}, store, writer, updater, "cp-1")

	if err := reconciler.Reconcile(context.Background(), releaseID.String()); err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}
	assertReleaseWritePayload(t, writer.input, releaseWriteExpectation{
		releaseID:            releaseID,
		applicationID:        applicationID,
		environmentID:        "env-1",
		namespace:            "devflow",
		observedWorkloadKind: "Deployment",
		observedWorkloadName: "demo-api",
		phase:                releasedomain.StepFailed,
		progress:             100,
		stepWrites: []stepWriteExpectation{
			{
				stepCode: "observe_rollout",
				status:   releasedomain.StepFailed,
				progress: 100,
				message: messageExpectation{
					nonEmpty: true,
				},
			},
			{
				stepCode: "finalize_release",
				status:   releasedomain.StepFailed,
				progress: 100,
				message: messageExpectation{
					contains: "finalized",
				},
			},
		},
	})
	if !updater.called {
		t.Fatal("expected terminal status updater to be called")
	}
	if updater.status != releasedomain.ReleaseFailed {
		t.Fatalf("status = %q, want %q", updater.status, releasedomain.ReleaseFailed)
	}
}

func TestReleaseReconcilerCompensatesTerminalReleaseStatusLabelConvergence(t *testing.T) {
	releaseID := uuid.New()
	applicationID := uuid.New()
	store := runtimerepo.NewMemoryStore()
	createRuntimeSpecAndObservedWorkload(t, store, applicationID, &runtimedomain.RuntimeObservedWorkload{
		ID:                  uuid.New(),
		ApplicationID:       applicationID,
		Environment:         "env-1",
		Namespace:           "devflow",
		WorkloadKind:        "Deployment",
		WorkloadName:        "demo-api",
		ObservedGeneration:  1,
		DesiredReplicas:     3,
		UpdatedReplicas:     3,
		ReadyReplicas:       3,
		AvailableReplicas:   3,
		UnavailableReplicas: 0,
		Labels: map[string]string{
			releasedomain.ReleaseIDLabel:     releaseID.String(),
			releasedomain.ControlPlaneLabel:  "cp-1",
			releasedomain.ReleaseStatusLabel: string(releasedomain.ReleaseRunning),
		},
		ObservedAt: time.Now().UTC(),
	})

	writer := &stubReleaseStepsWriter{}
	updater := &stubReleaseStatusLabelUpdater{}
	reconciler := NewReleaseReconciler(stubReleaseStateSource{
		item: &ReleaseRecord{
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
	if !updater.called {
		t.Fatal("expected release status label to converge for terminal release")
	}
	if updater.workload == nil {
		t.Fatal("expected status updater to receive workload")
	}
	if updater.workload.Labels[releasedomain.ReleaseStatusLabel] != string(releasedomain.ReleaseRunning) {
		t.Fatalf("status label before updater = %q, want %q", updater.workload.Labels[releasedomain.ReleaseStatusLabel], releasedomain.ReleaseRunning)
	}
	if updater.status != releasedomain.ReleaseSucceeded {
		t.Fatalf("status = %q, want %q", updater.status, releasedomain.ReleaseSucceeded)
	}
	if writer.input == nil {
		t.Fatal("expected step compensation to happen in the same reconcile pass")
	}
	if writer.input.Phase != releasedomain.StepSucceeded {
		t.Fatalf("phase = %q, want %q", writer.input.Phase, releasedomain.StepSucceeded)
	}
	if writer.input.ObservedWorkloadName != "demo-api" {
		t.Fatalf("ObservedWorkloadName = %q, want %q", writer.input.ObservedWorkloadName, "demo-api")
	}
}

func TestReleaseReconcilerReemitsTerminalCompensationWithoutLocalSuppression(t *testing.T) {
	releaseID := uuid.New()
	applicationID := uuid.New()
	store := runtimerepo.NewMemoryStore()
	createRuntimeSpecAndObservedWorkload(t, store, applicationID, &runtimedomain.RuntimeObservedWorkload{
		ID:                  uuid.New(),
		ApplicationID:       applicationID,
		Environment:         "env-1",
		Namespace:           "devflow",
		WorkloadKind:        "Deployment",
		WorkloadName:        "demo-api",
		ObservedGeneration:  1,
		DesiredReplicas:     3,
		UpdatedReplicas:     3,
		ReadyReplicas:       3,
		AvailableReplicas:   3,
		UnavailableReplicas: 0,
		Labels: map[string]string{
			releasedomain.ReleaseIDLabel:     releaseID.String(),
			releasedomain.ControlPlaneLabel:  "cp-1",
			releasedomain.ReleaseStatusLabel: string(releasedomain.ReleaseRunning),
		},
		ObservedAt: time.Now().UTC(),
	})

	writer := &recordingReleaseStepsWriter{}
	updater := &stubReleaseStatusLabelUpdater{}
	reconciler := NewReleaseReconciler(stubReleaseStateSource{
		item: &ReleaseRecord{
			ReleaseID:      releaseID,
			ApplicationID:  applicationID,
			EnvironmentID:  "env-1",
			ControlPlaneID: "cp-1",
			Status:         "Succeeded",
		},
	}, store, writer, updater, "cp-1")

	if err := reconciler.Reconcile(context.Background(), releaseID.String()); err != nil {
		t.Fatalf("first Reconcile failed: %v", err)
	}
	if err := reconciler.Reconcile(context.Background(), releaseID.String()); err != nil {
		t.Fatalf("second Reconcile failed: %v", err)
	}
	if len(writer.inputs) != 2 {
		t.Fatalf("write count = %d, want 2", len(writer.inputs))
	}
	assertReleaseWritePayload(t, writer.inputs[0], releaseWriteExpectation{
		releaseID:            releaseID,
		applicationID:        applicationID,
		environmentID:        "env-1",
		namespace:            "devflow",
		observedWorkloadKind: "Deployment",
		observedWorkloadName: "demo-api",
		phase:                releasedomain.StepSucceeded,
		progress:             100,
		stepWrites: []stepWriteExpectation{
			{
				stepCode: "observe_rollout",
				status:   releasedomain.StepSucceeded,
				progress: 100,
				message:  messageExpectation{nonEmpty: true},
			},
			{
				stepCode: "finalize_release",
				status:   releasedomain.StepSucceeded,
				progress: 100,
				message:  messageExpectation{contains: "finalized"},
			},
		},
	})
	assertReleaseWritePayload(t, writer.inputs[1], releaseWriteExpectation{
		releaseID:            releaseID,
		applicationID:        applicationID,
		environmentID:        "env-1",
		namespace:            "devflow",
		observedWorkloadKind: "Deployment",
		observedWorkloadName: "demo-api",
		phase:                releasedomain.StepSucceeded,
		progress:             100,
		stepWrites: []stepWriteExpectation{
			{
				stepCode: "observe_rollout",
				status:   releasedomain.StepSucceeded,
				progress: 100,
				message:  messageExpectation{nonEmpty: true},
			},
			{
				stepCode: "finalize_release",
				status:   releasedomain.StepSucceeded,
				progress: 100,
				message:  messageExpectation{contains: "finalized"},
			},
		},
	})
}

type stubReleaseStateSource struct {
	item *ReleaseRecord
}

func (s stubReleaseStateSource) GetRelease(context.Context, uuid.UUID) (*ReleaseRecord, error) {
	return s.item, nil
}

type stubReleaseStepsWriter struct {
	input *WriteReleaseStepsInput
}

func (s *stubReleaseStepsWriter) WriteReleaseSteps(_ context.Context, input WriteReleaseStepsInput) error {
	s.input = &input
	return nil
}

type recordingReleaseStepsWriter struct {
	inputs []*WriteReleaseStepsInput
}

func (s *recordingReleaseStepsWriter) WriteReleaseSteps(_ context.Context, input WriteReleaseStepsInput) error {
	copyInput := input
	s.inputs = append(s.inputs, &copyInput)
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

func createRuntimeSpecAndObservedWorkload(t *testing.T, store runtimerepo.Store, applicationID uuid.UUID, workload *runtimedomain.RuntimeObservedWorkload) {
	t.Helper()

	spec := &runtimedomain.RuntimeSpec{
		ID:            uuid.New(),
		ApplicationID: applicationID,
		Environment:   workload.Environment,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	if err := store.CreateRuntimeSpec(context.Background(), spec); err != nil {
		t.Fatalf("CreateRuntimeSpec failed: %v", err)
	}
	workload.RuntimeSpecID = spec.ID
	if err := store.UpsertObservedWorkload(context.Background(), workload); err != nil {
		t.Fatalf("UpsertObservedWorkload failed: %v", err)
	}
}

type releaseWriteExpectation struct {
	releaseID            uuid.UUID
	applicationID        uuid.UUID
	environmentID        string
	namespace            string
	observedWorkloadKind string
	observedWorkloadName string
	phase                releasedomain.StepStatus
	progress             int32
	stepWrites           []stepWriteExpectation
}

type stepWriteExpectation struct {
	stepCode string
	status   releasedomain.StepStatus
	progress int32
	message  messageExpectation
}

type messageExpectation struct {
	exact    string
	contains string
	nonEmpty bool
}

func assertReleaseWritePayload(t *testing.T, input *WriteReleaseStepsInput, want releaseWriteExpectation) {
	t.Helper()

	if input == nil {
		t.Fatal("expected steps writer to be invoked")
	}
	if input.ReleaseID != want.releaseID {
		t.Fatalf("ReleaseID = %s, want %s", input.ReleaseID, want.releaseID)
	}
	if input.ApplicationID != want.applicationID {
		t.Fatalf("ApplicationID = %s, want %s", input.ApplicationID, want.applicationID)
	}
	if input.EnvironmentID != want.environmentID {
		t.Fatalf("EnvironmentID = %q, want %q", input.EnvironmentID, want.environmentID)
	}
	if input.Namespace != want.namespace {
		t.Fatalf("Namespace = %q, want %q", input.Namespace, want.namespace)
	}
	if input.ObservedWorkloadKind != want.observedWorkloadKind {
		t.Fatalf("ObservedWorkloadKind = %q, want %q", input.ObservedWorkloadKind, want.observedWorkloadKind)
	}
	if input.ObservedWorkloadName != want.observedWorkloadName {
		t.Fatalf("ObservedWorkloadName = %q, want %q", input.ObservedWorkloadName, want.observedWorkloadName)
	}
	if input.Phase != want.phase {
		t.Fatalf("Phase = %q, want %q", input.Phase, want.phase)
	}
	if input.Progress != want.progress {
		t.Fatalf("Progress = %d, want %d", input.Progress, want.progress)
	}
	if len(input.StepWrites) != len(want.stepWrites) {
		t.Fatalf("StepWrites len = %d, want %d", len(input.StepWrites), len(want.stepWrites))
	}
	for i := range want.stepWrites {
		got := input.StepWrites[i]
		expected := want.stepWrites[i]
		if got.StepCode != expected.stepCode {
			t.Fatalf("StepWrites[%d].StepCode = %q, want %q", i, got.StepCode, expected.stepCode)
		}
		if got.Status != expected.status {
			t.Fatalf("StepWrites[%d].Status = %q, want %q", i, got.Status, expected.status)
		}
		if got.Progress != expected.progress {
			t.Fatalf("StepWrites[%d].Progress = %d, want %d", i, got.Progress, expected.progress)
		}
		switch {
		case expected.message.exact != "":
			if got.Message != expected.message.exact {
				t.Fatalf("StepWrites[%d].Message = %q, want %q", i, got.Message, expected.message.exact)
			}
		case expected.message.contains != "":
			if !strings.Contains(got.Message, expected.message.contains) {
				t.Fatalf("StepWrites[%d].Message = %q, want substring %q", i, got.Message, expected.message.contains)
			}
		case expected.message.nonEmpty:
			if strings.TrimSpace(got.Message) == "" {
				t.Fatalf("StepWrites[%d].Message = %q, want non-empty", i, got.Message)
			}
		}
	}
}
