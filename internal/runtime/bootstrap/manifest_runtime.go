package bootstrap

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/bsonger/devflow-service/internal/runtime/reconcile"
	"github.com/bsonger/devflow-service/internal/runtime/watch"
	"github.com/bsonger/devflow-service/internal/runtime/writeback"
	tknv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

const (
	manifestTektonStatusPath       = "/api/v1/release/manifests/tekton/status"
	manifestTektonResultPath       = "/api/v1/release/manifests/tekton/result"
	manifestTektonTasksPath        = "/api/v1/release/manifests/tekton/tasks"
	manifestTektonStatusLegacyPath = "/api/v1/manifests/tekton/status"
	manifestTektonResultLegacyPath = "/api/v1/manifests/tekton/result"
	manifestTektonTasksLegacyPath  = "/api/v1/manifests/tekton/tasks"
	defaultManifestPollInterval    = 15 * time.Second
	defaultTektonNamespace         = "tekton-pipelines"
)

var ErrManifestRuntimeNotConfigured = errors.New("manifest runtime bootstrap is not configured")

var inClusterConfig = rest.InClusterConfig

type ManifestRuntimeBootstrapConfig struct {
	Enabled               bool
	ControlPlaneID        string
	PollInterval          time.Duration
	Workers               int
	ReleaseServiceBaseURL string
	ObserverToken         string
	TektonNamespace       string
	TektonPipeline        string
}

type manifestRuntimeSource interface {
	Run(context.Context)
}

type manifestRuntimeReconciler interface {
	Reconcile(context.Context, string) error
}

type manifestRuntimeRefreshingSource interface {
	watch.TektonCache
	Refresh(context.Context) error
}

type manifestRuntimeBootstrapDeps struct {
	QueueFactory          func() watch.ManifestQueue
	ManifestSourceFactory func(watch.ManifestQueue) manifestRuntimeSource
	ReconcilerFactory     func() manifestRuntimeReconciler
}

func StartManifestRuntimeReconciler(ctx context.Context, cfg ManifestRuntimeBootstrapConfig) error {
	return startManifestRuntimeReconciler(ctx, defaultManifestRuntimeBootstrapDeps(cfg), cfg)
}

func startManifestRuntimeReconciler(ctx context.Context, deps manifestRuntimeBootstrapDeps, cfg ManifestRuntimeBootstrapConfig) error {
	if !cfg.Enabled {
		return nil
	}
	if strings.TrimSpace(cfg.ControlPlaneID) == "" {
		return ErrManifestRuntimeNotConfigured
	}
	if deps.QueueFactory == nil || deps.ManifestSourceFactory == nil || deps.ReconcilerFactory == nil {
		return ErrManifestRuntimeNotConfigured
	}

	queue := deps.QueueFactory()
	if queue == nil {
		return ErrManifestRuntimeNotConfigured
	}
	source := deps.ManifestSourceFactory(queue)
	if source == nil {
		queue.ShutDown()
		return ErrManifestRuntimeNotConfigured
	}
	reconciler := deps.ReconcilerFactory()
	if reconciler == nil {
		queue.ShutDown()
		return ErrManifestRuntimeNotConfigured
	}

	workers := cfg.Workers
	if workers <= 0 {
		workers = 1
	}

	go source.Run(ctx)
	go queue.Run(ctx, workers, reconciler.Reconcile)
	return nil
}

func defaultManifestRuntimeBootstrapDeps(cfg ManifestRuntimeBootstrapConfig) manifestRuntimeBootstrapDeps {
	cache, err := newDefaultManifestRuntimeTektonCache(cfg)
	if err != nil {
		return manifestRuntimeBootstrapDeps{}
	}
	writer := newManifestWriterAdapter(writeback.NewReleaseWriter(
		strings.TrimSpace(cfg.ReleaseServiceBaseURL),
		strings.TrimSpace(cfg.ObserverToken),
		&http.Client{Timeout: 10 * time.Second},
	))

	return manifestRuntimeBootstrapDeps{
		QueueFactory: func() watch.ManifestQueue {
			return watch.NewManifestQueue()
		},
		ManifestSourceFactory: func(queue watch.ManifestQueue) manifestRuntimeSource {
			return &manifestRuntimePollSource{
				cache:          cache,
				queue:          queue,
				controlPlaneID: strings.TrimSpace(cfg.ControlPlaneID),
				pollInterval:   cfg.PollInterval,
			}
		},
		ReconcilerFactory: func() manifestRuntimeReconciler {
			return reconcile.NewManifestReconciler(cache, writer)
		},
	}
}

type manifestRuntimePollSource struct {
	cache          manifestRuntimeRefreshingSource
	queue          watch.ManifestQueue
	controlPlaneID string
	pollInterval   time.Duration
}

func (s *manifestRuntimePollSource) Run(ctx context.Context) {
	if s == nil || s.cache == nil || s.queue == nil {
		<-ctx.Done()
		return
	}

	interval := s.pollInterval
	if interval <= 0 {
		interval = defaultManifestPollInterval
	}

	s.enqueue()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.enqueue()
		}
	}
}

