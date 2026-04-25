package domain

import "testing"

func TestDefaultReleaseStepsNormal(t *testing.T) {
	steps := DefaultReleaseSteps(Normal, ReleaseUpgrade)
	if len(steps) != 5 {
		t.Fatalf("unexpected step count: got %d want 5", len(steps))
	}
	if steps[0].Name != "ensure namespace" {
		t.Fatalf("unexpected first step: got %q want %q", steps[0].Name, "ensure namespace")
	}
	if steps[1].Name != "ensure pull secret" {
		t.Fatalf("unexpected second step: got %q want %q", steps[1].Name, "ensure pull secret")
	}
	if steps[2].Name != "ensure appproject destination" {
		t.Fatalf("unexpected third step: got %q want %q", steps[2].Name, "ensure appproject destination")
	}
	if steps[3].Name != "apply manifests" {
		t.Fatalf("unexpected fourth step: got %q want %q", steps[3].Name, "apply manifests")
	}
	if steps[4].Name != "deploy ready" {
		t.Fatalf("unexpected fifth step: got %q want %q", steps[4].Name, "deploy ready")
	}
}

func TestDefaultReleaseStepsCanary(t *testing.T) {
	steps := DefaultReleaseSteps(Canary, ReleaseUpgrade)
	if len(steps) != 8 {
		t.Fatalf("unexpected step count: got %d want 8", len(steps))
	}
	if steps[0].Name != "ensure namespace" {
		t.Fatalf("unexpected first step: got %q want %q", steps[0].Name, "ensure namespace")
	}
	if steps[3].Name != "apply manifests" {
		t.Fatalf("unexpected apply step: got %q want %q", steps[3].Name, "apply manifests")
	}
	if steps[4].Name != "canary 10% traffic" {
		t.Fatalf("unexpected canary step: got %q want %q", steps[4].Name, "canary 10% traffic")
	}
	if steps[7].Name != "canary 100% traffic" {
		t.Fatalf("unexpected last canary step: got %q want %q", steps[7].Name, "canary 100% traffic")
	}
}

func TestDefaultReleaseStepsBlueGreenRollback(t *testing.T) {
	steps := DefaultReleaseSteps(BlueGreen, ReleaseRollback)
	if len(steps) != 6 {
		t.Fatalf("unexpected step count: got %d want 6", len(steps))
	}
	if steps[0].Name != "ensure namespace" {
		t.Fatalf("unexpected first step: got %q want %q", steps[0].Name, "ensure namespace")
	}
	if steps[1].Name != "ensure pull secret" {
		t.Fatalf("unexpected second step: got %q want %q", steps[1].Name, "ensure pull secret")
	}
	if steps[2].Name != "ensure appproject destination" {
		t.Fatalf("unexpected third step: got %q want %q", steps[2].Name, "ensure appproject destination")
	}
	if steps[3].Name != "apply rollback manifests" {
		t.Fatalf("unexpected fourth step: got %q want %q", steps[3].Name, "apply rollback manifests")
	}
	if steps[4].Name != "green ready" {
		t.Fatalf("unexpected green step: got %q want %q", steps[4].Name, "green ready")
	}
	if steps[5].Name != "switch traffic" {
		t.Fatalf("unexpected traffic step: got %q want %q", steps[5].Name, "switch traffic")
	}
}

