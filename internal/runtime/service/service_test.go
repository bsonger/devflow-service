package service

import (
	"context"
	"database/sql"
	"testing"
	"time"

	runtimedomain "github.com/bsonger/devflow-service/internal/runtime/domain"
	sharederrs "github.com/bsonger/devflow-service/internal/shared/errs"
	"github.com/google/uuid"
)

type stubStore struct {
	createRuntimeSpecFunc                 func(context.Context, *runtimedomain.RuntimeSpec) error
	getRuntimeSpecFunc                    func(context.Context, uuid.UUID) (*runtimedomain.RuntimeSpec, error)
	getRuntimeSpecByApplicationEnvFunc    func(context.Context, uuid.UUID, string) (*runtimedomain.RuntimeSpec, error)
	ensureRuntimeSpecByApplicationEnvFunc func(context.Context, uuid.UUID, string) (*runtimedomain.RuntimeSpec, error)
	findRuntimeSpecByApplicationEnvFunc   func(context.Context, uuid.UUID, string) (*runtimedomain.RuntimeSpec, error)
	getApplicationNameFunc                func(context.Context, uuid.UUID) (string, error)
	resolveTargetNamespaceFunc            func(context.Context, uuid.UUID, string) (string, error)
	deleteRuntimeSpecByApplicationEnvFunc func(context.Context, uuid.UUID, string) error
	listRuntimeSpecsFunc                  func(context.Context) ([]*runtimedomain.RuntimeSpec, error)
	nextRevisionNumberFunc                func(context.Context, uuid.UUID) (int, error)
	createRuntimeSpecRevisionFunc         func(context.Context, *runtimedomain.RuntimeSpecRevision) error
	updateCurrentRevisionFunc             func(context.Context, uuid.UUID, uuid.UUID) error
	listRuntimeSpecRevisionsFunc          func(context.Context, uuid.UUID) ([]*runtimedomain.RuntimeSpecRevision, error)
	getRuntimeSpecRevisionFunc            func(context.Context, uuid.UUID) (*runtimedomain.RuntimeSpecRevision, error)
	upsertObservedWorkloadFunc            func(context.Context, *runtimedomain.RuntimeObservedWorkload) error
	deleteObservedWorkloadFunc            func(context.Context, uuid.UUID, string, string, string, time.Time) error
	getObservedWorkloadFunc               func(context.Context, uuid.UUID) (*runtimedomain.RuntimeObservedWorkload, error)
	upsertObservedPodFunc                 func(context.Context, *runtimedomain.RuntimeObservedPod) error
	deleteObservedPodFunc                 func(context.Context, uuid.UUID, string, string, time.Time) error
	listObservedPodsFunc                  func(context.Context, uuid.UUID) ([]*runtimedomain.RuntimeObservedPod, error)
	createRuntimeOperationFunc            func(context.Context, *runtimedomain.RuntimeOperation) error
	listRuntimeOperationsFunc             func(context.Context, uuid.UUID) ([]*runtimedomain.RuntimeOperation, error)
}

type stubK8sExecutor struct {
	deletePodFunc         func(context.Context, string, string) error
	restartDeploymentFunc func(context.Context, string, string) error
}

func (s stubK8sExecutor) DeletePod(ctx context.Context, namespace, name string) error {
	if s.deletePodFunc != nil {
		return s.deletePodFunc(ctx, namespace, name)
	}
	return nil
}

func (s stubK8sExecutor) RestartDeployment(ctx context.Context, namespace, name string) error {
	if s.restartDeploymentFunc != nil {
		return s.restartDeploymentFunc(ctx, namespace, name)
	}
	return nil
}

func (s stubStore) CreateRuntimeSpec(ctx context.Context, item *runtimedomain.RuntimeSpec) error {
	if s.createRuntimeSpecFunc != nil {
		return s.createRuntimeSpecFunc(ctx, item)
	}
	return nil
}

func (s stubStore) GetRuntimeSpec(ctx context.Context, id uuid.UUID) (*runtimedomain.RuntimeSpec, error) {
	if s.getRuntimeSpecFunc != nil {
		return s.getRuntimeSpecFunc(ctx, id)
	}
	return nil, sql.ErrNoRows
}