func (s *manifestRuntimePollSource) enqueue() {
	if err := s.cache.Refresh(context.Background()); err != nil {
		return
	}
	for _, manifestID := range s.cache.ListManifestIDs(s.controlPlaneID) {
		s.queue.Add(manifestID)
	}
}

func newDefaultManifestRuntimeTektonCache(cfg ManifestRuntimeBootstrapConfig) (manifestRuntimeRefreshingSource, error) {
	restCfg, err := inClusterConfig()
	if err != nil {
		return nil, nil
	}
	clientset, err := tektonclient.NewForConfig(restCfg)
	if err != nil {
		return nil, err
	}
	namespace := strings.TrimSpace(cfg.TektonNamespace)
	if namespace == "" {
		namespace = defaultTektonNamespace
	}
	return watch.NewPollingTektonCache(
		func(ctx context.Context) ([]tknv1.PipelineRun, error) {
			list, err := clientset.TektonV1().PipelineRuns(namespace).List(ctx, metav1.ListOptions{})
			if err != nil {
				return nil, err
			}
			return append([]tknv1.PipelineRun(nil), list.Items...), nil
		},
		func(ctx context.Context, taskNamespace, pipelineRunName string) ([]tknv1.TaskRun, error) {
			list, err := clientset.TektonV1().TaskRuns(taskNamespace).List(ctx, metav1.ListOptions{
				LabelSelector: "tekton.dev/pipelineRun=" + strings.TrimSpace(pipelineRunName),
			})
			if err != nil {
				return nil, err
			}
			return append([]tknv1.TaskRun(nil), list.Items...), nil
		},
		strings.TrimSpace(cfg.TektonPipeline),
	), nil
}

type manifestWriterAdapter struct {
	writer *writeback.ReleaseWriter
}

func newManifestWriterAdapter(writer *writeback.ReleaseWriter) reconcile.ManifestWriter {
	return &manifestWriterAdapter{writer: writer}
}

func (a *manifestWriterAdapter) WriteStatus(ctx context.Context, input reconcile.ManifestStatusWrite) error {
	return a.postJSON(ctx, map[string]any{
		"manifest_id": strings.TrimSpace(input.ManifestID),
		"pipeline_id": strings.TrimSpace(input.PipelineID),
		"status":      string(input.Status),
		"message":     strings.TrimSpace(input.Message),
	}, manifestTektonStatusPath, manifestTektonStatusLegacyPath)
}

func (a *manifestWriterAdapter) WriteTask(ctx context.Context, input reconcile.ManifestTaskWrite) error {
	return a.postJSON(ctx, map[string]any{
		"manifest_id": strings.TrimSpace(input.ManifestID),
		"pipeline_id": strings.TrimSpace(input.PipelineID),
		"task_name":   strings.TrimSpace(input.TaskName),
		"task_run":    strings.TrimSpace(input.TaskRun),
		"status":      string(input.Status),
		"message":     strings.TrimSpace(input.Message),
	}, manifestTektonTasksPath, manifestTektonTasksLegacyPath)
}

func (a *manifestWriterAdapter) WriteResult(ctx context.Context, input reconcile.ManifestResultWrite) error {
	return a.postJSON(ctx, map[string]any{
		"manifest_id":  strings.TrimSpace(input.ManifestID),
		"pipeline_id":  strings.TrimSpace(input.PipelineID),
		"commit_hash":  strings.TrimSpace(input.CommitHash),
		"image_ref":    strings.TrimSpace(input.ImageRef),
		"image_tag":    strings.TrimSpace(input.ImageTag),
		"image_digest": strings.TrimSpace(input.ImageDigest),
	}, manifestTektonResultPath, manifestTektonResultLegacyPath)
}

func (a *manifestWriterAdapter) postJSON(ctx context.Context, payload any, paths ...string) error {
	if a == nil || a.writer == nil {
		return ErrManifestRuntimeNotConfigured
	}
	var lastErr error
	for _, path := range uniqueManifestPaths(paths) {
		err := a.writer.PostJSON(ctx, path, payload)
		if err == nil {
			return nil
		}
		lastErr = err
		if !writeback.IsNotFound(err) {
			return err
		}
	}
	return lastErr
}

func uniqueManifestPaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		out = append(out, path)
	}
	return out
}

type stubManifestQueue struct{}

func (s *stubManifestQueue) Add(string) {}

func (s *stubManifestQueue) Run(ctx context.Context, workers int, handler func(context.Context, string) error) {
	<-ctx.Done()
}

func (s *stubManifestQueue) ShutDown() {}

type manifestRuntimeSourceFunc func(context.Context)

func (f manifestRuntimeSourceFunc) Run(ctx context.Context) {
	f(ctx)
	<-ctx.Done()
}

type manifestRuntimeReconcilerFunc func(context.Context, string) error

func (f manifestRuntimeReconcilerFunc) Reconcile(ctx context.Context, key string) error {
	return f(ctx, key)
}
