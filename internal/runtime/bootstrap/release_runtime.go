package bootstrap

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
	runtimedomain "github.com/bsonger/devflow-service/internal/runtime/domain"
	runtimeobserver "github.com/bsonger/devflow-service/internal/runtime/observer"
	"github.com/bsonger/devflow-service/internal/runtime/reconcile"
	runtimerepo "github.com/bsonger/devflow-service/internal/runtime/repository"
	"github.com/bsonger/devflow-service/internal/runtime/watch"
	"github.com/bsonger/devflow-service/internal/runtime/writeback"
	"github.com/google/uuid"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

var ErrReleaseRuntimeNotConfigured = errors.New("release runtime bootstrap is not configured")
var newWorkloadCache = watch.NewWorkloadCache

type ReleaseRuntimeBootstrapConfig struct {
	Enabled               bool
	ControlPlaneID        string
	PollInterval          time.Duration
	Workers               int
	ReleaseServiceBaseURL string
	ObserverToken         string
}

type releaseRuntimeSource interface {
	Run(context.Context)
}

type releaseRuntimeSources []releaseRuntimeSource

func (sources releaseRuntimeSources) Run(ctx context.Context) {
	for _, source := range sources {
		if source == nil {
			continue
		}
		go source.Run(ctx)
	}
	<-ctx.Done()
}

type releaseRuntimeReconciler interface {
	Reconcile(context.Context, string) error
}

type releaseRuntimeBootstrapDeps struct {
	QueueFactory         func() watch.ReleaseQueue
	ReleaseSourceFactory func(watch.ReleaseQueue) releaseRuntimeSource
	ReconcilerFactory    func() releaseRuntimeReconciler
}

func StartReleaseRuntimeReconciler(ctx context.Context, cfg ReleaseRuntimeBootstrapConfig) error {
	return startReleaseRuntimeReconciler(ctx, defaultReleaseRuntimeBootstrapDeps(cfg), cfg)
}

func startReleaseRuntimeReconciler(ctx context.Context, deps releaseRuntimeBootstrapDeps, cfg ReleaseRuntimeBootstrapConfig) error {
	if !cfg.Enabled {
		return nil
	}
	if strings.TrimSpace(cfg.ControlPlaneID) == "" {
		return ErrReleaseRuntimeNotConfigured
	}
	if deps.QueueFactory == nil || deps.ReleaseSourceFactory == nil || deps.ReconcilerFactory == nil {
		return ErrReleaseRuntimeNotConfigured
	}

	queue := deps.QueueFactory()
	if queue == nil {
		return ErrReleaseRuntimeNotConfigured
	}
	source := deps.ReleaseSourceFactory(queue)
	if source == nil {
		queue.ShutDown()
		return ErrReleaseRuntimeNotConfigured
	}
	reconciler := deps.ReconcilerFactory()
	if reconciler == nil {
		queue.ShutDown()
		return ErrReleaseRuntimeNotConfigured
	}

	workers := cfg.Workers
	if workers <= 0 {
		workers = 1
	}

	go source.Run(ctx)
	go queue.Run(ctx, workers, reconciler.Reconcile)
	return nil
}

