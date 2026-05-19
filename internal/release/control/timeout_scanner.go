package control

import (
	"time"

	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
)

type TimeoutScanner struct {
	evaluator TimeoutEvaluator
}

func NewTimeoutScanner() TimeoutScanner {
	return TimeoutScanner{evaluator: TimeoutEvaluator{}}
}

func (s TimeoutScanner) Evaluate(
	state releasedomain.ReleaseControlState,
	policy TimeoutPolicy,
	now time.Time,
) releasedomain.TransitionDecision {
	return s.evaluator.Evaluate(state, nil, policy, now)
}
