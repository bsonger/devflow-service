package service

import (
	"context"
	"database/sql"
	"errors"
	"time"

	intentdomain "github.com/bsonger/devflow-service/internal/intent/domain"
	"github.com/bsonger/devflow-service/internal/intent/repository"
	platformobs "github.com/bsonger/devflow-service/internal/platform/runtime/observability"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	sharederrs "github.com/bsonger/devflow-service/internal/shared/errs"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type IntentListFilter struct {
	Kind         string
	Status       string
	ResourceType string
	ClaimedBy    string
	ResourceID   *uuid.UUID
}

var IntentService = &intentService{store: repository.NewPostgresStore()}

type intentService struct {
	store repository.Store
}

func (s *intentService) repoStore() repository.Store {
	if s.store == nil {
		s.store = repository.NewPostgresStore()
	}
	return s.store
}

var ErrIntentNotFound = sharederrs.NotFound("intent not found")

func (s *intentService) CreateReleaseIntent(ctx context.Context, release *model.Release) (uuid.UUID, error) {
	log := platformobs.OperationLogger(ctx, "release_service", "create_release_intent", "release_intent",
		zap.String("devflow.release.id", releaseID(release)),
	)
	intent := &intentdomain.Intent{
		Kind:         model.IntentKindRelease,
		Status:       model.IntentPending,
		ResourceType: "release",
		ResourceID:   release.ID,
	}
	intent.WithCreateDefault()
	if err := s.repoStore().Insert(ctx, intent); err != nil {
		platformobs.LogOperationFailure(log, "create release intent failed", err)
		return uuid.Nil, err
	}
	if err := s.repoStore().BindIntentToRelease(ctx, release.ID, intent.ID); err != nil {
		platformobs.LogOperationFailure(log, "bind release intent failed", err,
			zap.String("intent_id", intent.ID.String()),
		)
		return intent.ID, err
	}
	release.ExecutionIntentID = uuidPtr(intent.ID)
	platformobs.LogOperationSuccess(log, "release intent created",
		zap.String("resource_id", intent.ID.String()),
		zap.String("intent_id", intent.ID.String()),
		zap.String("intent_kind", string(intent.Kind)),
		zap.String("resource_type", intent.ResourceType),
		zap.String("release_id", release.ID.String()),
	)
	return intent.ID, nil
}

func (s *intentService) UpdateStatus(ctx context.Context, id uuid.UUID, status model.IntentStatus, externalRef, message string) error {
	log := platformobs.OperationLogger(ctx, "release_service", "update_intent_status", "release_intent",
		zap.String("resource_id", id.String()),
		zap.String("intent_status", string(status)),
	)
	if err := s.repoStore().UpdateStatus(ctx, id, status, message); err != nil {
		platformobs.LogOperationFailure(log, "update intent status failed", err)
		return err
	}
	platformobs.LogOperationSuccess(log, "intent status updated")
	return nil
}

func (s *intentService) Get(ctx context.Context, id uuid.UUID) (*intentdomain.Intent, error) {
	return s.repoStore().Get(ctx, id)
}

func (s *intentService) List(ctx context.Context, filter IntentListFilter) ([]*intentdomain.Intent, error) {
	return s.repoStore().List(ctx, repository.ListFilter(filter))
}

func (s *intentService) ListPending(ctx context.Context, limit int) ([]intentdomain.Intent, error) {
	return s.repoStore().ListPending(ctx, limit)
}

func (s *intentService) ClaimNextPending(ctx context.Context, workerID string, leaseDuration time.Duration) (*intentdomain.Intent, error) {
	intent, err := s.repoStore().ClaimNextPending(ctx, workerID, leaseDuration)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrIntentNotFound
	}
	log := platformobs.OperationLogger(ctx, "release_service", "claim_next_intent", "release_intent",
		zap.String("worker_id", workerID),
	)
	if err != nil {
		platformobs.LogOperationFailure(log, "claim next intent failed", err)
		return nil, err
	}
	platformobs.LogOperationSuccess(log, "intent claimed",
		zap.String("resource_id", intent.ID.String()),
		zap.String("intent_id", intent.ID.String()),
		zap.Duration("lease_duration", leaseDuration),
		zap.String("intent_kind", string(intent.Kind)),
	)
	return intent, nil
}

func (s *intentService) ClaimNextPendingByKind(ctx context.Context, kind model.IntentKind, workerID string, leaseDuration time.Duration) (*intentdomain.Intent, error) {
	intent, err := s.repoStore().ClaimNextPendingByKind(ctx, kind, workerID, leaseDuration)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrIntentNotFound
	}
	log := platformobs.OperationLogger(ctx, "release_service", "claim_next_intent_by_kind", "release_intent",
		zap.String("worker_id", workerID),
		zap.String("intent_kind", string(kind)),
	)
	if err != nil {
		platformobs.LogOperationFailure(log, "claim next intent by kind failed", err)
		return nil, err
	}
	platformobs.LogOperationSuccess(log, "intent claimed",
		zap.String("resource_id", intent.ID.String()),
		zap.String("intent_id", intent.ID.String()),
		zap.Duration("lease_duration", leaseDuration),
	)
	return intent, nil
}

func (s *intentService) MarkSubmitted(ctx context.Context, id uuid.UUID, externalRef, message string) error {
	log := platformobs.OperationLogger(ctx, "release_service", "mark_intent_submitted", "release_intent",
		zap.String("resource_id", id.String()),
	)
	if err := s.repoStore().MarkSubmitted(ctx, id, message); err != nil {
		platformobs.LogOperationFailure(log, "mark intent submitted failed", err)
		return err
	}
	platformobs.LogOperationSuccess(log, "intent submitted")
	return nil
}

func (s *intentService) MarkFailed(ctx context.Context, id uuid.UUID, message string) error {
	log := platformobs.OperationLogger(ctx, "release_service", "mark_intent_failed", "release_intent",
		zap.String("resource_id", id.String()),
	)
	if err := s.repoStore().MarkFailed(ctx, id, message); err != nil {
		platformobs.LogOperationFailure(log, "mark intent failed failed", err)
		return err
	}
	platformobs.LogOperationSuccess(log, "intent failed")
	return nil
}

func (s *intentService) UpdateStatusByResource(ctx context.Context, kind model.IntentKind, resourceID uuid.UUID, status model.IntentStatus, externalRef, message string) error {
	log := platformobs.OperationLogger(ctx, "release_service", "update_intent_status_by_resource", "release_intent",
		zap.String("intent_kind", string(kind)),
		zap.String("resource_id", resourceID.String()),
		zap.String("intent_status", string(status)),
	)
	if err := s.repoStore().UpdateStatusByResource(ctx, kind, resourceID, status, message); err != nil {
		platformobs.LogOperationFailure(log, "update intent status by resource failed", err)
		return err
	}
	platformobs.LogOperationSuccess(log, "intent status updated by resource")
	return nil
}

func uuidPtr(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	v := id
	return &v
}

func releaseID(release *model.Release) string {
	if release == nil || release.ID == uuid.Nil {
		return ""
	}
	return release.ID.String()
}
