package control

import (
	"errors"
	"strings"
	"testing"
	"time"

	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
)

type stubStrategyController struct {
	strategy releasedomain.ReleaseStrategy
	evalFn   func(releasedomain.ReleaseControlState, releasedomain.RuntimeObservation) releasedomain.TransitionDecision
}

func (s stubStrategyController) Strategy() releasedomain.ReleaseStrategy { return s.strategy }

func (s stubStrategyController) EvaluateObservation(
	state releasedomain.ReleaseControlState,
	obs releasedomain.RuntimeObservation,
) releasedomain.TransitionDecision {
	if s.evalFn == nil {
		return releasedomain.NoTransitionDecision()
	}
	return s.evalFn(state, obs)
}

func (s stubStrategyController) TimeoutPolicy(releasedomain.ReleaseControlState) TimeoutPolicy {
	return TimeoutPolicy{}
}

func TestApplyObservationFailsWhenStrategyControllerMissing(t *testing.T) {
	engine := NewEngine()
	_, err := engine.ApplyObservation(
		releasedomain.NewInitialControlState(releasedomain.ReleaseStrategyRolling, time.Now()),
		releasedomain.RuntimeObservation{},
	)
	if !errors.Is(err, ErrStrategyControllerNotFound) {
		t.Fatalf("expected ErrStrategyControllerNotFound, got %v", err)
	}
}

func TestApplyObservationAcceptsValidTransition(t *testing.T) {
	now := time.Now()
	engine := NewEngine(stubStrategyController{
		strategy: releasedomain.ReleaseStrategyRolling,
		evalFn: func(state releasedomain.ReleaseControlState, _ releasedomain.RuntimeObservation) releasedomain.TransitionDecision {
			next := state.WithStatus(releasedomain.LifecycleFinalizing, now)
			return releasedomain.NewStateTransitionDecision(next)
		},
	})
	state := releasedomain.ReleaseControlState{
		LifecycleStatus: releasedomain.LifecycleRunning,
		Strategy:        releasedomain.ReleaseStrategyRolling,
		UpdatedAt:       now,
	}

	decision, err := engine.ApplyObservation(state, releasedomain.RuntimeObservation{})
	if err != nil {
		t.Fatalf("ApplyObservation() error = %v", err)
	}
	if decision.NextState == nil || decision.NextState.LifecycleStatus != releasedomain.LifecycleFinalizing {
		t.Fatalf("decision = %#v", decision)
	}
}

func TestApplyObservationRejectsInvalidTransition(t *testing.T) {
	now := time.Now()
	engine := NewEngine(stubStrategyController{
		strategy: releasedomain.ReleaseStrategyRolling,
		evalFn: func(state releasedomain.ReleaseControlState, _ releasedomain.RuntimeObservation) releasedomain.TransitionDecision {
			next := state.WithStatus(releasedomain.LifecycleSucceeded, now)
			return releasedomain.NewStateTransitionDecision(next)
		},
	})
	state := releasedomain.ReleaseControlState{
		LifecycleStatus: releasedomain.LifecycleRunning,
		Strategy:        releasedomain.ReleaseStrategyRolling,
		UpdatedAt:       now,
	}

	_, err := engine.ApplyObservation(state, releasedomain.RuntimeObservation{})
	if !errors.Is(err, ErrInvalidLifecycleTransition) {
		t.Fatalf("expected ErrInvalidLifecycleTransition, got %v", err)
	}
}

func TestApplyObservationRejectsTerminalStateMutation(t *testing.T) {
	now := time.Now()
	engine := NewEngine(stubStrategyController{
		strategy: releasedomain.ReleaseStrategyRolling,
		evalFn: func(state releasedomain.ReleaseControlState, _ releasedomain.RuntimeObservation) releasedomain.TransitionDecision {
			next := state.WithStatus(releasedomain.LifecycleSucceeded, now)
			return releasedomain.NewStateTransitionDecision(next)
		},
	})
	state := releasedomain.ReleaseControlState{
		LifecycleStatus: releasedomain.LifecycleSucceeded,
		Strategy:        releasedomain.ReleaseStrategyRolling,
		Terminal:        true,
		UpdatedAt:       now,
	}

	_, err := engine.ApplyObservation(state, releasedomain.RuntimeObservation{})
	if !errors.Is(err, ErrTerminalStateMutation) {
		t.Fatalf("expected ErrTerminalStateMutation, got %v", err)
	}
	if !strings.Contains(err.Error(), string(releasedomain.LifecycleSucceeded)) {
		t.Fatalf("expected terminal status in error, got %v", err)
	}
}
