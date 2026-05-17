package bootstrap

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
	runtimedomain "github.com/bsonger/devflow-service/internal/runtime/domain"
	runtimerepo "github.com/bsonger/devflow-service/internal/runtime/repository"
	"github.com/bsonger/devflow-service/internal/runtime/watch"
	"github.com/google/uuid"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

func TestStartReleaseRuntimeReconcilerStartsQueueDrivenWorkers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		queueCreated      atomic.Bool
		sourceStarted     atomic.Bool
		reconcilerCreated atomic.Bool
	)

	err := startReleaseRuntimeReconciler(ctx, releaseRuntimeBootstrapDeps{
		QueueFactory: func() watch.ReleaseQueue {
			queueCreated.Store(true)
			return &stubReleaseQueue{}
		},
		ReleaseSourceFactory: func(queue watch.ReleaseQueue) releaseRuntimeSource {
			if queue == nil {
				t.Fatal("expected queue")
			}
			return releaseRuntimeSourceFunc(func(context.Context) {
				sourceStarted.Store(true)
			})
		},
		ReconcilerFactory: func() releaseRuntimeReconciler {
			reconcilerCreated.Store(true)
			return releaseRuntimeReconcilerFunc(func(context.Context, string) error { return nil })
		},
	}, ReleaseRuntimeBootstrapConfig{
		Enabled:        true,
		ControlPlaneID: "cp-1",
		Workers:        2,
	})
	if err != nil {
		t.Fatalf("startReleaseRuntimeReconciler failed: %v", err)
	}

	time.Sleep(20 * time.Millisecond)

	if !queueCreated.Load() {
		t.Fatal("expected queue factory to run")
	}
	if !sourceStarted.Load() {
		t.Fatal("expected release source to start")
	}
	if !reconcilerCreated.Load() {
		t.Fatal("expected reconciler factory to run")
	}
}

func TestStartReleaseRuntimeReconcilerStartsWithDefaultDepsWhenEnabled(t *testing.T) {
	err := StartReleaseRuntimeReconciler(context.Background(), ReleaseRuntimeBootstrapConfig{
		Enabled:        true,
		ControlPlaneID: "cp-1",
	})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
}

func TestDefaultReleaseRuntimeBootstrapPrefersWorkloadEventSourceWhenClusterConfigAvailable(t *testing.T) {
	origCluster := inClusterConfig
	origWorkloadCache := newWorkloadCache
	defer func() {
		inClusterConfig = origCluster
		newWorkloadCache = origWorkloadCache
	}()

	inClusterConfig = func() (*rest.Config, error) {
		return &rest.Config{Host: "https://cluster.example"}, nil
	}

	newWorkloadCache = func(*rest.Config, watch.WorkloadCacheConfig) (watch.WorkloadCache, error) {
		return stubBootstrapWorkloadCache{}, nil
	}

	deps := defaultReleaseRuntimeBootstrapDeps(ReleaseRuntimeBootstrapConfig{
		Enabled:        true,
		ControlPlaneID: "cp-1",
		PollInterval:   15 * time.Second,
	})
	if deps.ReleaseSourceFactory == nil {
		t.Fatal("expected release source factory")
	}
	source := deps.ReleaseSourceFactory(&stubReleaseQueue{})
	sources, ok := source.(releaseRuntimeSources)
	if !ok {
		t.Fatalf("source type = %T, want releaseRuntimeSources", source)
	}
	if len(sources) != 2 {
		t.Fatalf("len(sources) = %d, want 2", len(sources))
	}
	if _, ok := sources[0].(*watch.ReleaseEventSource); !ok {
		t.Fatalf("sources[0] type = %T, want *watch.ReleaseEventSource", sources[0])
	}
	if _, ok := sources[1].(*watch.RunningReleaseSource); !ok {
		t.Fatalf("sources[1] type = %T, want *watch.RunningReleaseSource", sources[1])
	}
}

