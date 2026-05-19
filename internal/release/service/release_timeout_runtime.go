package service

import (
	"context"
	"strings"
	"time"

	releasecontrol "github.com/bsonger/devflow-service/internal/release/control"
	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/bsonger/devflow-service/internal/release/repository"
	"github.com/google/uuid"
)

type releaseTimeoutRuntime struct {
	scanner releasecontrol.TimeoutScanner
}

func newReleaseTimeoutRuntime() releaseTimeoutRuntime {
	return releaseTimeoutRuntime{
		scanner: releasecontrol.NewTimeoutScanner(),
	}
}

func (s *releaseService) ProcessTimeouts(ctx context.Context, now time.Time) (int, error) {
	releases, err := s.repoStore().List(ctx, repository.ListFilter{})
	if err != nil {
		return 0, err
	}
	runtime := newReleaseTimeoutRuntime()
	processed := 0
	for _, release := range releases {
		if release == nil {
			continue
		}
		handled, err := runtime.process(ctx, s, release, now)
		if err != nil {
			return processed, err
		}
		if handled {
			processed++
		}
	}
	return processed, nil
}

func (r releaseTimeoutRuntime) process(ctx context.Context, svc *releaseService, release *releasedomain.Release, now time.Time) (bool, error) {
	state, ok := compatControlStateFromRelease(release)
	if !ok {
		return false, nil
	}
	policy := compatTimeoutPolicy(state)
	decision := r.scanner.Evaluate(state, policy, now)
	if !decision.StateChanged || decision.NextState == nil {
		return false, nil
	}
	for _, write := range decision.StepWrites {
		if err := svc.applyTimeoutStepWrite(ctx, release.ID, write, now); err != nil {
			return false, err
		}
	}
	if status, ok := compatReleaseStatusFromControlState(*decision.NextState); ok {
		if err := svc.UpdateStatus(ctx, release.ID, status); err != nil {
			return false, err
		}
	}
	return true, nil
}

func compatControlStateFromRelease(release *releasedomain.Release) (releasedomain.ReleaseControlState, bool) {
	if release == nil {
		return releasedomain.ReleaseControlState{}, false
	}
	state := releasedomain.NewInitialControlState(compatReleaseStrategy(release.Strategy), release.UpdatedAt)
	switch release.Status {
	case releasedomain.ReleaseSyncing:
		state = state.WithStatus(releasedomain.LifecycleDispatching, release.UpdatedAt)
	case releasedomain.ReleaseRunning:
		state = state.WithStatus(releasedomain.LifecycleRunning, release.UpdatedAt)
	case releasedomain.ReleaseSucceeded, releasedomain.ReleaseRolledBack:
		state = state.WithStatus(releasedomain.LifecycleSucceeded, release.UpdatedAt)
	case releasedomain.ReleaseFailed, releasedomain.ReleaseSyncFailed:
		state = state.WithStatus(releasedomain.LifecycleFailed, release.UpdatedAt)
	default:
		return releasedomain.ReleaseControlState{}, false
	}
	return state, !state.Terminal
}

func compatTimeoutPolicy(state releasedomain.ReleaseControlState) releasecontrol.TimeoutPolicy {
	switch state.Strategy {
	case releasedomain.ReleaseStrategyRolling:
		return releasecontrol.TimeoutPolicy{
			DispatchTimeout:         2 * time.Minute,
			ProgressTimeout:         10 * time.Minute,
			FinalizeTimeout:         2 * time.Minute,
			ObservationStallTimeout: 5 * time.Minute,
		}
	default:
		return releasecontrol.TimeoutPolicy{}
	}
}

func compatReleaseStrategy(value string) releasedomain.ReleaseStrategy {
	switch strings.TrimSpace(releasedomain.NormalizeReleaseStrategy(value)) {
	case string(releasedomain.ReleaseStrategyCanary):
		return releasedomain.ReleaseStrategyCanary
	case string(releasedomain.ReleaseStrategyBlueGreen):
		return releasedomain.ReleaseStrategyBlueGreen
	default:
		return releasedomain.ReleaseStrategyRolling
	}
}

func compatReleaseStatusFromControlState(state releasedomain.ReleaseControlState) (releasedomain.ReleaseStatus, bool) {
	switch state.LifecycleStatus {
	case releasedomain.LifecycleFailed:
		switch state.FailureReason {
		case releasedomain.FailureDispatchFailed:
			return releasedomain.ReleaseSyncFailed, true
		case releasedomain.FailureRolloutFailed, releasedomain.FailureRolloutTimeout, releasedomain.FailureObservationStalled, releasedomain.FailureFinalizeFailed:
			return releasedomain.ReleaseFailed, true
		default:
			return releasedomain.ReleaseFailed, true
		}
	case releasedomain.LifecycleSucceeded:
		return releasedomain.ReleaseSucceeded, true
	default:
		return "", false
	}
}

func (s *releaseService) processReleaseTimeout(ctx context.Context, releaseID uuid.UUID, now time.Time) (bool, error) {
	release, err := s.Get(ctx, releaseID)
	if err != nil {
		return false, err
	}
	return newReleaseTimeoutRuntime().process(ctx, s, release, now)
}

func (s *releaseService) applyTimeoutStepWrite(ctx context.Context, releaseID uuid.UUID, write releasedomain.StepWrite, now time.Time) error {
	release, err := s.loadRelease(ctx, releaseID)
	if err != nil {
		return err
	}
	if isReleaseTerminalStatus(release.Status) {
		return nil
	}
	nextSteps := cloneReleaseSteps(release.Steps)
	currentStep := findReleaseStep(nextSteps, write.StepCode)
	if currentStep == nil {
		return ErrReleaseUnknownStep
	}
	if currentStep.Status == releasedomain.StepFailed || currentStep.Status == releasedomain.StepSucceeded {
		return nil
	}
	applyReleaseStepUpdate(nextSteps, write.StepCode, write.Status, int32(write.Progress), write.Message, nil, &now)
	release.Steps = nextSteps
	release.UpdatedAt = now
	return s.repoStore().UpdateSteps(ctx, release)
}
