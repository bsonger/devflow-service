package observer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	manifesthttp "github.com/bsonger/devflow-service/internal/manifest/transport/http"
	"github.com/bsonger/devflow-service/internal/platform/observer"
	platformobs "github.com/bsonger/devflow-service/internal/platform/runtime/observability"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	tknv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

const (
	defaultTektonNamespace         = "tekton-pipelines"
	defaultObserverInterval        = 15 * time.Second
	manifestIDLabel                = "devflow.manifest/id"
	manifestTektonStatusPath       = "/api/v1/release/manifests/tekton/status"
	manifestTektonResultPath       = "/api/v1/release/manifests/tekton/result"
	manifestTektonTasksPath        = "/api/v1/release/manifests/tekton/tasks"
	manifestTektonStatusLegacyPath = "/api/v1/manifests/tekton/status"
	manifestTektonResultLegacyPath = "/api/v1/manifests/tekton/result"
	manifestTektonTasksLegacyPath  = "/api/v1/manifests/tekton/tasks"
)

type TektonManifestObserverConfig struct {
	Enabled               bool
	ControlPlaneID        string
	TektonNamespace       string
	PollInterval          time.Duration
	ReleaseServiceBaseURL string
	ObserverToken         string
	HTTPTimeout           time.Duration
}

type TektonManifestObserver struct {
	cfg         TektonManifestObserverConfig
	tekton      tektonclient.Interface
	httpClient  *http.Client
	releaseBase string
	mu          sync.Mutex
	processed   map[string]string
}

func StartTektonManifestObserver(ctx context.Context, restCfg *rest.Config, cfg TektonManifestObserverConfig) error {
	if !cfg.Enabled {
		return nil
	}
	// Legacy polling path retained as a migration fallback while queue-driven manifest
	// reconcile remains feature-flagged and live Tekton cache wiring is still incomplete.
	cfg.TektonNamespace = strings.TrimSpace(cfg.TektonNamespace)
	if cfg.TektonNamespace == "" {
		cfg.TektonNamespace = defaultTektonNamespace
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = defaultObserverInterval
	}
	cfg.ReleaseServiceBaseURL = strings.TrimRight(strings.TrimSpace(cfg.ReleaseServiceBaseURL), "/")
	if cfg.ReleaseServiceBaseURL == "" {
		return nil
	}
	if cfg.HTTPTimeout <= 0 {
		cfg.HTTPTimeout = 10 * time.Second
	}

	clientset, err := tektonclient.NewForConfig(restCfg)
	if err != nil {
		return err
	}
	observer := &TektonManifestObserver{
		cfg:         cfg,
		tekton:      clientset,
		httpClient:  &http.Client{Timeout: cfg.HTTPTimeout},
		releaseBase: cfg.ReleaseServiceBaseURL,
		processed:   map[string]string{},
	}
	go observer.run(ctx)
	return nil
}

func (o *TektonManifestObserver) run(ctx context.Context) {
	ticker := time.NewTicker(o.cfg.PollInterval)
	defer ticker.Stop()
	o.sync(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			o.sync(ctx)
		}
	}
}

// sync polls build-side Tekton resources and forwards only manifest writeback updates to release-service.
// It does not report release rollout progress; deploy-phase diagnostics continue through release rows, bundle preview, and Argo-facing execution.
func (o *TektonManifestObserver) sync(ctx context.Context) {
	start := time.Now()
	success := false
	defer func() { observeRuntimeObserverSync(ctx, "tekton_manifest", success, time.Since(start)) }()
	log := platformobs.OperationLogger(ctx, "runtime_observer", "sync_tekton_manifest", "manifest")
	var pipelineRuns *tknv1.PipelineRunList
	err := platformobs.ObserveDependency(ctx, platformobs.DependencyCall{
		Kind:      "k8s",
		Target:    "tekton",
		Operation: "list_manifest_pipeline_runs",
	}, func(depCtx context.Context) error {
		var err error
		pipelineRuns, err = o.tekton.TektonV1().PipelineRuns(o.cfg.TektonNamespace).List(depCtx, metav1.ListOptions{
			LabelSelector: tektonManifestSelector(strings.TrimSpace(o.cfg.ControlPlaneID)),
		})
		return err
	})
	if err != nil {
		log.Warn("list tekton pipeline runs failed", zap.Error(err))
		return
	}
	for i := range pipelineRuns.Items {
		pr := &pipelineRuns.Items[i]
		if strings.TrimSpace(pr.Labels[manifestIDLabel]) == "" {
			continue
		}
		if err := o.syncPipelineRun(ctx, pr); err != nil {
			if isNotFoundWriteback(err) {
				continue
			}
			log.Warn("sync tekton pipeline run failed",
				zap.String("pipeline_run", pr.Name),
				zap.String("manifest_id", pr.Labels[manifestIDLabel]),
				zap.Error(err),
			)
		}
	}
	success = true
}

