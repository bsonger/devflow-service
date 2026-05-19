package domain

import "time"

type ReleaseControlState struct {
	LifecycleStatus LifecycleStatus `json:"lifecycle_status"`
	Strategy        ReleaseStrategy `json:"strategy"`
	StrategyPhase   StrategyPhase   `json:"strategy_phase"`
	FailureReason   FailureReason   `json:"failure_reason,omitempty"`
	FailureMessage  string          `json:"failure_message,omitempty"`
	Terminal        bool            `json:"terminal"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type RuntimeObservation struct {
	Healthy     bool      `json:"healthy"`
	Progressing bool      `json:"progressing"`
	Degraded    bool      `json:"degraded"`
	Terminal    bool      `json:"terminal"`
	Message     string    `json:"message,omitempty"`
	ObservedAt  time.Time `json:"observed_at"`
}

func NewInitialControlState(strategy ReleaseStrategy, now time.Time) ReleaseControlState {
	return ReleaseControlState{
		LifecycleStatus: LifecyclePending,
		Strategy:        strategy,
		StrategyPhase:   PhaseNone,
		Terminal:        false,
		UpdatedAt:       now,
	}
}

func (s ReleaseControlState) WithStatus(next LifecycleStatus, now time.Time) ReleaseControlState {
	s.LifecycleStatus = next
	s.Terminal = next.IsTerminal()
	s.UpdatedAt = now
	return s
}

func (s ReleaseControlState) WithPhase(next StrategyPhase, now time.Time) ReleaseControlState {
	s.StrategyPhase = next
	s.UpdatedAt = now
	return s
}

func (s ReleaseControlState) WithFailure(reason FailureReason, message string, now time.Time) ReleaseControlState {
	s.FailureReason = reason
	s.FailureMessage = message
	s.UpdatedAt = now
	return s
}
