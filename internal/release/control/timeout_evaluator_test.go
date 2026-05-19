package control

import (
	"testing"
	"time"

	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
)

func TestTimeoutEvaluatorFailsDispatchingReleaseAfterDispatchTimeout(t *testing.T) {
	now := time.Now()
	evaluator := TimeoutEvaluator{}
	state := releasedomain.ReleaseControlState{
		LifecycleStatus: releasedomain.LifecycleDispatching,
		Strategy:        releasedomain.ReleaseStrategyRolling,
		UpdatedAt:       now.Add(-3 * time.Minute),
	}

	decision := evaluator.Evaluate(state, nil, TimeoutPolicy{
		DispatchTimeout: time.Minute,
	}, now)
	if decision.NextState == nil || decision.NextState.FailureReason != releasedomain.FailureDispatchFailed {
		t.Fatalf("decision = %#v", decision)
	}
}

func TestTimeoutEvaluatorFailsRunningReleaseAfterProgressTimeout(t *testing.T) {
	now := time.Now()
	evaluator := TimeoutEvaluator{}
	state := releasedomain.ReleaseControlState{
		LifecycleStatus: releasedomain.LifecycleRunning,
		Strategy:        releasedomain.ReleaseStrategyRolling,
		UpdatedAt:       now.Add(-11 * time.Minute),
	}

	decision := evaluator.Evaluate(state, nil, TimeoutPolicy{
		ProgressTimeout: 10 * time.Minute,
	}, now)
	if decision.NextState == nil || decision.NextState.FailureReason != releasedomain.FailureRolloutTimeout {
		t.Fatalf("decision = %#v", decision)
	}
}

func TestTimeoutEvaluatorFailsRunningReleaseWhenObservationStalls(t *testing.T) {
	now := time.Now()
	evaluator := TimeoutEvaluator{}
	state := releasedomain.ReleaseControlState{
		LifecycleStatus: releasedomain.LifecycleRunning,
		Strategy:        releasedomain.ReleaseStrategyRolling,
		UpdatedAt:       now.Add(-time.Minute),
	}
	lastObservation := &releasedomain.RuntimeObservation{
		ObservedAt: now.Add(-6 * time.Minute),
	}

	decision := evaluator.Evaluate(state, lastObservation, TimeoutPolicy{
		ObservationStallTimeout: 5 * time.Minute,
	}, now)
	if decision.NextState == nil || decision.NextState.FailureReason != releasedomain.FailureObservationStalled {
		t.Fatalf("decision = %#v", decision)
	}
}

func TestTimeoutEvaluatorFailsFinalizingReleaseAfterFinalizeTimeout(t *testing.T) {
	now := time.Now()
	evaluator := TimeoutEvaluator{}
	state := releasedomain.ReleaseControlState{
		LifecycleStatus: releasedomain.LifecycleFinalizing,
		Strategy:        releasedomain.ReleaseStrategyRolling,
		UpdatedAt:       now.Add(-3 * time.Minute),
	}

	decision := evaluator.Evaluate(state, nil, TimeoutPolicy{
		FinalizeTimeout: 2 * time.Minute,
	}, now)
	if decision.NextState == nil || decision.NextState.FailureReason != releasedomain.FailureFinalizeFailed {
		t.Fatalf("decision = %#v", decision)
	}
}

func TestTimeoutEvaluatorReturnsNoTransitionWithinThreshold(t *testing.T) {
	now := time.Now()
	evaluator := TimeoutEvaluator{}
	state := releasedomain.ReleaseControlState{
		LifecycleStatus: releasedomain.LifecycleRunning,
		Strategy:        releasedomain.ReleaseStrategyRolling,
		UpdatedAt:       now.Add(-time.Minute),
	}

	decision := evaluator.Evaluate(state, nil, TimeoutPolicy{
		ProgressTimeout: 10 * time.Minute,
	}, now)
	if decision.NextState != nil || decision.StateChanged {
		t.Fatalf("expected no decision, got %#v", decision)
	}
}