func (s stubStore) GetRuntimeSpecByApplicationEnv(ctx context.Context, applicationId uuid.UUID, environment string) (*runtimedomain.RuntimeSpec, error) {
	if s.getRuntimeSpecByApplicationEnvFunc != nil {
		return s.getRuntimeSpecByApplicationEnvFunc(ctx, applicationId, environment)
	}
	return nil, nil
}

func (s stubStore) EnsureRuntimeSpecByApplicationEnv(ctx context.Context, applicationId uuid.UUID, environment string) (*runtimedomain.RuntimeSpec, error) {
	if s.ensureRuntimeSpecByApplicationEnvFunc != nil {
		return s.ensureRuntimeSpecByApplicationEnvFunc(ctx, applicationId, environment)
	}
	return s.GetRuntimeSpecByApplicationEnv(ctx, applicationId, environment)
}

func (s stubStore) FindRuntimeSpecByApplicationEnv(ctx context.Context, applicationId uuid.UUID, environment string) (*runtimedomain.RuntimeSpec, error) {
	if s.findRuntimeSpecByApplicationEnvFunc != nil {
		return s.findRuntimeSpecByApplicationEnvFunc(ctx, applicationId, environment)
	}
	if spec, err := s.GetRuntimeSpecByApplicationEnv(ctx, applicationId, environment); err != nil {
		return nil, err
	} else if spec != nil {
		return spec, nil
	}
	return nil, sql.ErrNoRows
}

func (s stubStore) GetApplicationName(ctx context.Context, applicationId uuid.UUID) (string, error) {
	if s.getApplicationNameFunc != nil {
		return s.getApplicationNameFunc(ctx, applicationId)
	}
	return "", sql.ErrNoRows
}

func (s stubStore) ResolveTargetNamespace(ctx context.Context, applicationId uuid.UUID, environment string) (string, error) {
	if s.resolveTargetNamespaceFunc != nil {
		return s.resolveTargetNamespaceFunc(ctx, applicationId, environment)
	}
	return "", sql.ErrNoRows
}

func (s stubStore) DeleteRuntimeSpecByApplicationEnv(ctx context.Context, applicationId uuid.UUID, environment string) error {
	if s.deleteRuntimeSpecByApplicationEnvFunc != nil {
		return s.deleteRuntimeSpecByApplicationEnvFunc(ctx, applicationId, environment)
	}
	return nil
}

func (s stubStore) ListRuntimeSpecs(ctx context.Context) ([]*runtimedomain.RuntimeSpec, error) {
	if s.listRuntimeSpecsFunc != nil {
		return s.listRuntimeSpecsFunc(ctx)
	}
	return nil, nil
}

func (s stubStore) NextRevisionNumber(ctx context.Context, runtimeSpecID uuid.UUID) (int, error) {
	if s.nextRevisionNumberFunc != nil {
		return s.nextRevisionNumberFunc(ctx, runtimeSpecID)
	}
	return 1, nil
}

func (s stubStore) CreateRuntimeSpecRevision(ctx context.Context, item *runtimedomain.RuntimeSpecRevision) error {
	if s.createRuntimeSpecRevisionFunc != nil {
		return s.createRuntimeSpecRevisionFunc(ctx, item)
	}
	return nil
}

func (s stubStore) UpdateCurrentRevision(ctx context.Context, runtimeSpecID uuid.UUID, revisionID uuid.UUID) error {
	if s.updateCurrentRevisionFunc != nil {
		return s.updateCurrentRevisionFunc(ctx, runtimeSpecID, revisionID)
	}
	return nil
}

func (s stubStore) ListRuntimeSpecRevisions(ctx context.Context, runtimeSpecID uuid.UUID) ([]*runtimedomain.RuntimeSpecRevision, error) {
	if s.listRuntimeSpecRevisionsFunc != nil {
		return s.listRuntimeSpecRevisionsFunc(ctx, runtimeSpecID)
	}
	return nil, nil
}

func (s stubStore) GetRuntimeSpecRevision(ctx context.Context, id uuid.UUID) (*runtimedomain.RuntimeSpecRevision, error) {
	if s.getRuntimeSpecRevisionFunc != nil {
		return s.getRuntimeSpecRevisionFunc(ctx, id)
	}
	return nil, sql.ErrNoRows
}

