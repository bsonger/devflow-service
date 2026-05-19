package reconcile

import (
	"testing"
	"time"

	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
)

func TestMapNormalizedDeploymentStateToRuntimeObservationHealthy(t *testing.T) {
	now := time.Now()
	obs := MapNormalizedDeploymentStateToRuntimeObservation(NormalizedReleaseObservedState{
		Phase:   releasedomain.StepSucceeded,
		Message: "deployment healthy",
	}, now)
	if !obs.Healthy || !obs.Terminal {
		t.Fatalf("observation = %#v", obs)
	}
}

func TestMapNormalizedDeploymentStateToRuntimeObservationProgressing(t *testing.T) {
	now := time.Now()
	obs := MapNormalizedDeploymentStateToRuntimeObservation(NormalizedReleaseObservedState{
		Phase:   releasedomain.StepRunning,
		Message: "deployment progressing",
	}, now)
	if !obs.Progressing || obs.Terminal {
		t.Fatalf("observation = %#v", obs)
	}
}

func TestMapNormalizedDeploymentStateToRuntimeObservationDegraded(t *testing.T) {
	now := time.Now()
	obs := MapNormalizedDeploymentStateToRuntimeObservation(NormalizedReleaseObservedState{
		Phase:   releasedomain.StepFailed,
		Message: "deployment failed",
	}, now)
	if !obs.Degraded || !obs.Terminal {
		t.Fatalf("observation = %#v", obs)
	}
}