func defaultReleaseRuntimeBootstrapDeps(cfg ReleaseRuntimeBootstrapConfig) releaseRuntimeBootstrapDeps {
	runtimeStore := runtimerepo.RuntimeStore
	writer := writeback.NewReleaseWriter(
		strings.TrimSpace(cfg.ReleaseServiceBaseURL),
		strings.TrimSpace(cfg.ObserverToken),
		&http.Client{Timeout: 10 * time.Second},
	)

	return releaseRuntimeBootstrapDeps{
		QueueFactory: func() watch.ReleaseQueue {
			return watch.NewReleaseQueue()
		},
		ReleaseSourceFactory: func(queue watch.ReleaseQueue) releaseRuntimeSource {
			runningSource := watch.NewRunningReleaseSource(newRuntimeStoreRunningReleaseSource(runtimeStore), queue, cfg.ControlPlaneID, cfg.PollInterval)
			restCfg, err := inClusterConfig()
			if err == nil {
				cache, cacheErr := newWorkloadCache(restCfg, watch.WorkloadCacheConfig{
					ResyncPeriod: cfg.PollInterval,
				})
				if cacheErr == nil && cache != nil {
					return releaseRuntimeSources{
						watch.NewReleaseEventSource(cache, queue, cfg.ControlPlaneID),
						runningSource,
					}
				}
			}
			return runningSource
		},
		ReconcilerFactory: func() releaseRuntimeReconciler {
			var labelUpdater reconcile.ReleaseStatusLabelUpdater
			if restCfg, err := inClusterConfig(); err == nil {
				if clientset, err := kubernetes.NewForConfig(restCfg); err == nil {
					if dynamicClient, err := dynamic.NewForConfig(restCfg); err == nil {
						labelUpdater = runtimeobserver.NewReleaseStatusLabelUpdater(clientset, dynamicClient)
					}
				}
			}
			return reconcile.NewReleaseReconciler(
				newReleaseStateSource(runtimeStore),
				runtimeStore,
				newReleaseStepsWriterAdapter(writer),
				labelUpdater,
				cfg.ControlPlaneID,
			)
		},
	}
}

type runtimeStoreRunningReleaseSource struct {
	runtimeStore runtimerepo.Store
}

func newRuntimeStoreRunningReleaseSource(runtimeStore runtimerepo.Store) watch.RunningReleaseLister {
	return &runtimeStoreRunningReleaseSource{
		runtimeStore: runtimeStore,
	}
}

func (s *runtimeStoreRunningReleaseSource) ListRunningReleases(ctx context.Context) ([]*watch.RunningRelease, error) {
	specs, err := s.runtimeStore.ListRuntimeSpecs(ctx)
	if err != nil {
		return nil, err
	}

	type candidate struct {
		item       *watch.RunningRelease
		observedAt time.Time
	}
	latestByAppEnv := map[string]candidate{}
	for _, spec := range specs {
		if spec == nil {
			continue
		}
		workload, err := s.runtimeStore.GetObservedWorkload(ctx, spec.ID)
		if err == sql.ErrNoRows || workload == nil {
			continue
		}
		if err != nil {
			return nil, err
		}
		item, ok := runningReleaseFromObservedWorkload(spec, workload)
		if !ok {
			continue
		}
		key := spec.ApplicationID.String() + "|" + strings.TrimSpace(spec.Environment)
		current, exists := latestByAppEnv[key]
		if !exists || workload.ObservedAt.After(current.observedAt) {
			latestByAppEnv[key] = candidate{item: item, observedAt: workload.ObservedAt}
		}
	}
	items := make([]*watch.RunningRelease, 0, len(latestByAppEnv))
	for _, entry := range latestByAppEnv {
		if entry.item == nil {
			continue
		}
		items = append(items, entry.item)
	}
	return items, nil
}

type releaseStateSourceWithRuntimeStore struct {
	runtimeStore runtimerepo.Store
}

func newReleaseStateSource(runtimeStore runtimerepo.Store) *releaseStateSourceWithRuntimeStore {
	return &releaseStateSourceWithRuntimeStore{
		runtimeStore: runtimeStore,
	}
}

func (s *releaseStateSourceWithRuntimeStore) GetRelease(ctx context.Context, releaseID uuid.UUID) (*reconcile.ReleaseRecord, error) {
	specs, err := s.runtimeStore.ListRuntimeSpecs(ctx)
	if err != nil {
		return nil, err
	}
	for _, spec := range specs {
		if spec == nil {
			continue
		}
		workload, err := s.runtimeStore.GetObservedWorkload(ctx, spec.ID)
		if err == sql.ErrNoRows || workload == nil {
			continue
		}
		if err != nil {
			return nil, err
		}
		item, ok := releaseFromObservedWorkload(spec, workload)
		if !ok || item.ReleaseID != releaseID {
			continue
		}
		return item, nil
	}
	return nil, nil
}

