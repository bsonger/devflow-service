package bootstrap

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bsonger/devflow-service/internal/runtime/watch"
	"k8s.io/client-go/rest"
)

func TestStartManifestRuntimeReconcilerStartsWhenEnabled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		queueCreated      atomic.Bool
		sourceStarted     atomic.Bool
		reconcilerCreated atomic.Bool
	)

	err := startManifestRuntimeReconciler(ctx, manifestRuntimeBootstrapDeps{
		QueueFactory: func() watch.ManifestQueue {
			queueCreated.Store(true)
			return &stubManifestQueue{}
		},
		ManifestSourceFactory: func(queue watch.ManifestQueue) manifestRuntimeSource {
			if queue == nil {
				t.Fatal("expected queue")
			}
			return manifestRuntimeSourceFunc(func(context.Context) {
				sourceStarted.Store(true)
			})
		},
		ReconcilerFactory: func() manifestRuntimeReconciler {
			reconcilerCreated.Store(true)
			return manifestRuntimeReconcilerFunc(func(context.Context, string) error { return nil })
		},
	}, ManifestRuntimeBootstrapConfig{
		Enabled:        true,
		ControlPlaneID: "cp-1",
		Workers:        2,
	})
	if err != nil {
		t.Fatalf("startManifestRuntimeReconciler failed: %v", err)
	}

	time.Sleep(20 * time.Millisecond)

	if !queueCreated.Load() {
		t.Fatal("expected queue factory to run")
	}
	if !sourceStarted.Load() {
		t.Fatal("expected manifest source to start")
	}
	if !reconcilerCreated.Load() {
		t.Fatal("expected reconciler factory to run")
	}
}

func TestManifestRuntimePollSourceRefreshesBeforeEnqueue(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	queue := &recordingManifestQueue{}
	cache := &refreshingStubTektonCache{
		refreshFn: func(context.Context) error {
			queue.events = append(queue.events, "refresh")
			return nil
		},
		listFn: func(string) []string {
			queue.events = append(queue.events, "list")
			return []string{"m-2", "m-1"}
		},
	}
	source := &manifestRuntimePollSource{
		cache:          cache,
		queue:          queue,
		controlPlaneID: "cp-1",
		pollInterval:   time.Hour,
	}

	done := make(chan struct{})
	go func() {
		source.Run(ctx)
		close(done)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()
	<-done

	if len(queue.added) != 2 || queue.added[0] != "m-2" || queue.added[1] != "m-1" {
		t.Fatalf("queued manifests = %#v", queue.added)
	}
	if len(queue.events) < 2 || queue.events[0] != "refresh" || queue.events[1] != "list" {
		t.Fatalf("event order = %#v, want refresh before list", queue.events)
	}
}

func TestManifestRuntimePollSourceSkipsEnqueueOnRefreshError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	queue := &recordingManifestQueue{}
	cache := &refreshingStubTektonCache{
		refreshFn: func(context.Context) error {
			return errors.New("boom")
		},
		listFn: func(string) []string {
			t.Fatal("ListManifestIDs should not be called after refresh failure")
			return nil
		},
	}
	source := &manifestRuntimePollSource{
		cache:          cache,
		queue:          queue,
		controlPlaneID: "cp-1",
		pollInterval:   time.Hour,
	}

	done := make(chan struct{})
	go func() {
		source.Run(ctx)
		close(done)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()
	<-done

	if len(queue.added) != 0 {
		t.Fatalf("queued manifests = %#v, want none", queue.added)
	}
}

func TestDefaultManifestRuntimeTektonCacheFiltersConfiguredPipeline(t *testing.T) {
	origInCluster := inClusterConfig
	defer func() { inClusterConfig = origInCluster }()

	inClusterConfig = func() (*rest.Config, error) {
		return nil, errors.New("not in cluster")
	}

	cache, err := newDefaultManifestRuntimeTektonCache(ManifestRuntimeBootstrapConfig{
		TektonPipeline: "build-pipeline",
	})
	if err != nil {
		t.Fatalf("newDefaultManifestRuntimeTektonCache() error = %v", err)
	}
	if cache != nil {
		t.Fatal("expected nil cache when cluster config is unavailable")
	}
}

type refreshingStubTektonCache struct {
	refreshFn func(context.Context) error
	listFn    func(string) []string
}

func (s *refreshingStubTektonCache) Refresh(ctx context.Context) error {
	if s.refreshFn == nil {
		return nil
	}
	return s.refreshFn(ctx)
}

func (s *refreshingStubTektonCache) ListManifestIDs(controlPlaneID string) []string {
	if s.listFn == nil {
		return nil
	}
	return s.listFn(controlPlaneID)
}

func (s *refreshingStubTektonCache) GetManifestSnapshot(string) (*watch.TektonSnapshot, bool) {
	return nil, false
}

type recordingManifestQueue struct {
	events []string
	added  []string
}

func (q *recordingManifestQueue) Add(manifestID string) {
	q.events = append(q.events, "add:"+manifestID)
	q.added = append(q.added, manifestID)
}

func (q *recordingManifestQueue) Run(ctx context.Context, workers int, handler func(context.Context, string) error) {
	<-ctx.Done()
}

func (q *recordingManifestQueue) ShutDown() {}