// syncPipelineRun translates one Tekton build pipeline run into manifest writeback callbacks.
// The observer's responsibility ends at build status/task/image-result synchronization; release bundle publication is a later release-service concern.
func (o *TektonManifestObserver) syncPipelineRun(ctx context.Context, pr *tknv1.PipelineRun) error {
	manifestID := strings.TrimSpace(pr.Labels[manifestIDLabel])
	if manifestID == "" {
		return nil
	}
	terminal := isTerminalManifestStatus(mapPipelineRunStatus(pr))
	stateKey := pipelineRunStateKey(pr)
	if terminal && o.isProcessed(pr.Name, stateKey) {
		return nil
	}
	var taskRuns *tknv1.TaskRunList
	err := platformobs.ObserveDependency(ctx, platformobs.DependencyCall{
		Kind:      "k8s",
		Target:    "tekton",
		Operation: "list_manifest_task_runs",
	}, func(depCtx context.Context) error {
		var err error
		taskRuns, err = o.tekton.TektonV1().TaskRuns(pr.Namespace).List(depCtx, metav1.ListOptions{
			LabelSelector: "tekton.dev/pipelineRun=" + pr.Name,
		})
		return err
	})
	if err != nil {
		return err
	}

	for i := range taskRuns.Items {
		if err := o.syncTaskRun(ctx, manifestID, pr.Name, &taskRuns.Items[i]); err != nil {
			if terminal && isNotFoundWriteback(err) {
				return nil
			}
			return err
		}
	}

	statusPayload := map[string]any{
		"manifest_id": manifestID,
		"pipeline_id": pr.Name,
		"status":      mapPipelineRunStatus(pr),
		"message":     pipelineMessage(pr),
	}
	if err := o.postJSON(ctx, statusPayload, manifestTektonStatusPath, manifestTektonStatusLegacyPath); err != nil {
		if terminal && isNotFoundWriteback(err) {
			return nil
		}
		return err
	}

	if result := buildResultPayload(manifestID, pr.Name, taskRuns.Items); result != nil {
		if err := o.postJSON(ctx, result, manifestTektonResultPath, manifestTektonResultLegacyPath); err != nil {
			if terminal && isNotFoundWriteback(err) {
				return nil
			}
			return err
		}
	}
	if terminal {
		o.markProcessed(pr.Name, stateKey)
	}
	return nil
}

func (o *TektonManifestObserver) syncTaskRun(ctx context.Context, manifestID, pipelineID string, tr *tknv1.TaskRun) error {
	start := time.Now()
	status := mapTaskRunStatus(tr)
	success := status != model.StepFailed
	defer func() { observeManifestTask(ctx, taskRunName(tr), string(status), success, time.Since(start)) }()
	payload := map[string]any{
		"manifest_id": manifestID,
		"pipeline_id": pipelineID,
		"task_name":   taskRunName(tr),
		"task_run":    tr.Name,
		"status":      status,
		"message":     taskRunMessage(tr),
	}
	if ts := tr.Status.StartTime; ts != nil {
		payload["start_time"] = ts.Time.UTC().Format(time.RFC3339Nano)
	}
	if ts := tr.Status.CompletionTime; ts != nil {
		payload["end_time"] = ts.Time.UTC().Format(time.RFC3339Nano)
	}
	return o.postJSON(ctx, payload, manifestTektonTasksPath, manifestTektonTasksLegacyPath)
}

