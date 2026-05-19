package reconcile

import (
	"time"

	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
)

func MapNormalizedDeploymentStateToRuntimeObservation(state NormalizedReleaseObservedState, now time.Time) releasedomain.RuntimeObservation {
	observation := releasedomain.RuntimeObservation{
		Message:    state.Message,
		ObservedAt: now,
	}
	switch state.Phase {
	case releasedomain.StepSucceeded:
		observation.Healthy = true
		observation.Terminal = true
	case releasedomain.StepFailed:
		observation.Degraded = true
		observation.Terminal = true
	case releasedomain.StepRunning:
		observation.Progressing = true
	}
	return observation
}
