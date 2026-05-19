package domain

import "strings"

type LifecycleStatus string

const (
	LifecyclePending     LifecycleStatus = "Pending"
	LifecycleDispatching LifecycleStatus = "Dispatching"
	LifecycleRunning     LifecycleStatus = "Running"
	LifecycleFinalizing  LifecycleStatus = "Finalizing"
	LifecycleSucceeded   LifecycleStatus = "Succeeded"
	LifecycleFailed      LifecycleStatus = "Failed"
)

func (s LifecycleStatus) IsTerminal() bool {
	switch s {
	case LifecycleSucceeded, LifecycleFailed:
		return true
	default:
		return false
	}
}

func (s LifecycleStatus) IsZero() bool {
	return strings.TrimSpace(string(s)) == ""
}