func (s stubStore) UpsertObservedWorkload(ctx context.Context, item *runtimedomain.RuntimeObservedWorkload) error {
	if s.upsertObservedWorkloadFunc != nil {
		return s.upsertObservedWorkloadFunc(ctx, item)
	}
	return nil
}

func (s stubStore) DeleteObservedWorkload(ctx context.Context, runtimeSpecID uuid.UUID, namespace, workloadKind, workloadName string, observedAt time.Time) error {
	if s.deleteObservedWorkloadFunc != nil {
		return s.deleteObservedWorkloadFunc(ctx, runtimeSpecID, namespace, workloadKind, workloadName, observedAt)
	}
	return nil
}

func (s stubStore) GetObservedWorkload(ctx context.Context, runtimeSpecID uuid.UUID) (*runtimedomain.RuntimeObservedWorkload, error) {
	if s.getObservedWorkloadFunc != nil {
		return s.getObservedWorkloadFunc(ctx, runtimeSpecID)
	}
	return nil, sql.ErrNoRows
}

func (s stubStore) UpsertObservedPod(ctx context.Context, item *runtimedomain.RuntimeObservedPod) error {
	if s.upsertObservedPodFunc != nil {
		return s.upsertObservedPodFunc(ctx, item)
	}
	return nil
}

func (s stubStore) DeleteObservedPod(ctx context.Context, runtimeSpecID uuid.UUID, namespace, podName string, observedAt time.Time) error {
	if s.deleteObservedPodFunc != nil {
		return s.deleteObservedPodFunc(ctx, runtimeSpecID, namespace, podName, observedAt)
	}
	return nil
}

func (s stubStore) ListObservedPods(ctx context.Context, runtimeSpecID uuid.UUID) ([]*runtimedomain.RuntimeObservedPod, error) {
	if s.listObservedPodsFunc != nil {
		return s.listObservedPodsFunc(ctx, runtimeSpecID)
	}
	return nil, nil
}

func (s stubStore) CreateRuntimeOperation(ctx context.Context, item *runtimedomain.RuntimeOperation) error {
	if s.createRuntimeOperationFunc != nil {
		return s.createRuntimeOperationFunc(ctx, item)
	}
	return nil
}

func (s stubStore) ListRuntimeOperations(ctx context.Context, runtimeSpecID uuid.UUID) ([]*runtimedomain.RuntimeOperation, error) {
	if s.listRuntimeOperationsFunc != nil {
		return s.listRuntimeOperationsFunc(ctx, runtimeSpecID)
	}
	return nil, nil
}

func TestCreateRuntimeSpecRejectsDuplicate(t *testing.T) {
	applicationId := uuid.New()
	svc := New(stubStore{
		getRuntimeSpecByApplicationEnvFunc: func(context.Context, uuid.UUID, string) (*runtimedomain.RuntimeSpec, error) {
			return &runtimedomain.RuntimeSpec{ID: uuid.New(), ApplicationID: applicationId, Environment: "staging"}, nil
		},
		resolveTargetNamespaceFunc: func(context.Context, uuid.UUID, string) (string, error) {
			return "expected-namespace", nil
		},
	}, nil)

	_, err := svc.CreateRuntimeSpec(context.Background(), CreateRuntimeSpecInput{
		ApplicationID: applicationId,
		Environment:   "staging",
	})
	if !sharederrs.HasCode(err, sharederrs.CodeConflict) {
		t.Fatalf("expected conflict error, got %v", err)
	}
}

