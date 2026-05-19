package domain

import "strings"

type StrategyPhase string

const (
	PhaseNone               StrategyPhase = ""
	PhaseRollingProgressing StrategyPhase = "RollingProgressing"
)

func (p StrategyPhase) IsZero() bool {
	return strings.TrimSpace(string(p)) == ""
}
