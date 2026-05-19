package control

import "time"

type TimeoutPolicy struct {
	DispatchTimeout         time.Duration
	ProgressTimeout         time.Duration
	FinalizeTimeout         time.Duration
	ObservationStallTimeout time.Duration
}
