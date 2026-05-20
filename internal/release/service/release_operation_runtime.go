package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/google/uuid"
)

func (s *releaseService) ApplyOperationRequest(ctx context.Context, releaseID uuid.UUID, operation releasedomain.ReleaseOperation, now time.Time) (bool, error) {
	release, err := s.loadRelease(ctx, releaseID)
	if err != nil {
		return false, err
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
