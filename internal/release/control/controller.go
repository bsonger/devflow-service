package control

import releasedomain "github.com/bsonger/devflow-service/internal/release/domain"

type StrategyController interface {
	Strategy() releasedomain.ReleaseStrategy
	EvaluateObservation(
		state releasedomain.ReleaseControlState,
		obs releasedomain.RuntimeObservation,
	) releasedomain.TransitionDecision
	TimeoutPolicy(state releasedomain.ReleaseControlState) TimeoutPolicy
}
