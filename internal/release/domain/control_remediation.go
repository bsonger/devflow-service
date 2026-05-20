package domain

import "strings"

type RemediationKind string

const (
	RemediationNone            RemediationKind = ""
	RemediationNotRequired     RemediationKind = "NotRequired"
	RemediationCleanupOnly     RemediationKind = "CleanupOnly"
	RemediationPendingRollback RemediationKind = "PendingRollback"
	RemediationRollingBack     RemediationKind = "RollingBack"
	RemediationRollbackFailed  RemediationKind = "RollbackFailed"
	RemediationRollbackDone    RemediationKind = "RollbackSucceeded"
)

type RemediationDecision struct {
	Kind   RemediationKind `json:"kind"`
	Reason string          `json:"reason,omitempty"`
}

func AssessRemediationNeed(state ReleaseControlState, failedStep string, operation ReleaseOperation) RemediationDecision {
	step := strings.TrimSpace(failedStep)
	if decision, ok := assessSharedRuntimeRemediationNeed(state, step, operation); ok {
		return decision
	}
	switch state.Strategy {
	case ReleaseStrategyRolling:
		return assessRollingRemediationNeed(state, step, operation)
	default:
		return RemediationDecision{
			Kind:   RemediationNotRequired,
			Reason: "release did not reach runtime activation",
		}
	}
}

func assessSharedRuntimeRemediationNeed(state ReleaseControlState, step string, operation ReleaseOperation) (RemediationDecision, bool) {
	switch step {
	case "observe_rollout", "finalize_release", "observe_preview", "switch_traffic", "verify_active", "deploy_canary", "canary_10", "canary_30", "canary_60", "canary_100":
		return RemediationDecision{
			Kind:   RemediationPendingRollback,
			Reason: "release may have partially or fully affected live runtime state",
		}, true
	}

	switch state.LifecycleStatus {
	case LifecycleRunning, LifecyclePaused, LifecycleFinalizing:
		if normalizeReleaseOperation(operation) == ReleaseOperationCancel {
			return RemediationDecision{
				Kind:   RemediationPendingRollback,
				Reason: "canceled release may have active runtime changes",
			}, true
		}
	}

	return RemediationDecision{}, false
}

func assessRollingRemediationNeed(state ReleaseControlState, step string, operation ReleaseOperation) RemediationDecision {
	switch step {
	case "freeze_inputs", "ensure_namespace", "ensure_pull_secret", "ensure_appproject_destination", "render_deployment_bundle", "publish_bundle":
		return RemediationDecision{
			Kind:   RemediationNotRequired,
			Reason: "release did not reach runtime activation",
		}
	case "create_argocd_application", "start_deployment":
		return RemediationDecision{
			Kind:   RemediationCleanupOnly,
			Reason: "release reached handoff preparation but did not require runtime rollback",
		}
	case "observe_rollout", "finalize_release":
		return RemediationDecision{
			Kind:   RemediationPendingRollback,
			Reason: "release may have partially or fully affected live runtime state",
		}
	}

	return RemediationDecision{
		Kind:   RemediationNotRequired,
		Reason: "no rollback remediation required for current release state",
	}
}
