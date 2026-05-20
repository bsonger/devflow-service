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
	switch state.Strategy {
	case ReleaseStrategyRolling:
		return assessRollingRemediationNeed(state, failedStep, operation)
	default:
		return RemediationDecision{
			Kind:   RemediationNotRequired,
			Reason: "remediation defaults to not required for unsupported strategy",
		}
	}
}

func assessRollingRemediationNeed(state ReleaseControlState, failedStep string, operation ReleaseOperation) RemediationDecision {
	step := strings.TrimSpace(failedStep)
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

	switch state.LifecycleStatus {
	case LifecycleRunning, LifecyclePaused, LifecycleFinalizing:
		if normalizeReleaseOperation(operation) == ReleaseOperationCancel {
			return RemediationDecision{
				Kind:   RemediationPendingRollback,
				Reason: "canceled release may have active runtime changes",
			}
		}
	}

	return RemediationDecision{
		Kind:   RemediationNotRequired,
		Reason: "no rollback remediation required for current release state",
	}
}