func (s *releaseStateSourceWithRuntimeStore) GetRunningRelease(ctx context.Context, releaseID uuid.UUID) (*reconcile.ReleaseRecord, error) {
	record, err := s.GetRelease(ctx, releaseID)
	if err != nil || record == nil {
		return record, err
	}
	if !strings.EqualFold(strings.TrimSpace(record.Status), string(releasedomain.ReleaseRunning)) {
		return nil, nil
	}
	return record, nil
}

func runningReleaseFromObservedWorkload(spec *runtimedomain.RuntimeSpec, workload *runtimedomain.RuntimeObservedWorkload) (*watch.RunningRelease, bool) {
	if spec == nil || workload == nil {
		return nil, false
	}
	releaseID, err := uuid.Parse(strings.TrimSpace(workload.Labels[releasedomain.ReleaseIDLabel]))
	if err != nil || releaseID == uuid.Nil {
		return nil, false
	}
	status := strings.TrimSpace(workload.Labels[releasedomain.ReleaseStatusLabel])
	if !strings.EqualFold(status, string(releasedomain.ReleaseRunning)) {
		return nil, false
	}
	return &watch.RunningRelease{
		ReleaseID:      releaseID,
		ControlPlaneID: strings.TrimSpace(workload.Labels[releasedomain.ControlPlaneLabel]),
		Status:         status,
	}, true
}

func releaseFromObservedWorkload(spec *runtimedomain.RuntimeSpec, workload *runtimedomain.RuntimeObservedWorkload) (*reconcile.ReleaseRecord, bool) {
	if spec == nil || workload == nil {
		return nil, false
	}
	releaseID, err := uuid.Parse(strings.TrimSpace(workload.Labels[releasedomain.ReleaseIDLabel]))
	if err != nil || releaseID == uuid.Nil {
		return nil, false
	}
	return &reconcile.ReleaseRecord{
		ReleaseID:      releaseID,
		ApplicationID:  spec.ApplicationID,
		EnvironmentID:  strings.TrimSpace(spec.Environment),
		ControlPlaneID: strings.TrimSpace(workload.Labels[releasedomain.ControlPlaneLabel]),
		Status:         strings.TrimSpace(workload.Labels[releasedomain.ReleaseStatusLabel]),
	}, true
}

type releaseStepsWriterAdapter struct {
	writer *writeback.ReleaseWriter
}

func newReleaseStepsWriterAdapter(writer *writeback.ReleaseWriter) reconcile.ReleaseStepsWriter {
	return &releaseStepsWriterAdapter{writer: writer}
}

func (a *releaseStepsWriterAdapter) WriteReleaseSteps(ctx context.Context, input reconcile.WriteReleaseStepsInput) error {
	if a == nil || a.writer == nil {
		return ErrReleaseRuntimeNotConfigured
	}
	stepWrites := make([]writeback.ReleaseStepWrite, 0, len(input.StepWrites))
	for _, step := range input.StepWrites {
		stepWrites = append(stepWrites, writeback.ReleaseStepWrite{
			StepCode: step.StepCode,
			Status:   step.Status,
			Progress: step.Progress,
			Message:  step.Message,
		})
	}
	return a.writer.WriteReleaseSteps(ctx, writeback.WriteReleaseStepsInput{
		ReleaseID:            input.ReleaseID,
		ApplicationID:        input.ApplicationID,
		EnvironmentID:        input.EnvironmentID,
		Namespace:            input.Namespace,
		ObservedWorkloadKind: input.ObservedWorkloadKind,
		ObservedWorkloadName: input.ObservedWorkloadName,
		Phase:                input.Phase,
		Progress:             input.Progress,
		Message:              input.Message,
		StepWrites:           stepWrites,
	})
}
