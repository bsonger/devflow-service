package observer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	manifesthttp "github.com/bsonger/devflow-service/internal/manifest/transport/http"
	"github.com/bsonger/devflow-service/internal/platform/logger"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	tknv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

const (
	defaultTektonNamespace  = "tekton-pipelines"
	defaultObserverInterval = 15 * time.Second
	manifestIDLabel         = "devflow.manifest/id"
)

type TektonManifestObserverConfig struct {
	Enabled               bool
	TektonNamespace       string
	PollInterval          time.Duration
	ReleaseServiceBaseURL string
	ObserverToken         string
	HTTPTimeout           time.Duration
}

type TektonManifestObserver struct {
	cfg         TektonManifestObserverConfig
	tekton      *tektonclient.Clientset
	httpClient  *http.Client
	releaseBase string
}

func StartTektonManifestObserver(ctx context.Context, restCfg *rest.Config, cfg TektonManifestObserverConfig) error {
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

func (o *TektonManifestObserver) sync(ctx context.Context) {
	log := logger.LoggerWithContext(ctx)
	if log == nil {
		log = zap.NewNop()
	}
	pipelineRuns, err := o.tekton.TektonV1().PipelineRuns(o.cfg.TektonNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: manifestIDLabel,
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
			log.Warn("sync tekton pipeline run failed",
				zap.String("pipeline_run", pr.Name),
				zap.String("manifest_id", pr.Labels[manifestIDLabel]),
				zap.Error(err),
			)
		}
	}
}

func (o *TektonManifestObserver) syncPipelineRun(ctx context.Context, pr *tknv1.PipelineRun) error {
	manifestID := strings.TrimSpace(pr.Labels[manifestIDLabel])
	if manifestID == "" {
		return nil
	}
	taskRuns, err := o.tekton.TektonV1().TaskRuns(pr.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "tekton.dev/pipelineRun=" + pr.Name,
	})
	if err != nil {
		return err
	}

	for i := range taskRuns.Items {
		if err := o.syncTaskRun(ctx, manifestID, pr.Name, &taskRuns.Items[i]); err != nil {
			return err
		}
	}

	statusPayload := map[string]any{
		"manifest_id": manifestID,
		"pipeline_id": pr.Name,
		"status":      mapPipelineRunStatus(pr),
		"message":     pipelineMessage(pr),
	}
	if err := o.postJSON(ctx, "/api/v1/manifests/tekton/status", statusPayload); err != nil {
		return err
	}

	if result := buildResultPayload(manifestID, pr.Name, taskRuns.Items); result != nil {
		if err := o.postJSON(ctx, "/api/v1/manifests/tekton/result", result); err != nil {
			return err
		}
	}
	return nil
}

func (o *TektonManifestObserver) syncTaskRun(ctx context.Context, manifestID, pipelineID string, tr *tknv1.TaskRun) error {
	payload := map[string]any{
		"manifest_id": manifestID,
		"pipeline_id": pipelineID,
		"task_name":   taskRunName(tr),
		"task_run":    tr.Name,
		"status":      mapTaskRunStatus(tr),
		"message":     taskRunMessage(tr),
	}
	if ts := tr.Status.StartTime; ts != nil {
		payload["start_time"] = ts.Time.UTC().Format(time.RFC3339Nano)
	}
	if ts := tr.Status.CompletionTime; ts != nil {
		payload["end_time"] = ts.Time.UTC().Format(time.RFC3339Nano)
	}
	return o.postJSON(ctx, "/api/v1/manifests/tekton/tasks", payload)
}

func (o *TektonManifestObserver) postJSON(ctx context.Context, path string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.releaseBase+path, bytes.NewReader(body))
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
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("writeback %s returned %d", path, resp.StatusCode)
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
	switch {
	case cond.Status == "True":
		return model.ManifestReady
	case cond.Status == "False":
		return model.ManifestFailed
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
	switch {
	case cond.Status == "True":
		return model.StepSucceeded
	case cond.Status == "False":
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
