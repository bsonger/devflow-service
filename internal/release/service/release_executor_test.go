package service

import (
	"context"
	"errors"
	"testing"
	"time"

	intentdomain "github.com/bsonger/devflow-service/internal/intent/domain"
	intentservice "github.com/bsonger/devflow-service/internal/intent/service"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/google/uuid"
)

type stubReleaseIntentCoordinator struct {
	claimFn         func(context.Context, model.IntentKind, string, time.Duration) (*intentdomain.Intent, error)
	markSubmittedFn func(context.Context, uuid.UUID, string, string) error
	markFailedFn    func(context.Context, uuid.UUID, string) error
}

func (s stubReleaseIntentCoordinator) ClaimNextPendingByKind(ctx context.Context, kind model.IntentKind, workerID string, leaseDuration time.Duration) (*intentdomain.Intent, error) {
	return s.claimFn(ctx, kind, workerID, leaseDuration)
}

func (s stubReleaseIntentCoordinator) MarkSubmitted(ctx context.Context, id uuid.UUID, externalRef, message string) error {
	if s.markSubmittedFn == nil {
		return nil
	}
	return s.markSubmittedFn(ctx, id, externalRef, message)
}

func (s stubReleaseIntentCoordinator) MarkFailed(ctx context.Context, id uuid.UUID, message string) error {
	if s.markFailedFn == nil {
		return nil
	}
	return s.markFailedFn(ctx, id, message)
}

func TestProcessNextReleaseIntentReturnsFalseWhenNoPendingIntent(t *testing.T) {
	originalIntentService := releaseIntentService
	originalDispatcher := dispatchReleaseIntent
	t.Cleanup(func() {
		releaseIntentService = originalIntentService
		dispatchReleaseIntent = originalDispatcher
	})

	releaseIntentService = stubReleaseIntentCoordinator{
		claimFn: func(context.Context, model.IntentKind, string, time.Duration) (*intentdomain.Intent, error) {
			return nil, intentservice.ErrIntentNotFound
		},
	}

	processed, err := (&releaseService{}).ProcessNextReleaseIntent(context.Background(), "worker-1", time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if processed {
		t.Fatal("expected processed=false when no pending release intent")
	}
}

func TestProcessNextReleaseIntentDispatchesAndMarksSubmitted(t *testing.T) {
	originalIntentService := releaseIntentService
	originalDispatcher := dispatchReleaseIntent
	t.Cleanup(func() {
		releaseIntentService = originalIntentService
		dispatchReleaseIntent = originalDispatcher
	})

	intentID := uuid.New()
	releaseID := uuid.New()
	dispatched := false
	submitted := false

	releaseIntentService = stubReleaseIntentCoordinator{
		claimFn: func(_ context.Context, kind model.IntentKind, workerID string, leaseDuration time.Duration) (*intentdomain.Intent, error) {
			if kind != model.IntentKindRelease {
				t.Fatalf("kind = %q", kind)
			}
			if workerID != "worker-1" {
				t.Fatalf("workerID = %q", workerID)
			}
			if leaseDuration != time.Minute {
				t.Fatalf("leaseDuration = %s", leaseDuration)
			}
			return &intentdomain.Intent{
				BaseModel:  model.BaseModel{ID: intentID},
				Kind:       model.IntentKindRelease,
				Status:     model.IntentPending,
				ResourceID: releaseID,
			}, nil
		},
		markSubmittedFn: func(_ context.Context, id uuid.UUID, externalRef, message string) error {
			submitted = id == intentID && externalRef == "" && message == "release dispatched"
			return nil
		},
	}
	dispatchReleaseIntent = func(_ context.Context, _ *releaseService, gotReleaseID uuid.UUID) error {
		dispatched = gotReleaseID == releaseID
		return nil
	}

	processed, err := (&releaseService{}).ProcessNextReleaseIntent(context.Background(), "worker-1", time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !processed {
		t.Fatal("expected processed=true")
	}
	if !dispatched {
		t.Fatal("expected dispatchReleaseIntent to be called")
	}
	if !submitted {
		t.Fatal("expected MarkSubmitted to be called")
	}
}

func TestProcessNextReleaseIntentMarksFailedWhenDispatchFails(t *testing.T) {
	originalIntentService := releaseIntentService
	originalDispatcher := dispatchReleaseIntent
	t.Cleanup(func() {
		releaseIntentService = originalIntentService
		dispatchReleaseIntent = originalDispatcher
	})

	intentID := uuid.New()
	releaseID := uuid.New()
	dispatchErr := errors.New("dispatch failed")
	failed := false

	releaseIntentService = stubReleaseIntentCoordinator{
		claimFn: func(context.Context, model.IntentKind, string, time.Duration) (*intentdomain.Intent, error) {
			return &intentdomain.Intent{
				BaseModel:  model.BaseModel{ID: intentID},
				Kind:       model.IntentKindRelease,
				Status:     model.IntentPending,
				ResourceID: releaseID,
			}, nil
		},
		markFailedFn: func(_ context.Context, id uuid.UUID, message string) error {
			failed = id == intentID && message == dispatchErr.Error()
			return nil
		},
	}
	dispatchReleaseIntent = func(context.Context, *releaseService, uuid.UUID) error {
		return dispatchErr
	}

	processed, err := (&releaseService{}).ProcessNextReleaseIntent(context.Background(), "worker-1", time.Minute)
	if !processed {
		t.Fatal("expected processed=true")
	}
	if !errors.Is(err, dispatchErr) {
		t.Fatalf("err = %v want %v", err, dispatchErr)
	}
	if !failed {
		t.Fatal("expected MarkFailed to be called")
	}
}
