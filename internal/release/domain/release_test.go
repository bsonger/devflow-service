package domain

import "testing"

func TestDefaultReleaseStepsNormal(t *testing.T) {
	steps := DefaultReleaseSteps(Normal, ReleaseUpgrade)
	if len(steps) != 10 {
		t.Fatalf("unexpected step count: got %d want 10", len(steps))
	}
	if steps[0].Code != "freeze_inputs" {
		t.Fatalf("unexpected first step code: got %q want %q", steps[0].Code, "freeze_inputs")
	}
	if steps[1].Code != "ensure_namespace" {
		t.Fatalf("unexpected second step code: got %q want %q", steps[1].Code, "ensure_namespace")
	}
	if steps[2].Code != "ensure_pull_secret" {
		t.Fatalf("unexpected third step code: got %q want %q", steps[2].Code, "ensure_pull_secret")
	}
	if steps[3].Code != "ensure_appproject_destination" {
		t.Fatalf("unexpected fourth step code: got %q want %q", steps[3].Code, "ensure_appproject_destination")
	}
	if steps[9].Code != "finalize_release" {
		t.Fatalf("unexpected final step code: got %q want %q", steps[9].Code, "finalize_release")
	}
}

func TestDefaultReleaseStepsCanary(t *testing.T) {
	steps := DefaultReleaseSteps(Canary, ReleaseUpgrade)
	if len(steps) != 13 {
		t.Fatalf("unexpected step count: got %d want 13", len(steps))
	}
	if steps[0].Code != "freeze_inputs" {
		t.Fatalf("unexpected first step code: got %q want %q", steps[0].Code, "freeze_inputs")
	}
	if steps[4].Code != "render_deployment_bundle" {
		t.Fatalf("unexpected render step code: got %q want %q", steps[4].Code, "render_deployment_bundle")
	}
	if steps[7].Code != "deploy_canary" {
		t.Fatalf("unexpected deploy canary code: got %q want %q", steps[4].Code, "deploy_canary")
	}
	if steps[8].Code != "canary_10" {
		t.Fatalf("unexpected canary step code: got %q want %q", steps[8].Code, "canary_10")
	}
	if steps[12].Code != "finalize_release" {
		t.Fatalf("unexpected last step code: got %q want %q", steps[12].Code, "finalize_release")
	}
}

func TestNormalizeReleaseStrategy(t *testing.T) {
	if got := NormalizeReleaseStrategy(""); got != string(ReleaseStrategyRolling) {
		t.Fatalf("NormalizeReleaseStrategy(\"\") = %q", got)
	}
	if got := NormalizeReleaseStrategy("blue-green"); got != string(ReleaseStrategyBlueGreen) {
		t.Fatalf("NormalizeReleaseStrategy(blue-green) = %q", got)
	}
	if got := NormalizeReleaseStrategy("canary"); got != string(ReleaseStrategyCanary) {
		t.Fatalf("NormalizeReleaseStrategy(canary) = %q", got)
	}
}

func TestDefaultReleaseStepsBlueGreenRollback(t *testing.T) {
	steps := DefaultReleaseSteps(BlueGreen, ReleaseRollback)
	if len(steps) != 12 {
		t.Fatalf("unexpected step count: got %d want 12", len(steps))
	}
	if steps[0].Code != "freeze_inputs" {
		t.Fatalf("unexpected first step code: got %q want %q", steps[0].Code, "freeze_inputs")
	}
	if steps[4].Code != "render_deployment_bundle" {
		t.Fatalf("unexpected render step code: got %q want %q", steps[4].Code, "render_deployment_bundle")
	}
	if steps[7].Code != "deploy_preview" {
		t.Fatalf("unexpected preview step code: got %q want %q", steps[4].Code, "deploy_preview")
	}
	if steps[9].Code != "switch_traffic" {
		t.Fatalf("unexpected switch traffic code: got %q want %q", steps[6].Code, "switch_traffic")
	}
	if steps[11].Code != "finalize_release" {
		t.Fatalf("unexpected final step code: got %q want %q", steps[11].Code, "finalize_release")
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
				{Code: "apply", Name: "apply", Status: StepPending},
				{Code: "deploy", Name: "deploy", Status: StepPending},
			},
			want: ReleasePending,
		},
		{
			name:          "running when some started",
			releaseAction: ReleaseUpgrade,
			steps: []ReleaseStep{
				{Code: "apply", Name: "apply", Status: StepSucceeded},
				{Code: "deploy", Name: "deploy", Status: StepRunning},
			},
			want: ReleaseRunning,
		},
		{
			name:          "succeeded when all succeeded",
			releaseAction: ReleaseUpgrade,
			steps: []ReleaseStep{
				{Code: "apply", Name: "apply", Status: StepSucceeded},
				{Code: "deploy", Name: "deploy", Status: StepSucceeded},
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
