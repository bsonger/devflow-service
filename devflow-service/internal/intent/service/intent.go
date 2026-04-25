package service

import (
	"context"
	"database/sql"
	"errors"
	"time"

	imagedomain "github.com/bsonger/devflow-service/internal/image/domain"
	intentdomain "github.com/bsonger/devflow-service/internal/intent/domain"
	"github.com/bsonger/devflow-service/internal/intent/repository"
	"github.com/bsonger/devflow-service/internal/platform/logger"
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

func (s *intentService) CreateBuildIntent(ctx context.Context, image *imagedomain.Image) (uuid.UUID, error) {
	intent := &intentdomain.Intent{
		Kind:         model.IntentKindBuild,
		Status:       model.IntentPending,
		ResourceType: "image",
		ResourceID:   image.ID,
	}
	intent.WithCreateDefault()
	if err := s.repoStore().Insert(ctx, intent); err != nil {
		return uuid.Nil, err
	}
	if err := s.repoStore().BindIntentToImage(ctx, image.ID, intent.ID); err != nil {
		return intent.ID, err
	}
	image.ExecutionIntentID = uuidPtr(intent.ID)
	logger.LoggerWithContext(ctx).Info("build intent created",
		zap.String("result", "success"),
		zap.String("resource", "intent"),
		zap.String("intent_id", intent.ID.String()),
		zap.String("intent_kind", string(intent.Kind)),
		zap.String("resource_type", intent.ResourceType),
		zap.String("resource_id", intent.ResourceID.String()),
		zap.String("image_id", image.ID.String()),
	)
	return intent.ID, nil
}

func (s *intentService) CreateReleaseIntent(ctx context.Context, release *model.Release) (uuid.UUID, error) {
	intent := &intentdomain.Intent{
		Kind:         model.IntentKindRelease,
		Status:       model.IntentPending,
		ResourceType: "release",
		ResourceID:   release.ID,
	}
	intent.WithCreateDefault()
	if err := s.repoStore().Insert(ctx, intent); err != nil {
		return uuid.Nil, err
	}
	if err := s.repoStore().BindIntentToRelease(ctx, release.ID, intent.ID); err != nil {
		return intent.ID, err
	}
	release.ExecutionIntentID = uuidPtr(intent.ID)
	logger.LoggerWithContext(ctx).Info("release intent created",
		zap.String("result", "success"),
		zap.String("resource", "intent"),
		zap.String("intent_id", intent.ID.String()),
		zap.String("intent_kind", string(intent.Kind)),
		zap.String("resource_type", intent.ResourceType),
		zap.String("resource_id", intent.ResourceID.String()),
		zap.String("release_id", release.ID.String()),
	)
	return intent.ID, nil
}

func (s *intentService) UpdateStatus(ctx context.Context, id uuid.UUID, status model.IntentStatus, externalRef, message string) error {
	return s.repoStore().UpdateStatus(ctx, id, status, message)
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
	if err != nil {
		return nil, err
	}
	return intent, nil
}

func (s *intentService) MarkSubmitted(ctx context.Context, id uuid.UUID, externalRef, message string) error {
	return s.repoStore().MarkSubmitted(ctx, id, message)
}

func (s *intentService) MarkFailed(ctx context.Context, id uuid.UUID, message string) error {
	return s.repoStore().MarkFailed(ctx, id, message)
}

func (s *intentService) UpdateStatusByResource(ctx context.Context, kind model.IntentKind, resourceID uuid.UUID, status model.IntentStatus, externalRef, message string) error {
	return s.repoStore().UpdateStatusByResource(ctx, kind, resourceID, status, message)
}

func uuidPtr(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	v := id
	return &v
}
