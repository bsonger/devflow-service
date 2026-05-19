package domain

type TransitionDecision struct {
	StateChanged bool
	NextState    *ReleaseControlState
	StepWrites   []StepWrite
}

type StepWrite struct {
	StepCode string
	Status   StepStatus
	Progress int
	Message  string
}

func NoTransitionDecision() TransitionDecision {
	return TransitionDecision{}
}

func NewStateTransitionDecision(next ReleaseControlState, writes ...StepWrite) TransitionDecision {
	return TransitionDecision{
		StateChanged: true,
		NextState:    &next,
		StepWrites:   writes,
	}
}

func NewStepWrite(stepCode string, status StepStatus, progress int, message string) StepWrite {
	return StepWrite{
		StepCode: stepCode,
		Status:   status,
		Progress: progress,
		Message:  message,
	}
}
