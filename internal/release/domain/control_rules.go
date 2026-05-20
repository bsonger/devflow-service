package domain

import "fmt"

var allowedLifecycleTransitions = map[LifecycleStatus]map[LifecycleStatus]struct{}{
	LifecyclePending: {
		LifecycleDispatching: {},
	},
	LifecycleDispatching: {
		LifecycleRunning: {},
		LifecycleFailed:  {},
	},
	LifecycleRunning: {
		LifecyclePaused:     {},
		LifecycleFinalizing: {},
	},
	LifecyclePaused: {
		LifecycleRunning:    {},
		LifecycleFailed:     {},
		LifecycleFinalizing: {},
	},
	LifecycleFinalizing: {
		LifecycleSucceeded: {},
		LifecycleFailed:    {},
	},
}

func CanTransition(from, to LifecycleStatus) bool {
	if from == to {
		return true
	}
	next, ok := allowedLifecycleTransitions[from]
	if !ok {
		return false
	}
	_, ok = next[to]
	return ok
}

func ValidateLifecycleTransition(from, to LifecycleStatus) error {
	if CanTransition(from, to) {
		return nil
	}
	return fmt.Errorf("invalid lifecycle transition from %q to %q", from, to)
}

func ValidateNonTerminalMutation(state ReleaseControlState) error {
	if state.Terminal || state.LifecycleStatus.IsTerminal() {
		return fmt.Errorf("terminal lifecycle status %q cannot be mutated", state.LifecycleStatus)
	}
	return nil
}
