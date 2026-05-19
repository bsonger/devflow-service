package rolling

import (
	"time"

	releasecontrol "github.com/bsonger/devflow-service/internal/release/control"
	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
)

type Controller struct{}

func NewController() *Controller {
	return &Controller{}
}

func (c *Controller) Strategy() releasedomain.ReleaseStrategy {
	return releasedomain.ReleaseStrategyRolling
}

var _ releasecontrol.StrategyController = (*Controller)(nil)

func (c *Controller) EvaluateObservation(
	state releasedomain.ReleaseControlState,
	obs releasedomain.RuntimeObservation,
) releasedomain.TransitionDecision {
	switch {
	case obs.Healthy && obs.Terminal:
		next := state.WithStatus(releasedomain.LifecycleFinalizing, obs.ObservedAt)
		next = next.WithPhase(releasedomain.PhaseNone, obs.ObservedAt)
		return releasedomain.NewStateTransitionDecision(next, releasedomain.NewStepWrite(
			"observe_rollout",
			releasedomain.StepSucceeded,
			100,
			obs.Message,
		))
	case obs.Degraded && obs.Terminal:
		next := state.WithStatus(releasedomain.LifecycleFinalizing, obs.ObservedAt)
		next = next.WithPhase(releasedomain.PhaseNone, obs.ObservedAt)
		next = next.WithFailure(releasedomain.FailureRolloutFailed, obs.Message, obs.ObservedAt)
		return releasedomain.NewStateTransitionDecision(next, releasedomain.NewStepWrite(
			"observe_rollout",
			releasedomain.StepFailed,
			100,
			obs.Message,
		))
	case obs.Progressing && !obs.Terminal:
		next := state.WithPhase(releasedomain.PhaseRollingProgressing, obs.ObservedAt)
		return releasedomain.NewStateTransitionDecision(next, releasedomain.NewStepWrite(
			"observe_rollout",
			releasedomain.StepRunning,
			25,
			obs.Message,
		))
	default:
		return releasedomain.NoTransitionDecision()
	}
}

func (c *Controller) TimeoutPolicy(_ releasedomain.ReleaseControlState) releasecontrol.TimeoutPolicy {
	return releasecontrol.TimeoutPolicy{
		DispatchTimeout:         2 * time.Minute,
		ProgressTimeout:         10 * time.Minute,
		FinalizeTimeout:         2 * time.Minute,
		ObservationStallTimeout: 5 * time.Minute,
	}
}
