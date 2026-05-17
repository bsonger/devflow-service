package watch

import (
	"context"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

func TestReleaseQueueEmitsLifecycleEvents(t *testing.T) {
	queue := NewReleaseQueue()
	defer queue.ShutDown()

	var events []string
	releaseQueueLogf = func(event string, fields ...zap.Field) {
		events = append(events, event)
	}
	defer func() { releaseQueueLogf = defaultReleaseQueueLogf }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go queue.Run(ctx, 1, func(context.Context, string) error {
		cancel()
		return nil
	})

	queue.Add("release-1")
	<-ctx.Done()

	if !slices.Contains(events, "queue_add") {
		t.Fatalf("events = %v, want queue_add", events)
	}
	if !slices.Contains(events, "queue_handle_start") {
		t.Fatalf("events = %v, want queue_handle_start", events)
	}
	if !slices.Contains(events, "queue_handle_done") {
		t.Fatalf("events = %v, want queue_handle_done", events)
	}
}

func TestRunningReleaseSourceEnqueuesMatchingRunningReleases(t *testing.T) {
	queue := NewReleaseQueue()
	defer queue.ShutDown()

	var handled atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go queue.Run(ctx, 1, func(context.Context, string) error {
		handled.Add(1)
		cancel()
		return nil
	})

	source := NewRunningReleaseSource(stubRunningReleaseLister{
		items: []*RunningRelease{
			{ReleaseID: uuid.New(), ControlPlaneID: "cp-1", Status: "running"},
			{ReleaseID: uuid.New(), ControlPlaneID: "cp-2", Status: "running"},
			{ReleaseID: uuid.New(), ControlPlaneID: "cp-1", Status: "succeeded"},
		},
	}, queue, "cp-1", time.Hour)

	source.sync(context.Background())
	<-ctx.Done()

	if handled.Load() != 1 {
		t.Fatalf("handled = %d, want 1", handled.Load())
	}
}

type stubRunningReleaseLister struct {
	items []*RunningRelease
}

func (s stubRunningReleaseLister) ListRunningReleases(context.Context) ([]*RunningRelease, error) {
	return s.items, nil
}
