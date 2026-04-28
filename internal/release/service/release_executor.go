package service

import (
	"context"
	"time"

	intentdomain "github.com/bsonger/devflow-service/internal/intent/domain"
	intentservice "github.com/bsonger/devflow-service/internal/intent/service"
	"github.com/bsonger/devflow-service/internal/platform/logger"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type releaseIntentCoordinator interface {
	ClaimNextPendingByKind(ctx context.Context, kind model.IntentKind, workerID string, leaseDuration time.Duration) (*intentdomain.Intent, error)
	MarkSubmitted(ctx context.Context, id uuid.UUID, externalRef, message string) error
	MarkFailed(ctx context.Context, id uuid.UUID, message string) error
}

var releaseIntentService releaseIntentCoordinator = intentservice.IntentService

var dispatchReleaseIntent = func(ctx context.Context, svc *releaseService, releaseID uuid.UUID) error {
	return svc.DispatchRelease(ctx, releaseID)
}

func (s *releaseService) ProcessNextReleaseIntent(ctx context.Context, workerID string, leaseDuration time.Duration) (bool, error) {
	intent, err := releaseIntentService.ClaimNextPendingByKind(ctx, model.IntentKindRelease, workerID, leaseDuration)
	if err == intentservice.ErrIntentNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	log := logger.LoggerWithContext(ctx)
	if log == nil {
		log = zap.NewNop()
	}
	log = log.With(
		zap.String("operation", "process_release_intent"),
		zap.String("resource", "intent"),
		zap.String("intent_id", intent.ID.String()),
		zap.String("release_id", intent.ResourceID.String()),
		zap.String("worker_id", workerID),
	)

	if err := dispatchReleaseIntent(ctx, s, intent.ResourceID); err != nil {
		_ = releaseIntentService.MarkFailed(ctx, intent.ID, err.Error())
		log.Error("release intent execution failed", zap.String("result", "error"), zap.Error(err))
		return true, err
	}

	if err := releaseIntentService.MarkSubmitted(ctx, intent.ID, "", "release dispatched"); err != nil {
		return true, err
	}
	log.Info("release intent dispatched", zap.String("result", "success"))
	return true, nil
}
