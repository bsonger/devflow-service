package repository

import (
	"context"
	"time"

	runtimedomain "github.com/bsonger/devflow-service/internal/runtime/domain"
	"github.com/google/uuid"
)

type Store interface {
	CreateRuntimeSpec(context.Context, *runtimedomain.RuntimeSpec) error
	GetRuntimeSpec(context.Context, uuid.UUID) (*runtimedomain.RuntimeSpec, error)
	GetRuntimeSpecByApplicationEnv(context.Context, uuid.UUID, string) (*runtimedomain.RuntimeSpec, error)
	EnsureRuntimeSpecByApplicationEnv(context.Context, uuid.UUID, string) (*runtimedomain.RuntimeSpec, error)
	GetApplicationName(context.Context, uuid.UUID) (string, error)
	ResolveTargetNamespace(context.Context, uuid.UUID, string) (string, error)
	DeleteRuntimeSpecByApplicationEnv(context.Context, uuid.UUID, string) error
	ListRuntimeSpecs(context.Context) ([]*runtimedomain.RuntimeSpec, error)
	NextRevisionNumber(context.Context, uuid.UUID) (int, error)
	CreateRuntimeSpecRevision(context.Context, *runtimedomain.RuntimeSpecRevision) error
	UpdateCurrentRevision(context.Context, uuid.UUID, uuid.UUID) error
	ListRuntimeSpecRevisions(context.Context, uuid.UUID) ([]*runtimedomain.RuntimeSpecRevision, error)
	GetRuntimeSpecRevision(context.Context, uuid.UUID) (*runtimedomain.RuntimeSpecRevision, error)
	UpsertObservedWorkload(context.Context, *runtimedomain.RuntimeObservedWorkload) error
	DeleteObservedWorkload(context.Context, uuid.UUID, string, string, string, time.Time) error
	GetObservedWorkload(context.Context, uuid.UUID) (*runtimedomain.RuntimeObservedWorkload, error)
	UpsertObservedPod(context.Context, *runtimedomain.RuntimeObservedPod) error
	DeleteObservedPod(context.Context, uuid.UUID, string, string, time.Time) error
	ListObservedPods(context.Context, uuid.UUID) ([]*runtimedomain.RuntimeObservedPod, error)
	CreateRuntimeOperation(context.Context, *runtimedomain.RuntimeOperation) error
	ListRuntimeOperations(context.Context, uuid.UUID) ([]*runtimedomain.RuntimeOperation, error)
}

var RuntimeStore Store = NewMemoryStore()
