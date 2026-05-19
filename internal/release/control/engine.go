package control

import (
	"fmt"

	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
)

type Engine struct {
	strategies map[releasedomain.ReleaseStrategy]StrategyController
}

func NewEngine(controllers ...StrategyController) *Engine {
	strategies := make(map[releasedomain.ReleaseStrategy]StrategyController, len(controllers))
	for _, controller := range controllers {
		if controller == nil {
			continue
		}
		strategies[controller.Strategy()] = controller
	}
	return &Engine{strategies: strategies}
}

func (e *Engine) ApplyObservation(
	state releasedomain.ReleaseControlState,
	obs releasedomain.RuntimeObservation,
) (releasedomain.TransitionDecision, error) {
	controller, err := e.controllerFor(state.Strategy)
	if err != nil {
		return releasedomain.TransitionDecision{}, err
	}
	decision := controller.EvaluateObservation(state, obs)
	if decision.NextState == nil {
		return decision, nil
	}
	if err := releasedomain.ValidateNonTerminalMutation(state); err != nil {
		return releasedomain.TransitionDecision{}, fmt.Errorf("%w: %v", ErrTerminalStateMutation, err)
	}
	if err := releasedomain.ValidateLifecycleTransition(state.LifecycleStatus, decision.NextState.LifecycleStatus); err != nil {
		return releasedomain.TransitionDecision{}, fmt.Errorf("%w: %v", ErrInvalidLifecycleTransition, err)
	}
	return decision, nil
}

func (e *Engine) controllerFor(strategy releasedomain.ReleaseStrategy) (StrategyController, error) {
	controller, ok := e.strategies[strategy]
	if !ok || controller == nil {
		return nil, fmt.Errorf("%w: %s", ErrStrategyControllerNotFound, strategy)
	}
	return controller, nil
}