func TestCreateRuntimeSpecRevisionUsesNextRevisionAndUpdatesCurrent(t *testing.T) {
	applicationId := uuid.New()
	runtimeSpecID := uuid.New()
	capturedRevision := 0
	updatedRuntimeSpecID := uuid.Nil
	updatedRevisionID := uuid.Nil
	svc := New(stubStore{
		getRuntimeSpecFunc: func(context.Context, uuid.UUID) (*runtimedomain.RuntimeSpec, error) {
			return &runtimedomain.RuntimeSpec{ID: runtimeSpecID, ApplicationID: applicationId, Environment: "staging"}, nil
		},
		nextRevisionNumberFunc: func(context.Context, uuid.UUID) (int, error) {
			return 3, nil
		},
		createRuntimeSpecRevisionFunc: func(_ context.Context, item *runtimedomain.RuntimeSpecRevision) error {
			capturedRevision = item.Revision
			return nil
		},
		updateCurrentRevisionFunc: func(_ context.Context, runtimeSpecIDArg uuid.UUID, revisionID uuid.UUID) error {
			updatedRuntimeSpecID = runtimeSpecIDArg
			updatedRevisionID = revisionID
			return nil
		},
	}, nil)

	item, err := svc.CreateRuntimeSpecRevision(context.Background(), runtimeSpecID, CreateRuntimeSpecRevisionInput{CreatedBy: "tester"})
	if err != nil {
		t.Fatalf("CreateRuntimeSpecRevision() error = %v", err)
	}
	if capturedRevision != 3 {
		t.Fatalf("revision = %d, want 3", capturedRevision)
	}
	if item.Revision != 3 {
		t.Fatalf("item.Revision = %d, want 3", item.Revision)
	}
	if updatedRuntimeSpecID != runtimeSpecID {
		t.Fatalf("updated runtime spec id = %s, want %s", updatedRuntimeSpecID, runtimeSpecID)
	}
	if updatedRevisionID != item.ID {
		t.Fatalf("updated revision id = %s, want %s", updatedRevisionID, item.ID)
	}
}

func TestSyncObservedPodRejectsNamespaceMismatch(t *testing.T) {
	applicationId := uuid.New()
	svc := New(stubStore{
		getRuntimeSpecByApplicationEnvFunc: func(context.Context, uuid.UUID, string) (*runtimedomain.RuntimeSpec, error) {
			return &runtimedomain.RuntimeSpec{ID: uuid.New(), ApplicationID: applicationId, Environment: "staging"}, nil
		},
		resolveTargetNamespaceFunc: func(context.Context, uuid.UUID, string) (string, error) {
			return "expected-namespace", nil
		},
	}, nil)

	_, err := svc.SyncObservedPod(context.Background(), SyncObservedPodInput{
		ApplicationID: applicationId,
		Environment:   "staging",
		Namespace:     "wrong-namespace",
		PodName:       "demo-0",
		Phase:         "Running",
	})
	if !sharederrs.HasCode(err, sharederrs.CodeInvalidArgument) {
		t.Fatalf("expected invalid_argument error, got %v", err)
	}
}

func TestSyncObservedWorkloadStoresObservedSummary(t *testing.T) {
	applicationID := uuid.New()
	runtimeSpecID := uuid.New()
	var captured *runtimedomain.RuntimeObservedWorkload
	svc := New(stubStore{
		ensureRuntimeSpecByApplicationEnvFunc: func(context.Context, uuid.UUID, string) (*runtimedomain.RuntimeSpec, error) {
			return &runtimedomain.RuntimeSpec{ID: runtimeSpecID, ApplicationID: applicationID, Environment: "production"}, nil
		},
		resolveTargetNamespaceFunc: func(context.Context, uuid.UUID, string) (string, error) {
			return "devflow", nil
		},
		upsertObservedWorkloadFunc: func(_ context.Context, item *runtimedomain.RuntimeObservedWorkload) error {
			captured = item
			return nil
		},
	}, nil)

	item, err := svc.SyncObservedWorkload(context.Background(), SyncObservedWorkloadInput{
		ApplicationID:   applicationID,
		Environment:     "production",
		WorkloadKind:    "Deployment",
		WorkloadName:    "meta-service",
		DesiredReplicas: 2,
		ReadyReplicas:   2,
		SummaryStatus:   "Healthy",
		Images:          []string{"repo/meta-service:latest"},
		Conditions:      []ObservedWorkloadConditionInput{{Type: "Available", Status: "True"}},
	})
	if err != nil {
		t.Fatalf("SyncObservedWorkload() error = %v", err)
	}
	if item.RuntimeSpecID != runtimeSpecID {
		t.Fatalf("runtimeSpecID = %s, want %s", item.RuntimeSpecID, runtimeSpecID)
	}
	if captured == nil || captured.WorkloadName != "meta-service" {
		t.Fatalf("captured workload = %#v", captured)
	}
}

