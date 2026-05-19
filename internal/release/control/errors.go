package control

import "errors"

var (
	ErrStrategyControllerNotFound = errors.New("strategy controller not found")
	ErrInvalidLifecycleTransition = errors.New("invalid lifecycle transition")
	ErrTerminalStateMutation      = errors.New("terminal state mutation")
)
