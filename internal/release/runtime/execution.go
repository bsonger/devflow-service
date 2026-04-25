package runtime

import "strings"

type ExecutionMode string

const (
	ExecutionModeDirect ExecutionMode = "direct"
	ExecutionModeIntent ExecutionMode = "intent"
)

var currentExecutionMode = ExecutionModeDirect

func SetExecutionMode(mode ExecutionMode) {
	switch mode {
	case ExecutionModeIntent:
		currentExecutionMode = ExecutionModeIntent
	default:
		currentExecutionMode = ExecutionModeDirect
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
	return currentExecutionMode
}

func IsIntentMode() bool {
	return currentExecutionMode == ExecutionModeIntent
}
