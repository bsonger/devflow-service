package bootstrap

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bsonger/devflow-service/internal/runtime/watch"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
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

func TestManifestRuntimeInformerSourceEnqueuesExistingManifestsAfterStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	queue := &recordingManifestQueue{}
	cache := &stubInformerTektonCache{
		listManifestIDs: func(controlPlaneID string) []string {
			if controlPlaneID != "cp-1" {
				t.Fatalf("controlPlaneID = %q, want cp-1", controlPlaneID)
			}
			return []string{"m-2", "m-1"}
		},
	}
	source := &manifestRuntimeInformerSource{
		cache:          cache,
		queue:          queue,
		controlPlaneID: "cp-1",
		eventSource:    watch.NewManifestEventSource(cache, queue, "cp-1"),
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
	if !cache.startCalled {
		t.Fatal("expected informer cache Start to be called")
	}
	if !cache.handlerRegistered {
		t.Fatal("expected manifest event source to register event handlers")
	}
}

func TestNewDefaultManifestRuntimeTektonSourceReturnsNilWhenClusterConfigUnavailable(t *testing.T) {
	origInCluster := inClusterConfig
	defer func() { inClusterConfig = origInCluster }()

	inClusterConfig = func() (*rest.Config, error) {
		return nil, errors.New("not in cluster")
	}

	cache, source, err := newDefaultManifestRuntimeTektonSource(ManifestRuntimeBootstrapConfig{
		TektonPipeline: "build-pipeline",
	})
	if err != nil {
		t.Fatalf("newDefaultManifestRuntimeTektonSource() error = %v", err)
	}
	if cache != nil || source != nil {
		t.Fatalf("expected nil source/cache when cluster config is unavailable, got cache=%T source=%T", cache, source)
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

type stubInformerTektonCache struct {
	startCalled       bool
	handlerRegistered bool
	listManifestIDs   func(string) []string
}

func (s *stubInformerTektonCache) Start(context.Context) error {
	s.startCalled = true
	return nil
}

func (s *stubInformerTektonCache) Ready() bool {
	return true
}

func (s *stubInformerTektonCache) AddEventHandler(cache.ResourceEventHandler) error {
	s.handlerRegistered = true
	return nil
}

func (s *stubInformerTektonCache) ListManifestIDs(controlPlaneID string) []string {
	if s.listManifestIDs == nil {
		return nil
	}
	return s.listManifestIDs(controlPlaneID)
}

func (s *stubInformerTektonCache) GetManifestSnapshot(string) (*watch.TektonSnapshot, bool) {
	return nil, false
}
