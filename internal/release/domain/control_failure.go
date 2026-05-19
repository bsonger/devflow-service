package domain

import "strings"

type FailureReason string

const (
	FailureNone               FailureReason = ""
	FailureDispatchFailed     FailureReason = "DispatchFailed"
	FailureRolloutFailed      FailureReason = "RolloutFailed"
	FailureRolloutTimeout     FailureReason = "RolloutTimeout"
	FailureFinalizeFailed     FailureReason = "FinalizeFailed"
	FailureObservationStalled FailureReason = "ObservationStalled"
)

func (r FailureReason) IsZero() bool {
	return strings.TrimSpace(string(r)) == ""
}
