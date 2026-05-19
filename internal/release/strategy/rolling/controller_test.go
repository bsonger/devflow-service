package rolling

import (
	"testing"
	"time"

	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
)

func TestEvaluateObservationProgressingKeepsRunningPhase(t *testing.T) {
	now := time.Now()
	controller := NewController()
	state := releasedomain.ReleaseControlState{
		LifecycleStatus: releasedomain.LifecycleRunning,
		Strategy:        releasedomain.ReleaseStrategyRolling,
		UpdatedAt:       now,
	}
	decision := controller.EvaluateObservation(state, releasedomain.RuntimeObservation{
		Progressing: true,
		Message:     "deployment progressing",
		ObservedAt:  now,
	})
	if decision.NextState == nil {
		t.Fatal("expected next state")
	}
	if decision.NextState.LifecycleStatus != releasedomain.LifecycleRunning {
		t.Fatalf("lifecycle = %q", decision.NextState.LifecycleStatus)
	}
	if decision.NextState.StrategyPhase != releasedomain.PhaseRollingProgressing {
		t.Fatalf("strategy phase = %q", decision.NextState.StrategyPhase)
	}
	if len(decision.StepWrites) != 1 || decision.StepWrites[0].Status != releasedomain.StepRunning {
		t.Fatalf("step writes = %#v", decision.StepWrites)
	}
}

func TestEvaluateObservationHealthyTerminalMovesToFinalizing(t *testing.T) {
	now := time.Now()
	controller := NewController()
	state := releasedomain.ReleaseControlState{
		LifecycleStatus: releasedomain.LifecycleRunning,
		Strategy:        releasedomain.ReleaseStrategyRolling,
		UpdatedAt:       now,
	}
	decision := controller.EvaluateObservation(state, releasedomain.RuntimeObservation{
		Healthy:    true,
		Terminal:   true,
		Message:    "deployment healthy",
		ObservedAt: now,
	})
	if decision.NextState == nil || decision.NextState.LifecycleStatus != releasedomain.LifecycleFinalizing {
		t.Fatalf("decision = %#v", decision)
	}
	if len(decision.StepWrites) != 1 || decision.StepWrites[0].Status != releasedomain.StepSucceeded {
		t.Fatalf("step writes = %#v", decision.StepWrites)
	}
}

func TestEvaluateObservationDegradedTerminalSetsFailureReason(t *testing.T) {
	now := time.Now()
	controller := NewController()
	state := releasedomain.ReleaseControlState{
		LifecycleStatus: releasedomain.LifecycleRunning,
		Strategy:        releasedomain.ReleaseStrategyRolling,
		UpdatedAt:       now,
	}
	decision := controller.EvaluateObservation(state, releasedomain.RuntimeObservation{
		Degraded:   true,
		Terminal:   true,
		Message:    "deployment failed",
		ObservedAt: now,
	})
	if decision.NextState == nil {
		t.Fatal("expected next state")
	}
	if decision.NextState.FailureReason != releasedomain.FailureRolloutFailed {
		t.Fatalf("failure reason = %q", decision.NextState.FailureReason)
	}
	if len(decision.StepWrites) != 1 || decision.StepWrites[0].Status != releasedomain.StepFailed {
		t.Fatalf("step writes = %#v", decision.StepWrites)
	}
}

func TestEvaluateObservationUnknownReturnsNoTransition(t *testing.T) {
	controller := NewController()
	decision := controller.EvaluateObservation(
		releasedomain.ReleaseControlState{Strategy: releasedomain.ReleaseStrategyRolling},
		releasedomain.RuntimeObservation{},
	)
	if decision.NextState != nil || len(decision.StepWrites) != 0 || decision.StateChanged {
		t.Fatalf("expected no transition, got %#v", decision)
	}
}
