package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/bsonger/devflow-service/internal/release/repository"
	"github.com/google/uuid"
)

func (s *releaseService) ApplyOperationRequest(ctx context.Context, releaseID uuid.UUID, operation releasedomain.ReleaseOperation, now time.Time) (bool, error) {
	release, err := s.loadRelease(ctx, releaseID)
	if err != nil {
		return false, err
	}
	if normalizeReleaseOperationForService(operation) == releasedomain.ReleaseOperationRollback {
		return s.applyRollbackOperation(ctx, release, now)
	}

	state, ok := compatControlStateFromRelease(release)
	if !ok {
		return false, fmt.Errorf("release %s cannot be mapped to control state", releaseID)
	}
	decision := releasedomain.EvaluateOperationRequest(state, operation)
	if !decision.Allowed {
		return false, fmt.Errorf("%s", strings.TrimSpace(decision.Reason))
	}

	switch decision.TargetStatus {
	case releasedomain.LifecyclePaused:
		if err := s.applyOperationStepMessage(ctx, release.ID, "observe_rollout", "deployment paused by operator", now); err != nil {
			return false, err
		}
	case releasedomain.LifecycleRunning:
		if err := s.applyOperationStepMessage(ctx, release.ID, "observe_rollout", "deployment resumed by operator", now); err != nil {
			return false, err
		}
	case releasedomain.LifecycleFailed:
		if err := s.applyOperationCancellation(ctx, release.ID, state, operation, now); err != nil {
			return false, err
		}
	case releasedomain.LifecyclePending:
		return false, fmt.Errorf("rollback execution is not implemented yet")
	}

	return true, nil
}

func (s *releaseService) applyRollbackOperation(ctx context.Context, release *releasedomain.Release, now time.Time) (bool, error) {
	if release == nil {
		return false, fmt.Errorf("release is required")
	}
	if strings.TrimSpace(release.RemediationStatus) != string(releasedomain.RemediationPendingRollback) {
		return false, fmt.Errorf("rollback requires remediation status %q", releasedomain.RemediationPendingRollback)
	}

	target, err := s.selectRollbackTarget(ctx, release)
	if err != nil {
		return false, err
	}

	if _, err := s.spawnRollbackRelease(ctx, release, target, now); err != nil {
		return false, err
	}

	release.RemediationStatus = string(releasedomain.RemediationRollingBack)
	release.RemediationReason = "rollback release created"
	release.UpdatedAt = now
	if err := s.repoStore().UpdateRow(ctx, release); err != nil {
		return false, err
	}
	return true, nil
}

type rollbackTarget struct {
	sourceReleaseID    uuid.UUID
	artifactRepository string
	artifactTag        string
	artifactDigest     string
	artifactRef        string
}

func (s *releaseService) selectRollbackTarget(ctx context.Context, release *releasedomain.Release) (rollbackTarget, error) {
	if release == nil {
		return rollbackTarget{}, fmt.Errorf("release is required")
	}

	items, err := s.repoStore().List(ctx, repository.ListFilter{
		ApplicationID: &release.ApplicationID,
		EnvironmentID: release.EnvironmentID,
	})
	if err != nil {
		return rollbackTarget{}, err
	}

	candidates := make([]*releasedomain.Release, 0, len(items))
	for _, item := range items {
		if item == nil || item.ID == release.ID {
			continue
		}
		if item.Status != releasedomain.ReleaseSucceeded {
			continue
		}
		if strings.TrimSpace(item.ArtifactRef) == "" && strings.TrimSpace(item.ArtifactDigest) == "" && strings.TrimSpace(item.ArtifactRepository) == "" {
			continue
		}
		candidates = append(candidates, item)
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].CreatedAt.After(candidates[j].CreatedAt)
	})
	if len(candidates) == 0 {
		return rollbackTarget{}, fmt.Errorf("rollback requires a previously succeeded release with deployment artifact in environment %q", strings.TrimSpace(release.EnvironmentID))
	}

	chosen := candidates[0]
	return rollbackTarget{
		sourceReleaseID:    chosen.ID,
		artifactRepository: strings.TrimSpace(chosen.ArtifactRepository),
		artifactTag:        strings.TrimSpace(chosen.ArtifactTag),
		artifactDigest:     strings.TrimSpace(chosen.ArtifactDigest),
		artifactRef:        strings.TrimSpace(chosen.ArtifactRef),
	}, nil
}

