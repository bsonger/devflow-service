package service

import (
	"context"
	"strings"
	"time"

	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/google/uuid"
)

func (s *releaseService) ApplyOperationRequest(ctx context.Context, releaseID uuid.UUID, operation releasedomain.ReleaseOperation, now time.Time) (bool, error) {
	return newReleaseOperationManager(s).applyOperationRequest(ctx, releaseID, operation, now)
}

func applyPausedCompatibilityState(release *releasedomain.Release) (*releasedomain.Release, bool) {
	if release == nil {
		return nil, false
	}
	if hasPausedRunningStep(release) {
		copyRelease := *release
		return &copyRelease, true
	}
	return release, false
}

func hasPausedRunningStep(release *releasedomain.Release) bool {
	if release == nil {
		return false
	}
	for _, step := range normalizeReleaseSteps(release) {
		if step.Status != releasedomain.StepRunning {
			continue
		}
		message := strings.ToLower(strings.TrimSpace(step.Message))
		if strings.Contains(message, "paused by operator") {
			return true
		}
	}
	return false
}

func currentLifecycleStepCode(release *releasedomain.Release, state releasedomain.ReleaseControlState) string {
	if release != nil {
		for _, step := range normalizeReleaseSteps(release) {
			if step.Status == releasedomain.StepRunning {
				return step.Code
			}
		}
	}
	switch state.LifecycleStatus {
	case releasedomain.LifecycleDispatching:
		return "create_argocd_application"
	case releasedomain.LifecyclePaused, releasedomain.LifecycleRunning:
		return "observe_rollout"
	case releasedomain.LifecycleFinalizing:
		return "finalize_release"
	default:
		return "observe_rollout"
	}
}

func normalizeReleaseOperationForService(operation releasedomain.ReleaseOperation) releasedomain.ReleaseOperation {
	switch strings.ToLower(strings.TrimSpace(string(operation))) {
	case "pause":
		return releasedomain.ReleaseOperationPause
	case "resume":
		return releasedomain.ReleaseOperationResume
	case "cancel":
		return releasedomain.ReleaseOperationCancel
	case "rollback":
		return releasedomain.ReleaseOperationRollback
	default:
		return operation
	}
}
