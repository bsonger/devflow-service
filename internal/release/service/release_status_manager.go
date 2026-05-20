package service

import (
	"context"
	"time"

	platformobs "github.com/bsonger/devflow-service/internal/platform/runtime/observability"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type releaseStatusManager struct {
	service *releaseService
}

func newReleaseStatusManager(service *releaseService) *releaseStatusManager {
	return &releaseStatusManager{service: service}
}

func (m *releaseStatusManager) updateStatus(ctx context.Context, releaseID uuid.UUID, status model.ReleaseStatus) error {
	release, err := m.service.loadRelease(ctx, releaseID)
	if err != nil {
		return err
	}
	if isReleaseTerminalStatus(release.Status) {
		return nil
	}
	if release.Status == status {
		return nil
	}
	previousStatus := release.Status
	release.Status = status
	release.UpdatedAt = time.Now()
	if err := m.service.repoStore().UpdateRow(ctx, release); err != nil {
		return err
	}

	statusLog := platformobs.OperationLogger(ctx, "release_service", "update_release_status", "release",
		zap.String("resource_id", release.ID.String()),
	)
	statusLog.Info("release status updated",
		zap.String("result", "success"),
		zap.String("previous_status", string(previousStatus)),
		zap.String("status", string(status)),
	)

	if err := newReleaseRemediationManager(m.service).syncRollbackSource(ctx, release, status); err != nil {
		return err
	}
	observeReleaseTerminal(ctx, release, status)
	newReleaseObserveController(m.service).runTerminal(ctx, release, status)
	return nil
}