func TestGetObservedWorkloadByApplicationEnvFailsWhenObserverIdentityMissing(t *testing.T) {
	applicationID := uuid.New()
	svc := New(stubStore{
		findRuntimeSpecByApplicationEnvFunc: func(context.Context, uuid.UUID, string) (*runtimedomain.RuntimeSpec, error) {
			return nil, sql.ErrNoRows
		},
	}, nil)

	_, err := svc.GetObservedWorkloadByApplicationEnv(context.Background(), applicationID, "staging")
	if !sharederrs.HasCode(err, sharederrs.CodeNotFound) {
		t.Fatalf("expected not_found error, got %v", err)
	}
	if err == nil || err.Error() != ErrRuntimeIdentityMissing.Error() {
		t.Fatalf("error = %v, want %v", err, ErrRuntimeIdentityMissing)
	}
}

func TestListObservedPodsByApplicationEnvFailsWhenObserverIdentityMissing(t *testing.T) {
	applicationID := uuid.New()
	svc := New(stubStore{
		findRuntimeSpecByApplicationEnvFunc: func(context.Context, uuid.UUID, string) (*runtimedomain.RuntimeSpec, error) {
			return nil, sql.ErrNoRows
		},
	}, nil)

	_, err := svc.ListObservedPodsByApplicationEnv(context.Background(), applicationID, "staging")
	if !sharederrs.HasCode(err, sharederrs.CodeNotFound) {
		t.Fatalf("expected not_found error, got %v", err)
	}
	if err == nil || err.Error() != ErrRuntimeIdentityMissing.Error() {
		t.Fatalf("error = %v, want %v", err, ErrRuntimeIdentityMissing)
	}
}

func TestDeletePodByApplicationEnvFailsWhenObserverIdentityMissing(t *testing.T) {
	applicationID := uuid.New()
	svc := New(stubStore{
		findRuntimeSpecByApplicationEnvFunc: func(context.Context, uuid.UUID, string) (*runtimedomain.RuntimeSpec, error) {
			return nil, sql.ErrNoRows
		},
	}, nil)

	_, err := svc.DeletePodByApplicationEnv(context.Background(), applicationID, "staging", "demo-0", "tester")
	if !sharederrs.HasCode(err, sharederrs.CodeNotFound) {
		t.Fatalf("expected not_found error, got %v", err)
	}
	if err == nil || err.Error() != ErrRuntimeIdentityMissing.Error() {
		t.Fatalf("error = %v, want %v", err, ErrRuntimeIdentityMissing)
	}
}

func TestRestartDeploymentReturnsAmbiguousWhenObservedWorkloadMissing(t *testing.T) {
	applicationID := uuid.New()
	runtimeSpecID := uuid.New()
	restarted := false
	svc := New(stubStore{
		getRuntimeSpecFunc: func(context.Context, uuid.UUID) (*runtimedomain.RuntimeSpec, error) {
			return &runtimedomain.RuntimeSpec{ID: runtimeSpecID, ApplicationID: applicationID, Environment: "production"}, nil
		},
		getObservedWorkloadFunc: func(context.Context, uuid.UUID) (*runtimedomain.RuntimeObservedWorkload, error) {
			return nil, sql.ErrNoRows
		},
	}, stubK8sExecutor{
		restartDeploymentFunc: func(context.Context, string, string) error {
			restarted = true
			return nil
		},
	})

	_, err := svc.RestartDeployment(context.Background(), runtimeSpecID, "", "tester")
	if !sharederrs.HasCode(err, sharederrs.CodeFailedPrecondition) {
		t.Fatalf("expected failed_precondition error, got %v", err)
	}
	if err == nil || err.Error() != ErrRuntimeWorkloadAmbiguous.Error() {
		t.Fatalf("error = %v, want %v", err, ErrRuntimeWorkloadAmbiguous)
	}
	if restarted {
		t.Fatalf("expected restart executor not to be called")
	}
}

