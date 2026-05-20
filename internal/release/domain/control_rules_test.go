package domain

import (
	"testing"
	"time"
)

func TestLifecycleStatusIsTerminal(t *testing.T) {
	if !LifecycleSucceeded.IsTerminal() {
		t.Fatal("expected succeeded to be terminal")
	}
	if !LifecycleFailed.IsTerminal() {
		t.Fatal("expected failed to be terminal")
	}
	if LifecycleRunning.IsTerminal() {
		t.Fatal("did not expect running to be terminal")
	}
}

func TestValidateLifecycleTransitionAllowsExpectedPaths(t *testing.T) {
	tests := []struct {
		from LifecycleStatus
		to   LifecycleStatus
	}{
		{LifecyclePending, LifecycleDispatching},
		{LifecycleDispatching, LifecycleRunning},
		{LifecycleDispatching, LifecycleFailed},
		{LifecycleRunning, LifecyclePaused},
		{LifecyclePaused, LifecycleRunning},
		{LifecyclePaused, LifecycleFailed},
		{LifecycleRunning, LifecycleFinalizing},
		{LifecycleFinalizing, LifecycleSucceeded},
		{LifecycleFinalizing, LifecycleFailed},
		{LifecycleRunning, LifecycleRunning},
	}
	for _, tt := range tests {
		if err := ValidateLifecycleTransition(tt.from, tt.to); err != nil {
			t.Fatalf("ValidateLifecycleTransition(%q,%q) error = %v", tt.from, tt.to, err)
		}
	}
}

func TestValidateLifecycleTransitionRejectsUnexpectedPaths(t *testing.T) {
	tests := []struct {
		from LifecycleStatus
		to   LifecycleStatus
	}{
		{LifecyclePending, LifecycleRunning},
		{LifecyclePaused, LifecycleSucceeded},
		{LifecycleRunning, LifecycleSucceeded},
		{LifecycleSucceeded, LifecycleRunning},
	}
	for _, tt := range tests {
		if err := ValidateLifecycleTransition(tt.from, tt.to); err == nil {
			t.Fatalf("expected transition %q -> %q to fail", tt.from, tt.to)
		}
	}
}

func TestTerminalStateCannotBeReopened(t *testing.T) {
	state := ReleaseControlState{
		LifecycleStatus: LifecycleSucceeded,
		Terminal:        true,
		UpdatedAt:       time.Now(),
	}
	if err := ValidateNonTerminalMutation(state); err == nil {
		t.Fatal("expected terminal mutation validation to fail")
	}
}
