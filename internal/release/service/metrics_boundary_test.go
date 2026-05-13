package service

import (
	"context"
	"testing"
	"time"

	store "github.com/bsonger/devflow-service/internal/platform/db"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/google/uuid"
)

func TestReleaseMetricAttributesExcludeServiceIdentity(t *testing.T) {
	attrs := releaseMetricAttributes(&model.Release{Type: model.ReleaseUpgrade})
	got := map[string]string{}
	for _, attr := range attrs {
		got[string(attr.Key)] = attr.Value.AsString()
	}
	if got["release_type"] != model.ReleaseUpgrade {
		t.Fatalf("release_type = %q, want %q", got["release_type"], model.ReleaseUpgrade)
	}
	for _, key := range []string{"service_name", "service_namespace", "deployment_environment_name"} {
		if _, ok := got[key]; ok {
			t.Fatalf("unexpected metric label %q in %v", key, got)
		}
	}
}

func TestUpdateStepRecordsReleaseStageMetricOnTerminalStage(t *testing.T) {
	setupTestDB(t)
	releaseID := uuid.New()
	appID := uuid.New()
	manifestID := uuid.New()

	stepsJSON, _ := marshalJSON(model.DefaultReleaseSteps(model.Normal, model.ReleaseUpgrade), "[]")
	_, err := store.DB().ExecContext(context.Background(), `
		insert into releases (id, application_id, manifest_id, env, type, strategy, steps, status, created_at, updated_at, deleted_at)
		values ($1,$2,$3,'staging','Upgrade','rolling',$4,'Running',$5,$6,null)
	`, releaseID.String(), appID.String(), manifestID.String(), stepsJSON, time.Now().Add(-2*time.Minute), time.Now().Add(-2*time.Minute))
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	var gotStage string
	var gotSuccess bool
	original := observeReleaseStageMetricsFunc
	observeReleaseStageMetricsFunc = func(_ context.Context, release *model.Release, stage string, success bool, duration time.Duration) {
		if release != nil && release.ID == releaseID {
			gotStage = stage
			gotSuccess = success
		}
	}
	defer func() { observeReleaseStageMetricsFunc = original }()

	svc := &releaseService{}
	if err := svc.UpdateStep(context.Background(), releaseID, "publish_bundle", model.StepSucceeded, 100, "ok", nil, nil); err != nil {
		t.Fatalf("UpdateStep returned error: %v", err)
	}
	if gotStage != "publish_bundle" {
		t.Fatalf("stage = %q, want publish_bundle", gotStage)
	}
	if !gotSuccess {
		t.Fatal("expected success metric")
	}
}