func TestRestartDeploymentReturnsAmbiguousWhenObservedWorkloadIsNotDeployment(t *testing.T) {
	applicationID := uuid.New()
	runtimeSpecID := uuid.New()
	restarted := false
	svc := New(stubStore{
		getRuntimeSpecFunc: func(context.Context, uuid.UUID) (*runtimedomain.RuntimeSpec, error) {
			return &runtimedomain.RuntimeSpec{ID: runtimeSpecID, ApplicationID: applicationID, Environment: "production"}, nil
		},
		getObservedWorkloadFunc: func(context.Context, uuid.UUID) (*runtimedomain.RuntimeObservedWorkload, error) {
			return &runtimedomain.RuntimeObservedWorkload{RuntimeSpecID: runtimeSpecID, WorkloadKind: "StatefulSet", WorkloadName: "meta-service"}, nil
		},
	}, stubK8sExecutor{
		restartDeploymentFunc: func(context.Context, string, string) error {
			restarted = true
			return nil
		},
	})

	_, err := svc.RestartDeployment(context.Background(), runtimeSpecID, "", "tester")
	if !sharederrs.HasCode(err, sharederrs.CodeFailedPrecondition) {
		t.Fatalf("expected failed_precondition error, got %v", err)
	}
	if err == nil || err.Error() != ErrRuntimeWorkloadAmbiguous.Error() {
		t.Fatalf("error = %v, want %v", err, ErrRuntimeWorkloadAmbiguous)
	}
	if restarted {
		t.Fatalf("expected restart executor not to be called")
	}
}

func TestDeletePodReturnsNamespaceUnresolvedBeforeKubernetesMutation(t *testing.T) {
	applicationID := uuid.New()
	runtimeSpecID := uuid.New()
	deleted := false
	svc := New(stubStore{
		getRuntimeSpecFunc: func(context.Context, uuid.UUID) (*runtimedomain.RuntimeSpec, error) {
			return &runtimedomain.RuntimeSpec{ID: runtimeSpecID, ApplicationID: applicationID, Environment: "staging"}, nil
		},
		resolveTargetNamespaceFunc: func(context.Context, uuid.UUID, string) (string, error) {
			return "", sql.ErrNoRows
		},
		findRuntimeSpecByApplicationEnvFunc: func(context.Context, uuid.UUID, string) (*runtimedomain.RuntimeSpec, error) {
			return &runtimedomain.RuntimeSpec{ID: runtimeSpecID, ApplicationID: applicationID, Environment: "staging"}, nil
		},
	}, stubK8sExecutor{
		deletePodFunc: func(context.Context, string, string) error {
			deleted = true
			return nil
		},
	})

	_, err := svc.DeletePod(context.Background(), runtimeSpecID, "demo-0", "tester")
	if !sharederrs.HasCode(err, sharederrs.CodeFailedPrecondition) {
		t.Fatalf("expected failed_precondition error, got %v", err)
	}
	if err == nil || err.Error() != ErrRuntimeNamespaceUnresolved.Error() {
		t.Fatalf("error = %v, want %v", err, ErrRuntimeNamespaceUnresolved)
	}
	if deleted {
		t.Fatalf("expected delete executor not to be called")
	}
}

func TestDeletePodReturnsTargetMissingWhenObservedPodAbsent(t *testing.T) {
	applicationID := uuid.New()
	runtimeSpecID := uuid.New()
	deleted := false
	svc := New(stubStore{
		getRuntimeSpecFunc: func(context.Context, uuid.UUID) (*runtimedomain.RuntimeSpec, error) {
			return &runtimedomain.RuntimeSpec{ID: runtimeSpecID, ApplicationID: applicationID, Environment: "staging"}, nil
		},
		resolveTargetNamespaceFunc: func(context.Context, uuid.UUID, string) (string, error) {
			return "devflow-staging", nil
		},
		listObservedPodsFunc: func(context.Context, uuid.UUID) ([]*runtimedomain.RuntimeObservedPod, error) {
			return []*runtimedomain.RuntimeObservedPod{{RuntimeSpecID: runtimeSpecID, Namespace: "devflow-staging", PodName: "other-pod"}}, nil
		},
	}, stubK8sExecutor{
		deletePodFunc: func(context.Context, string, string) error {
			deleted = true
			return nil
		},
	})

	_, err := svc.DeletePod(context.Background(), runtimeSpecID, "demo-0", "tester")
	if !sharederrs.HasCode(err, sharederrs.CodeFailedPrecondition) {
		t.Fatalf("expected failed_precondition error, got %v", err)
	}
	if err == nil || err.Error() != ErrRuntimePodTargetMissing.Error() {
		t.Fatalf("error = %v, want %v", err, ErrRuntimePodTargetMissing)
	}
	if deleted {
		t.Fatalf("expected delete executor not to be called")
	}
}

