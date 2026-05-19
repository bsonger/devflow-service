package control

import (
	"time"

	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
)

type TimeoutEvaluator struct{}

func (e TimeoutEvaluator) Evaluate(
	state releasedomain.ReleaseControlState,
	lastObservation *releasedomain.RuntimeObservation,
	policy TimeoutPolicy,
	now time.Time,
) releasedomain.TransitionDecision {
	switch state.LifecycleStatus {
	case releasedomain.LifecycleDispatching:
		if policy.DispatchTimeout > 0 && now.Sub(state.UpdatedAt) > policy.DispatchTimeout {
			next := state.WithStatus(releasedomain.LifecycleFailed, now)
			next = next.WithFailure(releasedomain.FailureDispatchFailed, "release dispatch timed out", now)
			return releasedomain.NewStateTransitionDecision(next, releasedomain.NewStepWrite(
				"start_deployment",
				releasedomain.StepFailed,
				100,
				"release dispatch timed out",
			))
		}
	case releasedomain.LifecycleRunning:
		if policy.ProgressTimeout > 0 && now.Sub(state.UpdatedAt) > policy.ProgressTimeout {
			next := state.WithStatus(releasedomain.LifecycleFailed, now)
			next = next.WithFailure(releasedomain.FailureRolloutTimeout, "release rollout timed out", now)
			return releasedomain.NewStateTransitionDecision(next, releasedomain.NewStepWrite(
				"observe_rollout",
				releasedomain.StepFailed,
				100,
				"release rollout timed out",
			))
		}
		if lastObservation != nil && policy.ObservationStallTimeout > 0 && now.Sub(lastObservation.ObservedAt) > policy.ObservationStallTimeout {
			next := state.WithStatus(releasedomain.LifecycleFailed, now)
			next = next.WithFailure(releasedomain.FailureObservationStalled, "release observation stalled", now)
			return releasedomain.NewStateTransitionDecision(next, releasedomain.NewStepWrite(
				"observe_rollout",
				releasedomain.StepFailed,
				100,
				"release observation stalled",
			))
		}
	case releasedomain.LifecycleFinalizing:
		if policy.FinalizeTimeout > 0 && now.Sub(state.UpdatedAt) > policy.FinalizeTimeout {
			next := state.WithStatus(releasedomain.LifecycleFailed, now)
			next = next.WithFailure(releasedomain.FailureFinalizeFailed, "release finalization timed out", now)
			return releasedomain.NewStateTransitionDecision(next, releasedomain.NewStepWrite(
				"finalize_release",
				releasedomain.StepFailed,
				100,
				"release finalization timed out",
			))
		}
	}
	return releasedomain.NoTransitionDecision()
}