func TestDeriveReleaseStatusFromSteps(t *testing.T) {
	tests := []struct {
		name          string
		releaseAction string
		currentStatus ReleaseStatus
		steps         []ReleaseStep
		want          ReleaseStatus
	}{
		{
			name:          "pending when all pending",
			releaseAction: ReleaseUpgrade,
			steps: []ReleaseStep{
				{Name: "apply", Status: StepPending},
				{Name: "deploy", Status: StepPending},
			},
			want: ReleasePending,
		},
		{
			name:          "running when some started",
			releaseAction: ReleaseUpgrade,
			steps: []ReleaseStep{
				{Name: "apply", Status: StepSucceeded},
				{Name: "deploy", Status: StepRunning},
			},
			want: ReleaseRunning,
		},
		{
			name:          "succeeded when all succeeded",
			releaseAction: ReleaseUpgrade,
			steps: []ReleaseStep{
				{Name: "apply", Status: StepSucceeded},
				{Name: "deploy", Status: StepSucceeded},
			},
			want: ReleaseSucceeded,
		},
		{
			name:          "rolled back for rollback release",
			releaseAction: ReleaseRollback,
			steps: []ReleaseStep{
				{Name: "apply rollback", Status: StepSucceeded},
				{Name: "deploy ready", Status: StepSucceeded},
			},
			want: ReleaseRolledBack,
		},
		{
			name:          "failed when one step failed",
			releaseAction: ReleaseUpgrade,
			steps: []ReleaseStep{
				{Name: "apply", Status: StepSucceeded},
				{Name: "deploy", Status: StepFailed},
			},
			want: ReleaseFailed,
		},
		{
			name:          "preserve terminal sync failed",
			releaseAction: ReleaseUpgrade,
			currentStatus: ReleaseSyncFailed,
			steps: []ReleaseStep{
				{Name: "apply", Status: StepSucceeded},
				{Name: "deploy", Status: StepSucceeded},
			},
			want: ReleaseSyncFailed,
		},
		{
			name:          "syncing to running when first post-sync step starts",
			releaseAction: ReleaseUpgrade,
			currentStatus: ReleaseSyncing,
			steps: []ReleaseStep{
				{Name: "ensure namespace", Status: StepSucceeded},
				{Name: "apply manifests", Status: StepPending},
				{Name: "deploy ready", Status: StepPending},
			},
			want: ReleaseRunning,
		},
		{
			name:          "syncing preserved when all steps pending",
			releaseAction: ReleaseUpgrade,
			currentStatus: ReleaseSyncing,
			steps: []ReleaseStep{
				{Name: "ensure namespace", Status: StepPending},
				{Name: "apply manifests", Status: StepPending},
			},
			want: ReleaseSyncing,
		},
		{
			name:          "syncing to failed when step fails",
			releaseAction: ReleaseUpgrade,
			currentStatus: ReleaseSyncing,
			steps: []ReleaseStep{
				{Name: "ensure namespace", Status: StepSucceeded},
				{Name: "apply manifests", Status: StepFailed, Message: "sync timeout"},
			},
			want: ReleaseFailed,
		},
		{
			name:          "preserve terminal rolled back",
			releaseAction: ReleaseRollback,
			currentStatus: ReleaseRolledBack,
			steps: []ReleaseStep{
				{Name: "apply rollback", Status: StepSucceeded},
				{Name: "deploy ready", Status: StepSucceeded},
			},
			want: ReleaseRolledBack,
		},
		{
			name:          "preserve terminal succeeded against late failed step",
			releaseAction: ReleaseUpgrade,
			currentStatus: ReleaseSucceeded,
			steps: []ReleaseStep{
				{Name: "apply", Status: StepSucceeded},
				{Name: "deploy", Status: StepFailed, Message: "late failure"},
			},
			want: ReleaseSucceeded,
		},
		{
			name:          "preserve terminal failed against late succeeded step",
			releaseAction: ReleaseUpgrade,
			currentStatus: ReleaseFailed,
			steps: []ReleaseStep{
				{Name: "apply", Status: StepSucceeded},
				{Name: "deploy", Status: StepSucceeded},
			},
			want: ReleaseFailed,
		},
		{
			name:          "empty steps with syncing preserves syncing",
			releaseAction: ReleaseUpgrade,
			currentStatus: ReleaseSyncing,
			steps:         []ReleaseStep{},
			want:          ReleaseSyncing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeriveReleaseStatusFromSteps(tt.releaseAction, tt.currentStatus, tt.steps)
			if got != tt.want {
				t.Fatalf("unexpected status: got %q want %q", got, tt.want)
			}
		})
	}
}
