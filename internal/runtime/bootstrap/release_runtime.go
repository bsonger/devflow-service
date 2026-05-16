package bootstrap

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
	releaserepo "github.com/bsonger/devflow-service/internal/release/repository"
	"github.com/bsonger/devflow-service/internal/runtime/reconcile"
	runtimerepo "github.com/bsonger/devflow-service/internal/runtime/repository"
	"github.com/bsonger/devflow-service/internal/runtime/watch"
	"github.com/bsonger/devflow-service/internal/runtime/writeback"
	"github.com/google/uuid"
)

var ErrReleaseRuntimeNotConfigured = errors.New("release runtime bootstrap is not configured")

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
	releaseStore := releaserepo.NewPostgresStore()
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
			return watch.NewRunningReleaseSource(
				newRuntimeStoreRunningReleaseSource(runtimeStore, releaseStore),
				queue,
				cfg.ControlPlaneID,
				cfg.PollInterval,
			)
		},
		ReconcilerFactory: func() releaseRuntimeReconciler {
			return reconcile.NewReleaseReconciler(
				newReleaseStateSource(runtimeStore, releaseStore),
				runtimeStore,
				newReleaseStepsWriterAdapter(writer),
				cfg.ControlPlaneID,
			)
		},
	}
}

type runtimeStoreRunningReleaseSource struct {
	runtimeStore runtimerepo.Store
	releaseStore releaserepo.Store
}

func newRuntimeStoreRunningReleaseSource(runtimeStore runtimerepo.Store, releaseStore releaserepo.Store) watch.RunningReleaseLister {
	return &runtimeStoreRunningReleaseSource{
		runtimeStore: runtimeStore,
		releaseStore: releaseStore,
	}
}

func (s *runtimeStoreRunningReleaseSource) ListRunningReleases(ctx context.Context) ([]*watch.RunningRelease, error) {
	specs, err := s.runtimeStore.ListRuntimeSpecs(ctx)
	if err != nil {
		return nil, err
	}

	seen := map[string]struct{}{}
	items := make([]*watch.RunningRelease, 0, len(specs))
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
		releaseID, err := uuid.Parse(strings.TrimSpace(workload.Labels[releasedomain.ReleaseIDLabel]))
		if err != nil || releaseID == uuid.Nil {
			continue
		}
		key := releaseID.String()
		if _, ok := seen[key]; ok {
			continue
		}
		release, err := s.releaseStore.Get(ctx, releaseID)
		if err == sql.ErrNoRows || release == nil {
			continue
		}
		if err != nil {
			return nil, err
		}
		if release.Status != releasedomain.ReleaseRunning {
			continue
		}
		seen[key] = struct{}{}
		items = append(items, &watch.RunningRelease{
			ReleaseID:      releaseID,
			ControlPlaneID: strings.TrimSpace(workload.Labels[releasedomain.ControlPlaneLabel]),
			Status:         string(release.Status),
		})
	}
	return items, nil
}

type releaseStateSourceWithRuntimeStore struct {
	releaseStore releaserepo.Store
	runtimeStore runtimerepo.Store
}

func newReleaseStateSource(runtimeStore runtimerepo.Store, releaseStore releaserepo.Store) reconcile.ReleaseStateSource {
	return &releaseStateSourceWithRuntimeStore{
		releaseStore: releaseStore,
		runtimeStore: runtimeStore,
	}
}

func (s *releaseStateSourceWithRuntimeStore) GetRunningRelease(ctx context.Context, releaseID uuid.UUID) (*reconcile.RunningRelease, error) {
	release, err := s.releaseStore.Get(ctx, releaseID)
	if err != nil {
		return nil, err
	}
	if release == nil || release.Status != releasedomain.ReleaseRunning {
		return nil, nil
	}
	controlPlaneID, _ := s.lookupControlPlaneID(ctx, release)
	return &reconcile.RunningRelease{
		ReleaseID:      release.ID,
		ApplicationID:  release.ApplicationID,
		EnvironmentID:  strings.TrimSpace(release.EnvironmentID),
		ControlPlaneID: controlPlaneID,
		Status:         string(release.Status),
	}, nil
}

func (s *releaseStateSourceWithRuntimeStore) lookupControlPlaneID(ctx context.Context, release *releasedomain.Release) (string, error) {
	if s == nil || s.runtimeStore == nil || release == nil {
		return "", nil
	}
	specs, err := s.runtimeStore.ListRuntimeSpecs(ctx)
	if err != nil {
		return "", err
	}
	for _, spec := range specs {
		if spec == nil {
			continue
		}
		if spec.ApplicationID != release.ApplicationID || strings.TrimSpace(spec.Environment) != strings.TrimSpace(release.EnvironmentID) {
			continue
		}
		workload, err := s.runtimeStore.GetObservedWorkload(ctx, spec.ID)
		if err == sql.ErrNoRows || workload == nil {
			continue
		}
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(workload.Labels[releasedomain.ReleaseIDLabel]) != release.ID.String() {
			continue
		}
		return strings.TrimSpace(workload.Labels[releasedomain.ControlPlaneLabel]), nil
	}
	return "", nil
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