func TestDeletePodReturnsNamespaceMismatchWhenObservedPodLeavesResolvedNamespace(t *testing.T) {
	applicationID := uuid.New()
	runtimeSpecID := uuid.New()
	deleted := false
	svc := New(stubStore{
		getRuntimeSpecFunc: func(context.Context, uuid.UUID) (*runtimedomain.RuntimeSpec, error) {
			return &runtimedomain.RuntimeSpec{ID: runtimeSpecID, ApplicationID: applicationID, Environment: "staging"}, nil
		},
		resolveTargetNamespaceFunc: func(context.Context, uuid.UUID, string) (string, error) {
			return "devflow-staging", nil
		},
		listObservedPodsFunc: func(context.Context, uuid.UUID) ([]*runtimedomain.RuntimeObservedPod, error) {
			return []*runtimedomain.RuntimeObservedPod{{RuntimeSpecID: runtimeSpecID, Namespace: "other-namespace", PodName: "demo-0"}}, nil
		},
	}, stubK8sExecutor{
		deletePodFunc: func(context.Context, string, string) error {
			deleted = true
			return nil
		},
	})

	_, err := svc.DeletePod(context.Background(), runtimeSpecID, "demo-0", "tester")
	if !sharederrs.HasCode(err, sharederrs.CodeInvalidArgument) {
		t.Fatalf("expected invalid_argument error, got %v", err)
	}
	if err == nil || err.Error() != ErrNamespaceMismatch.Error() {
		t.Fatalf("error = %v, want %v", err, ErrNamespaceMismatch)
	}
	if deleted {
		t.Fatalf("expected delete executor not to be called")
	}
}

func TestDeletePodDeletesObservedPodInResolvedNamespaceAndReturnsAcknowledgement(t *testing.T) {
	applicationID := uuid.New()
	runtimeSpecID := uuid.New()
	var deletedNamespace string
	var deletedName string
	var recorded *runtimedomain.RuntimeOperation
	svc := New(stubStore{
		getRuntimeSpecFunc: func(context.Context, uuid.UUID) (*runtimedomain.RuntimeSpec, error) {
			return &runtimedomain.RuntimeSpec{ID: runtimeSpecID, ApplicationID: applicationID, Environment: "staging"}, nil
		},
		resolveTargetNamespaceFunc: func(context.Context, uuid.UUID, string) (string, error) {
			return "devflow-staging", nil
		},
		listObservedPodsFunc: func(context.Context, uuid.UUID) ([]*runtimedomain.RuntimeObservedPod, error) {
			return []*runtimedomain.RuntimeObservedPod{{RuntimeSpecID: runtimeSpecID, Namespace: "devflow-staging", PodName: "demo-0"}}, nil
		},
		createRuntimeOperationFunc: func(_ context.Context, item *runtimedomain.RuntimeOperation) error {
			recorded = item
			return nil
		},
	}, stubK8sExecutor{
		deletePodFunc: func(_ context.Context, namespace, name string) error {
			deletedNamespace = namespace
			deletedName = name
			return nil
		},
	})

	ack, err := svc.DeletePod(context.Background(), runtimeSpecID, "demo-0", "tester")
	if err != nil {
		t.Fatalf("DeletePod() error = %v", err)
	}
	if deletedNamespace != "devflow-staging" || deletedName != "demo-0" {
		t.Fatalf("delete target = %s/%s, want devflow-staging/demo-0", deletedNamespace, deletedName)
	}
	if recorded == nil {
		t.Fatalf("expected runtime operation to be recorded")
	}
	if recorded.TargetNamespace != "devflow-staging" {
		t.Fatalf("recorded namespace = %q, want devflow-staging", recorded.TargetNamespace)
	}
	if recorded.ConvergenceState != RuntimeConvergencePending {
		t.Fatalf("convergence state = %q, want %q", recorded.ConvergenceState, RuntimeConvergencePending)
	}
	if ack == nil {
		t.Fatal("expected acknowledgement")
	}
	if ack.MutationState != RuntimeMutationAccepted {
		t.Fatalf("mutation state = %q, want %q", ack.MutationState, RuntimeMutationAccepted)
	}
	if ack.ConvergenceState != RuntimeConvergencePending {
		t.Fatalf("convergence state = %q, want %q", ack.ConvergenceState, RuntimeConvergencePending)
	}
	if ack.TargetKind != "pod" {
		t.Fatalf("target kind = %q, want pod", ack.TargetKind)
	}
	if ack.TargetNamespace != "devflow-staging" {
		t.Fatalf("target namespace = %q, want devflow-staging", ack.TargetNamespace)
	}
}

