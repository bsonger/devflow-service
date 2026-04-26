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
	deleteRuntimeSpecByApplicationEnvFunc func(context.Context, uuid.UUID, string) error
	listRuntimeSpecsFunc                  func(context.Context) ([]*runtimedomain.RuntimeSpec, error)
	nextRevisionNumberFunc                func(context.Context, uuid.UUID) (int, error)
	createRuntimeSpecRevisionFunc         func(context.Context, *runtimedomain.RuntimeSpecRevision) error
	updateCurrentRevisionFunc             func(context.Context, uuid.UUID, uuid.UUID) error
	listRuntimeSpecRevisionsFunc          func(context.Context, uuid.UUID) ([]*runtimedomain.RuntimeSpecRevision, error)
	getRuntimeSpecRevisionFunc            func(context.Context, uuid.UUID) (*runtimedomain.RuntimeSpecRevision, error)
	upsertObservedPodFunc                 func(context.Context, *runtimedomain.RuntimeObservedPod) error
	deleteObservedPodFunc                 func(context.Context, uuid.UUID, string, string, time.Time) error
	listObservedPodsFunc                  func(context.Context, uuid.UUID) ([]*runtimedomain.RuntimeObservedPod, error)
	createRuntimeOperationFunc            func(context.Context, *runtimedomain.RuntimeOperation) error
	listRuntimeOperationsFunc             func(context.Context, uuid.UUID) ([]*runtimedomain.RuntimeOperation, error)
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

func (s stubStore) GetRuntimeSpecByApplicationEnv(ctx context.Context, applicationID uuid.UUID, environment string) (*runtimedomain.RuntimeSpec, error) {
	if s.getRuntimeSpecByApplicationEnvFunc != nil {
		return s.getRuntimeSpecByApplicationEnvFunc(ctx, applicationID, environment)
	}
	return nil, nil
}

func (s stubStore) DeleteRuntimeSpecByApplicationEnv(ctx context.Context, applicationID uuid.UUID, environment string) error {
	if s.deleteRuntimeSpecByApplicationEnvFunc != nil {
		return s.deleteRuntimeSpecByApplicationEnvFunc(ctx, applicationID, environment)
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
	applicationID := uuid.New()
	svc := New(stubStore{
		getRuntimeSpecByApplicationEnvFunc: func(context.Context, uuid.UUID, string) (*runtimedomain.RuntimeSpec, error) {
			return &runtimedomain.RuntimeSpec{ID: uuid.New(), ApplicationID: applicationID, Environment: "staging"}, nil
		},
	}, nil)

	_, err := svc.CreateRuntimeSpec(context.Background(), CreateRuntimeSpecInput{
		ApplicationID: applicationID,
		Environment:   "staging",
	})
	if !sharederrs.HasCode(err, sharederrs.CodeConflict) {
		t.Fatalf("expected conflict error, got %v", err)
	}
}

func TestCreateRuntimeSpecRevisionUsesNextRevisionAndUpdatesCurrent(t *testing.T) {
	applicationID := uuid.New()
	runtimeSpecID := uuid.New()
	capturedRevision := 0
	updatedRuntimeSpecID := uuid.Nil
	updatedRevisionID := uuid.Nil
	svc := New(stubStore{
		getRuntimeSpecFunc: func(context.Context, uuid.UUID) (*runtimedomain.RuntimeSpec, error) {
			return &runtimedomain.RuntimeSpec{ID: runtimeSpecID, ApplicationID: applicationID, Environment: "staging"}, nil
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
	applicationID := uuid.New()
	svc := New(stubStore{
		getRuntimeSpecByApplicationEnvFunc: func(context.Context, uuid.UUID, string) (*runtimedomain.RuntimeSpec, error) {
			return &runtimedomain.RuntimeSpec{ID: uuid.New(), ApplicationID: applicationID, Environment: "staging"}, nil
		},
	}, nil)

	_, err := svc.SyncObservedPod(context.Background(), SyncObservedPodInput{
		ApplicationID: applicationID,
		Environment:   "staging",
		Namespace:     "wrong-namespace",
		PodName:       "demo-0",
		Phase:         "Running",
	})
	if !sharederrs.HasCode(err, sharederrs.CodeInvalidArgument) {
		t.Fatalf("expected invalid_argument error, got %v", err)
	}
}
