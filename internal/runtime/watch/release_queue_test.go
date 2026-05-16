package watch

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

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
