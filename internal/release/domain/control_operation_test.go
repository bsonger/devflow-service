package domain

import (
	"strings"
	"testing"
	"time"
)

func TestEvaluateOperationRequestAllowsPauseDuringRunning(t *testing.T) {
	state := ReleaseControlState{
		LifecycleStatus: LifecycleRunning,
		Strategy:        ReleaseStrategyRolling,
		UpdatedAt:       time.Now(),
	}

	decision := EvaluateOperationRequest(state, ReleaseOperationPause)
	if !decision.Allowed {
		t.Fatalf("pause should be allowed: %+v", decision)
	}
	if decision.TargetStatus != LifecyclePaused {
		t.Fatalf("pause target = %q, want %q", decision.TargetStatus, LifecyclePaused)
	}
}

func TestEvaluateOperationRequestRejectsPauseOutsideRunning(t *testing.T) {
	state := ReleaseControlState{
		LifecycleStatus: LifecycleDispatching,
		Strategy:        ReleaseStrategyRolling,
		UpdatedAt:       time.Now(),
	}

	decision := EvaluateOperationRequest(state, ReleaseOperationPause)
	if decision.Allowed {
		t.Fatalf("pause should be rejected: %+v", decision)
	}
	if !strings.Contains(strings.ToLower(decision.Reason), "running") {
		t.Fatalf("unexpected reason: %q", decision.Reason)
	}
}

func TestEvaluateOperationRequestAllowsCancelBeforeTerminal(t *testing.T) {
	tests := []LifecycleStatus{
		LifecyclePending,
		LifecycleDispatching,
		LifecycleRunning,
		LifecycleFinalizing,
	}

	for _, status := range tests {
		state := ReleaseControlState{
			LifecycleStatus: status,
			Strategy:        ReleaseStrategyRolling,
			UpdatedAt:       time.Now(),
		}
	decision := EvaluateOperationRequest(state, ReleaseOperationCancel)
	if !decision.Allowed {
		t.Fatalf("cancel should be allowed for %q: %+v", status, decision)
	}
		if decision.TargetStatus != LifecycleFailed {
			t.Fatalf("cancel target for %q = %q, want %q", status, decision.TargetStatus, LifecycleFailed)
		}
	}
}

func TestEvaluateOperationRequestRejectsCancelAfterTerminal(t *testing.T) {
	state := ReleaseControlState{
		LifecycleStatus: LifecycleSucceeded,
		Strategy:        ReleaseStrategyRolling,
		Terminal:        true,
		UpdatedAt:       time.Now(),
	}

	decision := EvaluateOperationRequest(state, ReleaseOperationCancel)
	if decision.Allowed {
		t.Fatalf("cancel should be rejected: %+v", decision)
	}
}

func TestEvaluateOperationRequestAllowsRollbackOnlyAfterFailedOrSucceeded(t *testing.T) {
	allowed := []LifecycleStatus{LifecycleSucceeded, LifecycleFailed}
	for _, status := range allowed {
		state := ReleaseControlState{
			LifecycleStatus: status,
			Strategy:        ReleaseStrategyRolling,
			Terminal:        true,
			UpdatedAt:       time.Now(),
		}
		decision := EvaluateOperationRequest(state, ReleaseOperationRollback)
		if !decision.Allowed {
			t.Fatalf("rollback should be allowed for %q: %+v", status, decision)
		}
		if decision.TargetStatus != LifecyclePending {
			t.Fatalf("rollback target for %q = %q, want %q", status, decision.TargetStatus, LifecyclePending)
		}
	}

	rejected := ReleaseControlState{
		LifecycleStatus: LifecycleRunning,
		Strategy:        ReleaseStrategyRolling,
		UpdatedAt:       time.Now(),
	}
	decision := EvaluateOperationRequest(rejected, ReleaseOperationRollback)
	if decision.Allowed {
		t.Fatalf("rollback should be rejected while running: %+v", decision)
	}
}

func TestEvaluateOperationRequestRejectsResumeWithoutPausedPhase(t *testing.T) {
	state := ReleaseControlState{
		LifecycleStatus: LifecycleRunning,
		Strategy:        ReleaseStrategyRolling,
		UpdatedAt:       time.Now(),
	}

	decision := EvaluateOperationRequest(state, ReleaseOperationResume)
	if decision.Allowed {
		t.Fatalf("resume should be rejected without paused phase: %+v", decision)
	}
}

func TestEvaluateOperationRequestAllowsResumeWhenPaused(t *testing.T) {
	state := ReleaseControlState{
		LifecycleStatus: LifecyclePaused,
		Strategy:        ReleaseStrategyRolling,
		UpdatedAt:       time.Now(),
	}

	decision := EvaluateOperationRequest(state, ReleaseOperationResume)
	if !decision.Allowed {
		t.Fatalf("resume should be allowed while paused: %+v", decision)
	}
	if decision.TargetStatus != LifecycleRunning {
		t.Fatalf("resume target = %q, want %q", decision.TargetStatus, LifecycleRunning)
	}
}