func (o *TektonManifestObserver) postJSON(ctx context.Context, payload any, paths ...string) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	ordered := uniqueManifestWritebackPaths(paths)
	var lastErr error
	for idx, path := range ordered {
		err = platformobs.ObserveDependency(ctx, platformobs.DependencyCall{
			Kind:      "http",
			Target:    "release_service",
			Operation: "manifest_tekton_writeback",
		}, func(depCtx context.Context) error {
			req, err := http.NewRequestWithContext(depCtx, http.MethodPost, o.releaseBase+path, bytes.NewReader(body))
			if err != nil {
				return err
			}
			req.Header.Set("Content-Type", "application/json")
			if token := strings.TrimSpace(o.cfg.ObserverToken); token != "" {
				req.Header.Set(manifesthttp.ManifestObserverTokenHeader, token)
			}
			resp, err := o.httpClient.Do(req)
			if err != nil {
				return err
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
			return &writebackError{path: path, statusCode: resp.StatusCode}
		})
		if err == nil {
			return nil
		}
		lastErr = err
		if idx == len(ordered)-1 || !isNotFoundWriteback(err) {
			return err
		}
	}
	return lastErr
}

func uniqueManifestWritebackPaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	ordered := make([]string, 0, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		ordered = append(ordered, path)
	}
	return ordered
}

func tektonManifestSelector(controlPlaneID string) string {
	parts := []string{
		manifestIDLabel,
		observer.ObserveStateLabel + "=" + observer.ObserveStateRunning,
	}
	if strings.TrimSpace(controlPlaneID) != "" {
		parts = append(parts, model.ControlPlaneLabel+"="+strings.TrimSpace(controlPlaneID))
	}
	return strings.Join(parts, ",")
}

func mapPipelineRunStatus(pr *tknv1.PipelineRun) model.ManifestStatus {
	if pr == nil {
		return model.ManifestPending
	}
	cond := pipelineSucceededCondition(pr)
	if cond == nil {
		if pr.Status.StartTime != nil {
			return model.ManifestRunning
		}
		return model.ManifestPending
	}
	switch cond.Status {
	case "True":
		return model.ManifestAvailable
	case "False":
		return model.ManifestUnavailable
	default:
		if pr.Status.StartTime != nil {
			return model.ManifestRunning
		}
		return model.ManifestPending
	}
}

func mapTaskRunStatus(tr *tknv1.TaskRun) model.StepStatus {
	if tr == nil {
		return model.StepPending
	}
	cond := taskRunSucceededCondition(tr)
	if cond == nil {
		if tr.Status.StartTime != nil {
			return model.StepRunning
		}
		return model.StepPending
	}
	switch cond.Status {
	case "True":
		return model.StepSucceeded
	case "False":
		return model.StepFailed
	default:
		if tr.Status.StartTime != nil {
			return model.StepRunning
		}
		return model.StepPending
	}
}

func buildResultPayload(manifestID, pipelineID string, taskRuns []tknv1.TaskRun) map[string]any {
	var (
		commit      string
		imageTag    string
		imageDigest string
		imageRef    string
	)
	for i := range taskRuns {
		tr := &taskRuns[i]
		for _, result := range tr.Status.Results {
			switch result.Name {
			case "commit":
				commit = strings.TrimSpace(result.Value.StringVal)
			case "IMAGE_TAG":
				imageTag = strings.TrimSpace(result.Value.StringVal)
			case "IMAGE_DIGEST":
				imageDigest = strings.TrimSpace(result.Value.StringVal)
			}
		}
		if imageRef == "" {
			imageRef = imageRefFromTaskRun(tr, imageTag, imageDigest)
		}
	}
	if commit == "" && imageRef == "" && imageTag == "" && imageDigest == "" {
		return nil
	}
	payload := map[string]any{
		"manifest_id": manifestID,
		"pipeline_id": pipelineID,
	}
	if commit != "" {
		payload["commit_hash"] = commit
	}
	if imageRef != "" {
		payload["image_ref"] = imageRef
	}
	if imageTag != "" {
		payload["image_tag"] = imageTag
	}
	if imageDigest != "" {
		payload["image_digest"] = imageDigest
	}
	return payload
}

