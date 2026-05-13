package service

import (
	"context"
	"testing"
	"time"

	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/google/uuid"
)

func TestUpdateManifestStatusRecordsTerminalMetric(t *testing.T) {
	setupManifestTestDB(t)
	svc := &manifestService{}
	manifestID := uuid.New()
	appID := uuid.New()
	now := time.Now().Add(-2 * time.Minute)
	item := &manifestdomain.Manifest{
		BaseModel:     model.BaseModel{ID: manifestID, CreatedAt: now, UpdatedAt: now},
		ApplicationID: appID,
		PipelineID:    "pipe-metric-status",
		Status:        model.ManifestRunning,
		ImageRef:      "registry.example.com/devflow/demo-api:pending",
	}
	if err := svc.repoStore().Insert(context.Background(), item); err != nil {
		t.Fatalf("insert manifest: %v", err)
	}
	originalMark := manifestMarkPipelineRunObserveState
	manifestMarkPipelineRunObserveState = func(context.Context, string, string, string) error {
		return nil
	}
	defer func() { manifestMarkPipelineRunObserveState = originalMark }()

	var called bool
	original := observeManifestMetricsFunc
	observeManifestMetricsFunc = func(_ context.Context, manifest *manifestdomain.Manifest, success bool, duration time.Duration) {
		called = manifest != nil && manifest.ID == manifestID && success && duration > 0
	}
	defer func() { observeManifestMetricsFunc = original }()

	if err := svc.UpdateManifestStatus(context.Background(), item.PipelineID, model.ManifestAvailable); err != nil {
		t.Fatalf("UpdateManifestStatus() error = %v", err)
	}
	if !called {
		t.Fatal("expected manifest metric observation")
	}
}
