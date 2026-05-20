package service

import (
	"context"
	"strings"
	"time"

	model "github.com/bsonger/devflow-service/internal/release/domain"
)

type releaseRemediationManager struct {
	service *releaseService
}

func newReleaseRemediationManager(service *releaseService) *releaseRemediationManager {
	return &releaseRemediationManager{service: service}
}

func (m *releaseRemediationManager) syncRollbackSource(ctx context.Context, release *model.Release, status model.ReleaseStatus) error {
	if release == nil || !strings.EqualFold(strings.TrimSpace(release.Type), model.ReleaseRollback) || release.RollbackSourceReleaseID == nil {
		return nil
	}

	var (
		nextStatus model.RemediationKind
		reason     string
	)
	switch status {
	case model.ReleaseRolledBack:
		nextStatus = model.RemediationRollbackDone
		reason = "rollback release succeeded"
	case model.ReleaseFailed, model.ReleaseSyncFailed:
		nextStatus = model.RemediationRollbackFailed
		reason = "rollback release failed"
	default:
		return nil
	}

	sourceRelease, err := m.service.loadRelease(ctx, *release.RollbackSourceReleaseID)
	if err != nil {
		return err
	}
	sourceRelease.RemediationStatus = string(nextStatus)
	sourceRelease.RemediationReason = reason
	sourceRelease.UpdatedAt = time.Now()
	return m.service.repoStore().UpdateRow(ctx, sourceRelease)
}