func TestRunningReleaseSourceReenqueuesReleaseThatLeavesRunningSet(t *testing.T) {
	releaseID := uuid.New()
	queue := &recordingReleaseQueue{}
	source := watch.NewRunningReleaseSource(&stubRunningReleaseLister{
		batches: [][]*watch.RunningRelease{
			{{ReleaseID: releaseID, ControlPlaneID: "cp-1", Status: string(releasedomain.ReleaseRunning)}},
			{},
		},
	}, queue, "cp-1", 5*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go source.Run(ctx)

	got, ok := queue.WaitForCount(2, time.Second)
	if !ok {
		t.Fatalf("timed out waiting for replay enqueue after leaving running set, got %v", queue.Added())
	}
	if got[0] != releaseID.String() || got[1] != releaseID.String() {
		t.Fatalf("enqueued %v, want replay of %s after leaving running set", got, releaseID)
	}
}

func TestRuntimeStoreRunningReleaseSourceListsOnlyRunningReleaseForMatchingObservedWorkload(t *testing.T) {
	runtimeStore := runtimerepo.NewMemoryStore()
	releaseID := uuid.New()
	appID := uuid.New()
	spec := &runtimedomain.RuntimeSpec{
		ID:            uuid.New(),
		ApplicationID: appID,
		Environment:   "staging",
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	if err := runtimeStore.CreateRuntimeSpec(context.Background(), spec); err != nil {
		t.Fatalf("CreateRuntimeSpec failed: %v", err)
	}
	if err := runtimeStore.UpsertObservedWorkload(context.Background(), &runtimedomain.RuntimeObservedWorkload{
		ID:            uuid.New(),
		RuntimeSpecID: spec.ID,
		ApplicationID: appID,
		Environment:   "staging",
		Namespace:     "devflow",
		WorkloadKind:  "Deployment",
		WorkloadName:  "demo-api",
		Labels: map[string]string{
			releasedomain.ReleaseIDLabel:     releaseID.String(),
			releasedomain.ControlPlaneLabel:  "cp-1",
			releasedomain.ReleaseStatusLabel: string(releasedomain.ReleaseRunning),
		},
		ObservedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("UpsertObservedWorkload failed: %v", err)
	}

	source := newRuntimeStoreRunningReleaseSource(runtimeStore)

	items, err := source.ListRunningReleases(context.Background())
	if err != nil {
		t.Fatalf("ListRunningReleases failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if items[0].ReleaseID != releaseID {
		t.Fatalf("ReleaseID = %s", items[0].ReleaseID)
	}
	if items[0].ControlPlaneID != "cp-1" {
		t.Fatalf("ControlPlaneID = %q", items[0].ControlPlaneID)
	}
	if items[0].Status != string(releasedomain.ReleaseRunning) {
		t.Fatalf("Status = %q", items[0].Status)
	}
}

func TestRuntimeStoreRunningReleaseSourceSkipsNonRunningReleaseStatus(t *testing.T) {
	runtimeStore := runtimerepo.NewMemoryStore()
	releaseID := uuid.New()
	appID := uuid.New()
	spec := &runtimedomain.RuntimeSpec{
		ID:            uuid.New(),
		ApplicationID: appID,
		Environment:   "staging",
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	if err := runtimeStore.CreateRuntimeSpec(context.Background(), spec); err != nil {
		t.Fatalf("CreateRuntimeSpec failed: %v", err)
	}
	if err := runtimeStore.UpsertObservedWorkload(context.Background(), &runtimedomain.RuntimeObservedWorkload{
		ID:            uuid.New(),
		RuntimeSpecID: spec.ID,
		ApplicationID: appID,
		Environment:   "staging",
		Namespace:     "devflow",
		WorkloadKind:  "Deployment",
		WorkloadName:  "demo-api",
		Labels: map[string]string{
			releasedomain.ReleaseIDLabel:     releaseID.String(),
			releasedomain.ControlPlaneLabel:  "cp-1",
			releasedomain.ReleaseStatusLabel: "Succeeded",
		},
		ObservedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("UpsertObservedWorkload failed: %v", err)
	}

	source := newRuntimeStoreRunningReleaseSource(runtimeStore)

	items, err := source.ListRunningReleases(context.Background())
	if err != nil {
		t.Fatalf("ListRunningReleases failed: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("len(items) = %d, want 0", len(items))
	}
}

func TestReleaseStateSourceReadsControlPlaneFromObservedWorkload(t *testing.T) {
	runtimeStore := runtimerepo.NewMemoryStore()
	releaseID := uuid.New()
	appID := uuid.New()
	spec := &runtimedomain.RuntimeSpec{
		ID:            uuid.New(),
		ApplicationID: appID,
		Environment:   "prod",
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	if err := runtimeStore.CreateRuntimeSpec(context.Background(), spec); err != nil {
		t.Fatalf("CreateRuntimeSpec failed: %v", err)
	}
	if err := runtimeStore.UpsertObservedWorkload(context.Background(), &runtimedomain.RuntimeObservedWorkload{
		ID:            uuid.New(),
		RuntimeSpecID: spec.ID,
		ApplicationID: appID,
		Environment:   "prod",
		Namespace:     "devflow",
		WorkloadKind:  "Deployment",
		WorkloadName:  "demo-api",
		Labels: map[string]string{
			releasedomain.ReleaseIDLabel:     releaseID.String(),
			releasedomain.ControlPlaneLabel:  "cp-prod",
			releasedomain.ReleaseStatusLabel: string(releasedomain.ReleaseRunning),
		},
		ObservedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("UpsertObservedWorkload failed: %v", err)
	}

	source := newReleaseStateSource(runtimeStore)

	item, err := source.GetRunningRelease(context.Background(), releaseID)
	if err != nil {
		t.Fatalf("GetRunningRelease failed: %v", err)
	}
	if item == nil {
		t.Fatal("expected running release")
	}
	if item.ControlPlaneID != "cp-prod" {
		t.Fatalf("ControlPlaneID = %q", item.ControlPlaneID)
	}
	if item.Status != string(releasedomain.ReleaseRunning) {
		t.Fatalf("Status = %q", item.Status)
	}
}

func TestReleaseStateSourceSkipsNonRunningReleaseStatus(t *testing.T) {
	runtimeStore := runtimerepo.NewMemoryStore()
	releaseID := uuid.New()
	appID := uuid.New()
	spec := &runtimedomain.RuntimeSpec{
		ID:            uuid.New(),
		ApplicationID: appID,
		Environment:   "prod",
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	if err := runtimeStore.CreateRuntimeSpec(context.Background(), spec); err != nil {
		t.Fatalf("CreateRuntimeSpec failed: %v", err)
	}
	if err := runtimeStore.UpsertObservedWorkload(context.Background(), &runtimedomain.RuntimeObservedWorkload{
		ID:            uuid.New(),
		RuntimeSpecID: spec.ID,
		ApplicationID: appID,
		Environment:   "prod",
		Namespace:     "devflow",
		WorkloadKind:  "Deployment",
		WorkloadName:  "demo-api",
		Labels: map[string]string{
			releasedomain.ReleaseIDLabel:     releaseID.String(),
			releasedomain.ControlPlaneLabel:  "cp-prod",
			releasedomain.ReleaseStatusLabel: "Succeeded",
		},
		ObservedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("UpsertObservedWorkload failed: %v", err)
	}

	source := newReleaseStateSource(runtimeStore)

	item, err := source.GetRunningRelease(context.Background(), releaseID)
	if err != nil {
		t.Fatalf("GetRunningRelease failed: %v", err)
	}
	if item != nil {
		t.Fatalf("item = %#v, want nil", item)
	}
}

func TestReleaseStateSourceAcceptsLowercaseRunningReleaseStatus(t *testing.T) {
	runtimeStore := runtimerepo.NewMemoryStore()
	releaseID := uuid.New()
	appID := uuid.New()
	spec := &runtimedomain.RuntimeSpec{
		ID:            uuid.New(),
		ApplicationID: appID,
		Environment:   "prod",
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	if err := runtimeStore.CreateRuntimeSpec(context.Background(), spec); err != nil {
		t.Fatalf("CreateRuntimeSpec failed: %v", err)
	}
	if err := runtimeStore.UpsertObservedWorkload(context.Background(), &runtimedomain.RuntimeObservedWorkload{
		ID:            uuid.New(),
		RuntimeSpecID: spec.ID,
		ApplicationID: appID,
		Environment:   "prod",
		Namespace:     "devflow",
		WorkloadKind:  "Deployment",
		WorkloadName:  "demo-api",
		Labels: map[string]string{
			releasedomain.ReleaseIDLabel:     releaseID.String(),
			releasedomain.ControlPlaneLabel:  "cp-prod",
			releasedomain.ReleaseStatusLabel: "running",
		},
		ObservedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("UpsertObservedWorkload failed: %v", err)
	}

	source := newReleaseStateSource(runtimeStore)

	item, err := source.GetRunningRelease(context.Background(), releaseID)
	if err != nil {
		t.Fatalf("GetRunningRelease failed: %v", err)
	}
	if item == nil {
		t.Fatal("expected running release")
	}
	if item.ReleaseID != releaseID {
		t.Fatalf("ReleaseID = %s", item.ReleaseID)
	}
}

type stubReleaseQueue struct{}

func (s *stubReleaseQueue) Add(string) {}

func (s *stubReleaseQueue) Run(ctx context.Context, workers int, handler func(context.Context, string) error) {
	<-ctx.Done()
}

func (s *stubReleaseQueue) ShutDown() {}

type recordingReleaseQueue struct {
	mu    sync.Mutex
	cond  *sync.Cond
	added []string
}

func (q *recordingReleaseQueue) Add(releaseID string) {
	q.init()
	q.mu.Lock()
	q.added = append(q.added, releaseID)
	q.cond.Broadcast()
	q.mu.Unlock()
}

func (q *recordingReleaseQueue) Run(context.Context, int, func(context.Context, string) error) {}

func (q *recordingReleaseQueue) ShutDown() {}

func (q *recordingReleaseQueue) Added() []string {
	q.init()
	q.mu.Lock()
	defer q.mu.Unlock()
	return append([]string(nil), q.added...)
}

func (q *recordingReleaseQueue) WaitForCount(want int, timeout time.Duration) ([]string, bool) {
	q.init()
	deadline := time.Now().Add(timeout)
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.added) < want {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return append([]string(nil), q.added...), false
		}
		timer := time.AfterFunc(remaining, func() {
			q.mu.Lock()
			q.cond.Broadcast()
			q.mu.Unlock()
		})
		q.cond.Wait()
		if !timer.Stop() && len(q.added) < want && time.Now().After(deadline) {
			return append([]string(nil), q.added...), false
		}
	}
	return append([]string(nil), q.added...), true
}

func (q *recordingReleaseQueue) init() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.cond == nil {
		q.cond = sync.NewCond(&q.mu)
	}
}

type stubRunningReleaseLister struct {
	mu      sync.Mutex
	batches [][]*watch.RunningRelease
	index   int
}

func (s *stubRunningReleaseLister) ListRunningReleases(context.Context) ([]*watch.RunningRelease, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.index >= len(s.batches) {
		return nil, nil
	}
	batch := s.batches[s.index]
	s.index++
	return batch, nil
}

type stubBootstrapWorkloadCache struct{}

func (s stubBootstrapWorkloadCache) Start(context.Context) error { return nil }
func (s stubBootstrapWorkloadCache) Ready() bool                 { return true }
func (s stubBootstrapWorkloadCache) AddEventHandler(cache.ResourceEventHandler) error {
	return nil
}
func (s stubBootstrapWorkloadCache) ListDeployments(string, labels.Selector) ([]appsv1.Deployment, error) {
	return nil, nil
}
func (s stubBootstrapWorkloadCache) ListRollouts(string, labels.Selector) ([]unstructured.Unstructured, error) {
	return nil, nil
}
func (s stubBootstrapWorkloadCache) ListPods(string, labels.Selector) ([]corev1.Pod, error) {
	return nil, nil
}

type releaseRuntimeSourceFunc func(context.Context)

func (f releaseRuntimeSourceFunc) Run(ctx context.Context) {
	f(ctx)
	<-ctx.Done()
}

type releaseRuntimeReconcilerFunc func(context.Context, string) error

func (f releaseRuntimeReconcilerFunc) Reconcile(ctx context.Context, key string) error {
	return f(ctx, key)
}
