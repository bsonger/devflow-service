package bootstrap

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
	releaserepo "github.com/bsonger/devflow-service/internal/release/repository"
	runtimedomain "github.com/bsonger/devflow-service/internal/runtime/domain"
	"github.com/bsonger/devflow-service/internal/runtime/reconcile"
	runtimerepo "github.com/bsonger/devflow-service/internal/runtime/repository"
	"github.com/bsonger/devflow-service/internal/runtime/watch"
	"github.com/bsonger/devflow-service/internal/runtime/writeback"
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
	if _, ok := source.(*watch.ReleaseEventSource); !ok {
		t.Fatalf("source type = %T, want *watch.ReleaseEventSource", source)
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
			releasedomain.ReleaseIDLabel:    releaseID.String(),
			releasedomain.ControlPlaneLabel: "cp-1",
		},
		ObservedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("UpsertObservedWorkload failed: %v", err)
	}

	source := newRuntimeStoreRunningReleaseSource(runtimeStore, stubReleaseStore{
		getFn: func(context.Context, uuid.UUID) (*releasedomain.Release, error) {
			return &releasedomain.Release{
				BaseModel: releasedomain.BaseModel{ID: releaseID},
				Status:    releasedomain.ReleaseRunning,
			}, nil
		},
	})

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
			releasedomain.ReleaseIDLabel:    releaseID.String(),
			releasedomain.ControlPlaneLabel: "cp-prod",
		},
		ObservedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("UpsertObservedWorkload failed: %v", err)
	}

	source := newReleaseStateSource(runtimeStore, stubReleaseStore{
		getFn: func(context.Context, uuid.UUID) (*releasedomain.Release, error) {
			return &releasedomain.Release{
				BaseModel:     releasedomain.BaseModel{ID: releaseID},
				ApplicationID: appID,
				EnvironmentID: "prod",
				Status:        releasedomain.ReleaseRunning,
			}, nil
		},
	})

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
}

func TestReleaseStepsWriterAdapterPostsWriteback(t *testing.T) {
	var called atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Store(true)
		if r.URL.Path != "/api/v1/verify/release/steps" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	adapter := newReleaseStepsWriterAdapter(writeback.NewReleaseWriter(server.URL, "token-a", server.Client()))
	err := adapter.WriteReleaseSteps(context.Background(), reconcile.WriteReleaseStepsInput{
		ReleaseID:            uuid.New(),
		ApplicationID:        uuid.New(),
		EnvironmentID:        "staging",
		Namespace:            "devflow",
		ObservedWorkloadKind: "Deployment",
		ObservedWorkloadName: "demo-api",
		Phase:                releasedomain.StepRunning,
		Progress:             10,
		Message:              "waiting",
		StepWrites: []reconcile.ReleaseStepWrite{{
			StepCode: "observe_rollout",
			Status:   releasedomain.StepRunning,
			Progress: 10,
			Message:  "waiting",
		}},
	})
	if err != nil {
		t.Fatalf("WriteReleaseSteps failed: %v", err)
	}
	if !called.Load() {
		t.Fatal("expected writeback request")
	}
}

type stubReleaseQueue struct{}

func (s *stubReleaseQueue) Add(string) {}

func (s *stubReleaseQueue) Run(ctx context.Context, workers int, handler func(context.Context, string) error) {
	<-ctx.Done()
}

func (s *stubReleaseQueue) ShutDown() {}

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

type stubReleaseStore struct {
	getFn func(context.Context, uuid.UUID) (*releasedomain.Release, error)
}

func (s stubReleaseStore) Insert(context.Context, *releasedomain.Release) error { return nil }
func (s stubReleaseStore) Get(ctx context.Context, id uuid.UUID) (*releasedomain.Release, error) {
	return s.getFn(ctx, id)
}
func (s stubReleaseStore) Delete(context.Context, uuid.UUID) error { return nil }
func (s stubReleaseStore) List(context.Context, releaserepo.ListFilter) ([]*releasedomain.Release, error) {
	return nil, nil
}
func (s stubReleaseStore) UpdateRow(context.Context, *releasedomain.Release) error   { return nil }
func (s stubReleaseStore) UpdateSteps(context.Context, *releasedomain.Release) error { return nil }
func (s stubReleaseStore) UpdateArgoMetadata(context.Context, uuid.UUID, string, string, time.Time) error {
	return nil
}
