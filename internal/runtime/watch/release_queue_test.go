package watch

import (
	"context"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bsonger/devflow-service/internal/platform/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func TestReleaseQueueEmitsLifecycleEvents(t *testing.T) {
	queue := NewReleaseQueue()
	defer queue.ShutDown()

	var (
		mu              sync.Mutex
		events          []string
		queueHandleDone = make(chan struct{})
	)
	releaseQueueLogf = func(event string, fields ...zap.Field) {
		mu.Lock()
		events = append(events, event)
		mu.Unlock()
		if event == "queue_handle_done" {
			select {
			case <-queueHandleDone:
			default:
				close(queueHandleDone)
			}
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runDone := make(chan struct{})
	go func() {
		defer close(runDone)
		queue.Run(ctx, 1, func(context.Context, string) error {
			return nil
		})
	}()
	defer func() {
		cancel()
		select {
		case <-runDone:
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for release queue to stop")
		}
		releaseQueueLogf = defaultReleaseQueueLogf
	}()

	queue.Add("release-1")

	select {
	case <-queueHandleDone:
	case <-time.After(5 * time.Second):
		mu.Lock()
		got := append([]string(nil), events...)
		mu.Unlock()
		t.Fatalf("timed out waiting for queue_handle_done; events = %v", got)
	}

	cancel()
	select {
	case <-runDone:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for release queue run loop")
	}

	mu.Lock()
	got := append([]string(nil), events...)
	mu.Unlock()

	if !slices.Contains(got, "queue_add") {
		t.Fatalf("events = %v, want queue_add", got)
	}
	if !slices.Contains(got, "queue_handle_start") {
		t.Fatalf("events = %v, want queue_handle_start", got)
	}
	if !slices.Contains(got, "queue_handle_done") {
		t.Fatalf("events = %v, want queue_handle_done", got)
	}
}

func TestRunningReleaseSourceEnqueuesMatchingRunningReleases(t *testing.T) {
	logger.RootLogger = zap.NewNop()
	defer func() { logger.RootLogger = nil }()

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
