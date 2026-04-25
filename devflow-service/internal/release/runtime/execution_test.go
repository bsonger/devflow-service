package runtime

import "testing"

func TestSetExecutionModeFromString(t *testing.T) {
	SetExecutionMode(ExecutionModeDirect)
	t.Cleanup(func() { SetExecutionMode(ExecutionModeDirect) })

	SetExecutionModeFromString(" intent ")
	if got := GetExecutionMode(); got != ExecutionModeIntent {
		t.Fatalf("GetExecutionMode() = %q, want %q", got, ExecutionModeIntent)
	}
	if !IsIntentMode() {
		t.Fatal("expected IsIntentMode() to be true")
	}

	SetExecutionModeFromString("unknown")
	if got := GetExecutionMode(); got != ExecutionModeDirect {
		t.Fatalf("GetExecutionMode() = %q, want %q", got, ExecutionModeDirect)
	}
	if IsIntentMode() {
		t.Fatal("expected IsIntentMode() to be false")
	}
}