func TestResolveRuntimeNamespaceReturnsExplicitNamespaceUnresolved(t *testing.T) {
	applicationID := uuid.New()
	svc := New(stubStore{
		resolveTargetNamespaceFunc: func(context.Context, uuid.UUID, string) (string, error) {
			return "", sql.ErrNoRows
		},
		findRuntimeSpecByApplicationEnvFunc: func(context.Context, uuid.UUID, string) (*runtimedomain.RuntimeSpec, error) {
			return nil, sql.ErrNoRows
		},
	}, nil)

	_, err := svc.(*runtimeService).resolveRuntimeNamespace(context.Background(), applicationID, "staging", "")
	if !sharederrs.HasCode(err, sharederrs.CodeFailedPrecondition) {
		t.Fatalf("expected failed_precondition error, got %v", err)
	}
	if err == nil || err.Error() != ErrRuntimeNamespaceUnresolved.Error() {
		t.Fatalf("error = %v, want %v", err, ErrRuntimeNamespaceUnresolved)
	}
}

func TestRestartDeploymentFallsBackToObservedWorkloadNameAndReturnsAcknowledgement(t *testing.T) {
	applicationID := uuid.New()
	runtimeSpecID := uuid.New()
	var restartedName string
	var recorded *runtimedomain.RuntimeOperation
	svc := New(stubStore{
		getRuntimeSpecFunc: func(context.Context, uuid.UUID) (*runtimedomain.RuntimeSpec, error) {
			return &runtimedomain.RuntimeSpec{ID: runtimeSpecID, ApplicationID: applicationID, Environment: "production"}, nil
		},
		resolveTargetNamespaceFunc: func(context.Context, uuid.UUID, string) (string, error) {
			return "devflow", nil
		},
		getObservedWorkloadFunc: func(context.Context, uuid.UUID) (*runtimedomain.RuntimeObservedWorkload, error) {
			return &runtimedomain.RuntimeObservedWorkload{RuntimeSpecID: runtimeSpecID, Namespace: "devflow", WorkloadKind: "Deployment", WorkloadName: "meta-service"}, nil
		},
		createRuntimeOperationFunc: func(context.Context, *runtimedomain.RuntimeOperation) error {
			recorded = &runtimedomain.RuntimeOperation{}
			*recorded = *recorded
			return nil
		},
	}, stubK8sExecutor{
		restartDeploymentFunc: func(_ context.Context, namespace, name string) error {
			restartedName = name
			return nil
		},
	})

	ack, err := svc.RestartDeployment(context.Background(), runtimeSpecID, "", "tester")
	if err != nil {
		t.Fatalf("RestartDeployment() error = %v", err)
	}
	if restartedName != "meta-service" {
		t.Fatalf("restartedName = %s, want meta-service", restartedName)
	}
	if ack == nil {
		t.Fatal("expected acknowledgement")
	}
	if ack.TargetKind != "deployment" {
		t.Fatalf("target kind = %q, want deployment", ack.TargetKind)
	}
	if ack.TargetName != "meta-service" {
		t.Fatalf("target name = %q, want meta-service", ack.TargetName)
	}
	if ack.TargetNamespace != "devflow" {
		t.Fatalf("target namespace = %q, want devflow", ack.TargetNamespace)
	}
	if ack.ObservedWorkload != "meta-service" {
		t.Fatalf("observed workload = %q, want meta-service", ack.ObservedWorkload)
	}
	if ack.MutationState != RuntimeMutationAccepted {
		t.Fatalf("mutation state = %q, want %q", ack.MutationState, RuntimeMutationAccepted)
	}
	if ack.ConvergenceState != RuntimeConvergencePending {
		t.Fatalf("convergence state = %q, want %q", ack.ConvergenceState, RuntimeConvergencePending)
	}
}