func imageRefFromTaskRun(tr *tknv1.TaskRun, imageTag, imageDigest string) string {
	repository := taskRunParam(tr, "IMAGE_REPOSITORY")
	if repository == "" {
		return ""
	}
	if imageDigest != "" {
		return repository + "@" + imageDigest
	}
	if imageTag == "" {
		imageTag = taskRunParam(tr, "IMAGE_TAG")
	}
	if imageTag != "" {
		return repository + ":" + imageTag
	}
	return ""
}

func taskRunParam(tr *tknv1.TaskRun, name string) string {
	if tr == nil {
		return ""
	}
	for _, param := range tr.Spec.Params {
		if param.Name == name {
			return strings.TrimSpace(param.Value.StringVal)
		}
	}
	return ""
}

func taskRunName(tr *tknv1.TaskRun) string {
	if tr == nil {
		return ""
	}
	if name := strings.TrimSpace(tr.Labels["tekton.dev/pipelineTask"]); name != "" {
		return name
	}
	return tr.Name
}

func pipelineMessage(pr *tknv1.PipelineRun) string {
	if pr == nil {
		return ""
	}
	cond := pipelineSucceededCondition(pr)
	if cond == nil {
		return ""
	}
	return strings.TrimSpace(cond.Message)
}

func taskRunMessage(tr *tknv1.TaskRun) string {
	if tr == nil {
		return ""
	}
	cond := taskRunSucceededCondition(tr)
	if cond == nil {
		return ""
	}
	return strings.TrimSpace(cond.Message)
}

func pipelineSucceededCondition(pr *tknv1.PipelineRun) *apisConditionView {
	if pr == nil {
		return nil
	}
	for i := range pr.Status.Conditions {
		cond := pr.Status.Conditions[i]
		if string(cond.Type) == "Succeeded" {
			return &apisConditionView{Status: string(cond.Status), Message: cond.Message}
		}
	}
	return nil
}

func taskRunSucceededCondition(tr *tknv1.TaskRun) *apisConditionView {
	if tr == nil {
		return nil
	}
	for i := range tr.Status.Conditions {
		cond := tr.Status.Conditions[i]
		if string(cond.Type) == "Succeeded" {
			return &apisConditionView{Status: string(cond.Status), Message: cond.Message}
		}
	}
	return nil
}

type apisConditionView struct {
	Status  string
	Message string
}

type writebackError struct {
	path       string
	statusCode int
}

func (e *writebackError) Error() string {
	return fmt.Sprintf("writeback %s returned %d", e.path, e.statusCode)
}

func isNotFoundWriteback(err error) bool {
	var target *writebackError
	return err != nil && errors.As(err, &target) && target.statusCode == http.StatusNotFound
}

func isTerminalManifestStatus(status model.ManifestStatus) bool {
	switch status {
	case model.ManifestAvailable, model.ManifestUnavailable:
		return true
	default:
		return false
	}
}

func pipelineRunStateKey(pr *tknv1.PipelineRun) string {
	if pr == nil {
		return ""
	}
	if ts := pr.Status.CompletionTime; ts != nil {
		return pr.Name + "|" + ts.UTC().Format(time.RFC3339Nano)
	}
	if rv := strings.TrimSpace(pr.ResourceVersion); rv != "" {
		return pr.Name + "|" + rv
	}
	return pr.Name
}

func (o *TektonManifestObserver) isProcessed(pipelineID, stateKey string) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.processed[pipelineID] == stateKey && stateKey != ""
}

func (o *TektonManifestObserver) markProcessed(pipelineID, stateKey string) {
	if strings.TrimSpace(pipelineID) == "" || strings.TrimSpace(stateKey) == "" {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.processed[pipelineID] = stateKey
}
