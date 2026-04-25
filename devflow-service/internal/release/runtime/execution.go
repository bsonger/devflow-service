package runtime

import (
	"strings"
	"sync/atomic"
)

type ExecutionMode string

const (
	ExecutionModeDirect ExecutionMode = "direct"
	ExecutionModeIntent ExecutionMode = "intent"
)

const (
	executionModeDirect uint32 = iota
	executionModeIntent
)

var currentExecutionMode atomic.Uint32

func SetExecutionMode(mode ExecutionMode) {
	switch mode {
	case ExecutionModeIntent:
		currentExecutionMode.Store(executionModeIntent)
	default:
		currentExecutionMode.Store(executionModeDirect)
	}
}

func SetExecutionModeFromString(mode string) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case string(ExecutionModeIntent):
		SetExecutionMode(ExecutionModeIntent)
	default:
		SetExecutionMode(ExecutionModeDirect)
	}
}

func GetExecutionMode() ExecutionMode {
	if currentExecutionMode.Load() == executionModeIntent {
		return ExecutionModeIntent
	}
	return ExecutionModeDirect
}

func IsIntentMode() bool {
	return GetExecutionMode() == ExecutionModeIntent
}