func (s *releaseService) spawnRollbackRelease(ctx context.Context, release *releasedomain.Release, target rollbackTarget, now time.Time) (uuid.UUID, error) {
	if release == nil {
		return uuid.Nil, fmt.Errorf("release is required")
	}
	sourceReleaseID := target.sourceReleaseID
	rollbackRelease := &releasedomain.Release{
		ManifestID:                       release.ManifestID,
		ApplicationID:                    release.ApplicationID,
		EnvironmentID:                    release.EnvironmentID,
		Strategy:                         release.Strategy,
		Type:                             releasedomain.ReleaseRollback,
		Status:                           releasedomain.ReleaseSyncing,
		Steps:                            releasedomain.DefaultReleaseSteps(releasedomain.ReleaseStrategyToType(release.Strategy), releasedomain.ReleaseRollback),
		AppConfigSnapshot:                release.AppConfigSnapshot,
		RoutesSnapshot:                   release.RoutesSnapshot,
		RollbackSourceReleaseID:          &sourceReleaseID,
		RollbackTargetArtifactRepository: target.artifactRepository,
		RollbackTargetArtifactTag:        target.artifactTag,
		RollbackTargetArtifactDigest:     target.artifactDigest,
		RollbackTargetArtifactRef:        target.artifactRef,
	}
	markReleaseStepCompleted(rollbackRelease, "freeze_inputs", "rollback release inputs frozen successfully")
	if stepCode, message := releaseDeploymentStartStep(rollbackRelease); stepCode != "" {
		for i := range rollbackRelease.Steps {
			if rollbackRelease.Steps[i].Code != stepCode {
				continue
			}
			rollbackRelease.Steps[i].Status = releasedomain.StepRunning
			rollbackRelease.Steps[i].Progress = 25
			rollbackRelease.Steps[i].Message = message
			break
		}
	}
	rollbackRelease.WithCreateDefault()
	if !now.IsZero() {
		rollbackRelease.CreatedAt = now
		rollbackRelease.UpdatedAt = now
	}
	if err := s.repoStore().Insert(ctx, rollbackRelease); err != nil {
		return uuid.Nil, err
	}
	return rollbackRelease.ID, nil
}

func applyPausedCompatibilityState(release *releasedomain.Release) (*releasedomain.Release, bool) {
	if release == nil {
		return nil, false
	}
	observeRollout := findReleaseStep(release.Steps, "observe_rollout")
	if observeRollout == nil {
		return release, false
	}
	message := strings.ToLower(strings.TrimSpace(observeRollout.Message))
	if observeRollout.Status == releasedomain.StepRunning && strings.Contains(message, "paused by operator") {
		copyRelease := *release
		return &copyRelease, true
	}
	return release, false
}

func (s *releaseService) applyOperationStepMessage(ctx context.Context, releaseID uuid.UUID, stepCode, message string, now time.Time) error {
	release, err := s.loadRelease(ctx, releaseID)
	if err != nil {
		return err
	}
	if isReleaseTerminalStatus(release.Status) {
		return nil
	}
	nextSteps := cloneReleaseSteps(release.Steps)
	step := findReleaseStep(nextSteps, stepCode)
	if step == nil {
		return ErrReleaseUnknownStep
	}
	applyReleaseStepUpdate(nextSteps, stepCode, step.Status, step.Progress, strings.TrimSpace(message), nil, nil)
	release.Steps = nextSteps
	release.UpdatedAt = now
	return s.repoStore().UpdateSteps(ctx, release)
}

func (s *releaseService) applyOperationCancellation(ctx context.Context, releaseID uuid.UUID, state releasedomain.ReleaseControlState, operation releasedomain.ReleaseOperation, now time.Time) error {
	stepCode := currentLifecycleStepCode(state)
	remediation := releasedomain.AssessRemediationNeed(state, stepCode, operation)
	message := "release canceled by operator"
	switch remediation.Kind {
	case releasedomain.RemediationCleanupOnly:
		message = "release canceled by operator; cleanup required"
	case releasedomain.RemediationPendingRollback:
		message = "release canceled by operator; rollback required"
	}
	if err := s.applyOperationFailureStep(ctx, releaseID, stepCode, now, message); err != nil {
		return err
	}
	if err := s.applyReleaseRemediation(ctx, releaseID, remediation, now); err != nil {
		return err
	}
	return s.UpdateStatus(ctx, releaseID, releasedomain.ReleaseFailed)
}

func (s *releaseService) applyOperationStepTerminalFailure(ctx context.Context, releaseID uuid.UUID, state releasedomain.ReleaseControlState, now time.Time, message string) error {
	stepCode := "observe_rollout"
	switch state.LifecycleStatus {
	case releasedomain.LifecycleDispatching:
		stepCode = "start_deployment"
	case releasedomain.LifecycleFinalizing, releasedomain.LifecyclePaused:
		stepCode = "finalize_release"
	}
	if err := s.applyTimeoutStepWrite(ctx, releaseID, releasedomain.StepWrite{
		StepCode: stepCode,
		Status:   releasedomain.StepFailed,
		Progress: 100,
		Message:  strings.TrimSpace(message),
	}, now); err != nil {
		return err
	}
	return s.UpdateStatus(ctx, releaseID, releasedomain.ReleaseFailed)
}

func (s *releaseService) applyOperationFailureStep(ctx context.Context, releaseID uuid.UUID, stepCode string, now time.Time, message string) error {
	return s.applyTimeoutStepWrite(ctx, releaseID, releasedomain.StepWrite{
		StepCode: stepCode,
		Status:   releasedomain.StepFailed,
		Progress: 100,
		Message:  strings.TrimSpace(message),
	}, now)
}

func currentLifecycleStepCode(state releasedomain.ReleaseControlState) string {
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

func (s *releaseService) applyReleaseRemediation(ctx context.Context, releaseID uuid.UUID, decision releasedomain.RemediationDecision, now time.Time) error {
	release, err := s.loadRelease(ctx, releaseID)
	if err != nil {
		return err
	}
	release.RemediationStatus = string(decision.Kind)
	release.RemediationReason = strings.TrimSpace(decision.Reason)
	release.UpdatedAt = now
	return s.repoStore().UpdateRow(ctx, release)
}
