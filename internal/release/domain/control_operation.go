package domain

import "strings"

type ReleaseOperation string

const (
	ReleaseOperationPause    ReleaseOperation = "Pause"
	ReleaseOperationResume   ReleaseOperation = "Resume"
	ReleaseOperationCancel   ReleaseOperation = "Cancel"
	ReleaseOperationRollback ReleaseOperation = "Rollback"
)

type OperationRequestDecision struct {
	Allowed      bool            `json:"allowed"`
	TargetStatus LifecycleStatus `json:"target_status,omitempty"`
	Reason       string          `json:"reason,omitempty"`
}

func EvaluateOperationRequest(state ReleaseControlState, operation ReleaseOperation) OperationRequestDecision {
	switch normalizeReleaseOperation(operation) {
	case ReleaseOperationPause:
		if state.LifecycleStatus != LifecycleRunning {
			return rejectOperation("pause is only allowed while release is running")
		}
		return allowOperation(LifecyclePaused)
	case ReleaseOperationResume:
		if state.LifecycleStatus != LifecyclePaused {
			return rejectOperation("resume is only allowed while release is paused")
		}
		return allowOperation(LifecycleRunning)
	case ReleaseOperationCancel:
		if state.Terminal || state.LifecycleStatus.IsTerminal() {
			return rejectOperation("cancel is not allowed after release reaches terminal state")
		}
		return allowOperation(LifecycleFailed)
	case ReleaseOperationRollback:
		switch state.LifecycleStatus {
		case LifecycleSucceeded, LifecycleFailed:
			return allowOperation(LifecyclePending)
		default:
			return rejectOperation("rollback is only allowed after release succeeds or fails")
		}
	default:
		return rejectOperation("unsupported release operation")
	}
}

func normalizeReleaseOperation(operation ReleaseOperation) ReleaseOperation {
	switch strings.ToLower(strings.TrimSpace(string(operation))) {
	case "pause":
		return ReleaseOperationPause
	case "resume":
		return ReleaseOperationResume
	case "cancel":
		return ReleaseOperationCancel
	case "rollback":
		return ReleaseOperationRollback
	default:
		return ReleaseOperation(strings.TrimSpace(string(operation)))
	}
}

func allowOperation(target LifecycleStatus) OperationRequestDecision {
	return OperationRequestDecision{
		Allowed:      true,
		TargetStatus: target,
	}
}

func rejectOperation(reason string) OperationRequestDecision {
	return OperationRequestDecision{
		Allowed: false,
		Reason:  strings.TrimSpace(reason),
	}
}
