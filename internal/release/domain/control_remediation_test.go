package domain

import (
	"testing"
	"time"
)

func TestAssessRemediationNeedRollingReturnsNotRequiredBeforeRuntimeActivation(t *testing.T) {
	state := ReleaseControlState{
		LifecycleStatus: LifecycleDispatching,
		Strategy:        ReleaseStrategyRolling,
		UpdatedAt:       time.Now(),
	}
	decision := AssessRemediationNeed(state, "publish_bundle", ReleaseOperationCancel)
	if decision.Kind != RemediationNotRequired {
		t.Fatalf("kind = %q, want %q", decision.Kind, RemediationNotRequired)
	}
}

func TestAssessRemediationNeedRollingReturnsCleanupOnlyDuringHandoff(t *testing.T) {
	state := ReleaseControlState{
		LifecycleStatus: LifecycleDispatching,
		Strategy:        ReleaseStrategyRolling,
		UpdatedAt:       time.Now(),
	}
	decision := AssessRemediationNeed(state, "create_argocd_application", ReleaseOperationCancel)
	if decision.Kind != RemediationCleanupOnly {
		t.Fatalf("kind = %q, want %q", decision.Kind, RemediationCleanupOnly)
	}

	decision = AssessRemediationNeed(state, "start_deployment", ReleaseOperationCancel)
	if decision.Kind != RemediationCleanupOnly {
		t.Fatalf("kind = %q, want %q", decision.Kind, RemediationCleanupOnly)
	}
}

func TestAssessRemediationNeedRollingReturnsRollbackAfterRuntimeActivation(t *testing.T) {
	state := ReleaseControlState{
		LifecycleStatus: LifecycleRunning,
		Strategy:        ReleaseStrategyRolling,
		UpdatedAt:       time.Now(),
	}
	decision := AssessRemediationNeed(state, "observe_rollout", ReleaseOperationCancel)
	if decision.Kind != RemediationPendingRollback {
		t.Fatalf("kind = %q, want %q", decision.Kind, RemediationPendingRollback)
	}

	finalizing := ReleaseControlState{
		LifecycleStatus: LifecycleFinalizing,
		Strategy:        ReleaseStrategyRolling,
		UpdatedAt:       time.Now(),
	}
	decision = AssessRemediationNeed(finalizing, "finalize_release", ReleaseOperationCancel)
	if decision.Kind != RemediationPendingRollback {
		t.Fatalf("kind = %q, want %q", decision.Kind, RemediationPendingRollback)
	}
}

func TestAssessRemediationNeedRollingReturnsRollbackForPausedReleaseCancel(t *testing.T) {
	state := ReleaseControlState{
		LifecycleStatus: LifecyclePaused,
		Strategy:        ReleaseStrategyRolling,
		UpdatedAt:       time.Now(),
	}
	decision := AssessRemediationNeed(state, "observe_rollout", ReleaseOperationCancel)
	if decision.Kind != RemediationPendingRollback {
		t.Fatalf("kind = %q, want %q", decision.Kind, RemediationPendingRollback)
	}
}

func TestAssessRemediationNeedCanaryAndBlueGreenReturnRollbackAfterRuntimeActivation(t *testing.T) {
	canary := ReleaseControlState{
		LifecycleStatus: LifecycleRunning,
		Strategy:        ReleaseStrategyCanary,
		UpdatedAt:       time.Now(),
	}
	decision := AssessRemediationNeed(canary, "canary_30", ReleaseOperationCancel)
	if decision.Kind != RemediationPendingRollback {
		t.Fatalf("canary kind = %q, want %q", decision.Kind, RemediationPendingRollback)
	}

	blueGreen := ReleaseControlState{
		LifecycleStatus: LifecycleRunning,
		Strategy:        ReleaseStrategyBlueGreen,
		UpdatedAt:       time.Now(),
	}
	decision = AssessRemediationNeed(blueGreen, "switch_traffic", ReleaseOperationCancel)
	if decision.Kind != RemediationPendingRollback {
		t.Fatalf("blue-green kind = %q, want %q", decision.Kind, RemediationPendingRollback)
	}
}
