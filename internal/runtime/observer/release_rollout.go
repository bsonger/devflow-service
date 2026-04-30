package observer

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bsonger/devflow-service/internal/platform/logger"
	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
	runtimedomain "github.com/bsonger/devflow-service/internal/runtime/domain"
	"github.com/bsonger/devflow-service/internal/runtime/repository"
	"github.com/google/uuid"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const releaseObserverTokenHeader = "X-Devflow-Observer-Token"

type ReleaseRolloutObserverConfig struct {
	Enabled               bool
	PollInterval          time.Duration
	ReleaseServiceBaseURL string
	ObserverToken         string
	HTTPTimeout           time.Duration
}

type releaseRolloutContext struct {
	ReleaseID      uuid.UUID
	ApplicationID  uuid.UUID
	EnvironmentID  string
	Namespace      string
	DeploymentName string
}

type ReleaseRolloutObserver struct {
	cfg         ReleaseRolloutObserverConfig
	clientset   kubernetes.Interface
	httpClient  *http.Client
	releaseBase string
	store       repository.Store
	mu          sync.Mutex
	processed   map[string]string
}

func StartReleaseRolloutObserver(ctx context.Context, restCfg *rest.Config, cfg ReleaseRolloutObserverConfig) error {
	cfg.ReleaseServiceBaseURL = strings.TrimRight(strings.TrimSpace(cfg.ReleaseServiceBaseURL), "/")
	if !cfg.Enabled || cfg.ReleaseServiceBaseURL == "" {
		return nil
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = defaultObserverInterval
	}
	if cfg.HTTPTimeout <= 0 {
		cfg.HTTPTimeout = 10 * time.Second
	}
	clientset, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return err
	}
	observer := &ReleaseRolloutObserver{
		cfg:         cfg,
		clientset:   clientset,
		httpClient:  &http.Client{Timeout: cfg.HTTPTimeout},
		releaseBase: cfg.ReleaseServiceBaseURL,
		store:       repository.RuntimeStore,
		processed:   map[string]string{},
	}
	go observer.run(ctx)
	return nil
}

func (o *ReleaseRolloutObserver) run(ctx context.Context) {
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

func (o *ReleaseRolloutObserver) sync(ctx context.Context) {
	log := logger.LoggerWithContext(ctx)
	if log == nil {
		log = zap.NewNop()
	}

	specs, err := o.store.ListRuntimeSpecs(ctx)
	if err != nil {
		log.Warn("list runtime specs for release rollout observer failed", zap.Error(err))
		return
	}
	for _, spec := range specs {
		if spec == nil {
			continue
		}
		if err := o.syncRuntimeSpec(ctx, spec, log); err != nil {
			log.Warn("sync rollout observer from runtime state failed",
				zap.String("runtime_spec_id", spec.ID.String()),
				zap.String("application_id", spec.ApplicationID.String()),
				zap.String("environment_id", spec.Environment),
				zap.Error(err),
			)
		}
	}
}

func (o *ReleaseRolloutObserver) syncRuntimeSpec(ctx context.Context, spec *runtimedomain.RuntimeSpec, log *zap.Logger) error {
	workload, err := o.store.GetObservedWorkload(ctx, spec.ID)
	if err == sql.ErrNoRows || workload == nil {
		log.Debug("skip release rollout observer because runtime workload state is missing",
			zap.String("runtime_spec_id", spec.ID.String()),
			zap.String("application_id", spec.ApplicationID.String()),
			zap.String("environment_id", spec.Environment),
		)
		return nil
	}
	if err != nil {
		return err
	}
	rollout, skipReason := deriveReleaseRolloutContext(workload)
	if skipReason != "" {
		log.Debug("skip release rollout observer because release metadata is incomplete",
			zap.String("runtime_spec_id", spec.ID.String()),
			zap.String("namespace", strings.TrimSpace(workload.Namespace)),
			zap.String("workload_name", strings.TrimSpace(workload.WorkloadName)),
			zap.String("reason", skipReason),
		)
		return nil
	}

	deployment, err := o.lookupDeployment(ctx, rollout)
	if err != nil {
		return err
	}
	phase, message, progress, stateKey := deriveReleaseRolloutState(rollout.Namespace, rollout.DeploymentName, deployment)
	if o.isProcessed(rollout.ReleaseID.String(), stateKey) {
		log.Debug("skip duplicate rollout writeback event",
			zap.String("release_id", rollout.ReleaseID.String()),
			zap.String("deployment", rollout.DeploymentName),
			zap.String("namespace", rollout.Namespace),
			zap.String("state_key", stateKey),
		)
		return nil
	}
	if err := o.writeReleaseSteps(ctx, rollout, phase, progress, message); err != nil {
		log.Warn("release rollout writeback failed",
			zap.String("release_id", rollout.ReleaseID.String()),
			zap.String("deployment", rollout.DeploymentName),
			zap.String("namespace", rollout.Namespace),
			zap.String("phase", string(phase)),
			zap.Error(err),
		)
		return err
	}
	log.Debug("release rollout state emitted",
		zap.String("release_id", rollout.ReleaseID.String()),
		zap.String("application_id", rollout.ApplicationID.String()),
		zap.String("environment_id", rollout.EnvironmentID),
		zap.String("deployment", rollout.DeploymentName),
		zap.String("namespace", rollout.Namespace),
		zap.String("phase", string(phase)),
		zap.Int32("progress", progress),
		zap.String("state_key", stateKey),
	)
	o.markProcessed(rollout.ReleaseID.String(), stateKey)
	return nil
}

func deriveReleaseRolloutContext(workload *runtimedomain.RuntimeObservedWorkload) (releaseRolloutContext, string) {
	if workload == nil {
		return releaseRolloutContext{}, "missing_workload"
	}
	releaseID, err := uuid.Parse(strings.TrimSpace(workload.Labels[releasedomain.ReleaseIDLabel]))
	if err != nil || releaseID == uuid.Nil {
		return releaseRolloutContext{}, "missing_release_id_label"
	}
	applicationID, err := uuid.Parse(strings.TrimSpace(workload.Labels[releasedomain.ReleaseApplicationLabel]))
	if err != nil || applicationID == uuid.Nil {
		return releaseRolloutContext{}, "missing_application_id_label"
	}
	environmentID := strings.TrimSpace(workload.Labels[releasedomain.ReleaseEnvironmentLabel])
	if environmentID == "" {
		environmentID = strings.TrimSpace(workload.Environment)
	}
	if environmentID == "" {
		return releaseRolloutContext{}, "missing_environment_id_label"
	}
	namespace := strings.TrimSpace(workload.Namespace)
	if namespace == "" {
		return releaseRolloutContext{}, "missing_namespace"
	}
	deploymentName := strings.TrimSpace(workload.WorkloadName)
	if deploymentName == "" {
		deploymentName = strings.TrimSpace(workload.Labels["app.kubernetes.io/name"])
	}
	if deploymentName == "" {
		return releaseRolloutContext{}, "missing_deployment_name"
	}
	return releaseRolloutContext{
		ReleaseID:      releaseID,
		ApplicationID:  applicationID,
		EnvironmentID:  environmentID,
		Namespace:      namespace,
		DeploymentName: deploymentName,
	}, ""
}

func (o *ReleaseRolloutObserver) lookupDeployment(ctx context.Context, rollout releaseRolloutContext) (*appsv1.Deployment, error) {
	deployments, err := o.clientset.AppsV1().Deployments(rollout.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: releasedomain.ReleaseIDLabel + "=" + rollout.ReleaseID.String(),
	})
	if err != nil {
		return nil, err
	}
	deployment := pickPrimaryDeployment(rollout.DeploymentName, deployments.Items)
	if deployment != nil {
		return deployment, nil
	}
	return nil, nil
}

func (o *ReleaseRolloutObserver) writeReleaseSteps(ctx context.Context, rollout releaseRolloutContext, phase releasedomain.StepStatus, progress int32, message string) error {
	switch phase {
	case releasedomain.StepSucceeded:
		if err := o.postStep(ctx, rollout.ReleaseID, "observe_rollout", releasedomain.StepSucceeded, 100, message); err != nil {
			return err
		}
		if err := o.postStep(ctx, rollout.ReleaseID, "finalize_release", releasedomain.StepSucceeded, 100, "release finalized after deployment became healthy"); err != nil {
			return err
		}
	case releasedomain.StepFailed:
		if err := o.postStep(ctx, rollout.ReleaseID, "observe_rollout", releasedomain.StepFailed, 100, message); err != nil {
			return err
		}
		if err := o.postStep(ctx, rollout.ReleaseID, "finalize_release", releasedomain.StepFailed, 100, "release finalized after deployment failure"); err != nil {
			return err
		}
	default:
		if err := o.postStep(ctx, rollout.ReleaseID, "observe_rollout", releasedomain.StepRunning, progress, message); err != nil {
			return err
		}
	}
	return nil
}

func pickPrimaryDeployment(appName string, items []appsv1.Deployment) *appsv1.Deployment {
	if len(items) == 0 {
		return nil
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Name == appName {
			return true
		}
		if items[j].Name == appName {
			return false
		}
		return items[i].Name < items[j].Name
	})
	return &items[0]
}

func deriveReleaseRolloutState(namespace, appName string, deployment *appsv1.Deployment) (releasedomain.StepStatus, string, int32, string) {
	if deployment == nil {
		message := fmt.Sprintf("waiting for deployment %s in namespace %s", firstNonEmptyString(appName, "application"), firstNonEmptyString(namespace, "unknown"))
		return releasedomain.StepRunning, message, 10, "missing"
	}
	desired := int32Value(deployment.Spec.Replicas)
	updated := int(deployment.Status.UpdatedReplicas)
	ready := int(deployment.Status.ReadyReplicas)
	available := int(deployment.Status.AvailableReplicas)
	unavailable := int(deployment.Status.UnavailableReplicas)
	generationObserved := deployment.Status.ObservedGeneration >= deployment.Generation
	progressingReason, progressingStatus := deploymentConditionSummary(deployment.Status.Conditions, appsv1.DeploymentProgressing)
	replicaFailureReason, replicaFailureStatus := deploymentConditionSummary(deployment.Status.Conditions, appsv1.DeploymentReplicaFailure)

	if progressingReason == "ProgressDeadlineExceeded" || replicaFailureStatus == "True" {
		message := fmt.Sprintf("deployment failed (progressing_reason=%s, replica_failure_reason=%s, ready=%d/%d, updated=%d/%d)", firstNonEmptyString(progressingReason, "unknown"), firstNonEmptyString(replicaFailureReason, "unknown"), ready, desired, updated, desired)
		return releasedomain.StepFailed, message, 100, "failed|" + progressingReason + "|" + replicaFailureReason
	}

	if desired > 0 && generationObserved && updated >= desired && ready >= desired && available >= desired && unavailable == 0 {
		message := fmt.Sprintf("deployment healthy (ready=%d/%d, updated=%d/%d, available=%d/%d)", ready, desired, updated, desired, available, desired)
		return releasedomain.StepSucceeded, message, 100, fmt.Sprintf("succeeded|%d|%d|%d|%d", desired, updated, ready, available)
	}

	progress := int32(25)
	if desired > 0 {
		candidate := int32((available * 100) / desired)
		if candidate > progress {
			progress = candidate
		}
	}
	if progress > 99 {
		progress = 99
	}
	message := fmt.Sprintf("deployment progressing (ready=%d/%d, updated=%d/%d, available=%d/%d, unavailable=%d, observed_generation=%t, progressing_status=%s)", ready, desired, updated, desired, available, desired, unavailable, generationObserved, firstNonEmptyString(progressingStatus, "Unknown"))
	return releasedomain.StepRunning, message, progress, fmt.Sprintf("running|%d|%d|%d|%d|%d|%t|%s|%s", desired, updated, ready, available, unavailable, generationObserved, progressingStatus, progressingReason)
}

func deploymentConditionSummary(conditions []appsv1.DeploymentCondition, conditionType appsv1.DeploymentConditionType) (string, string) {
	for _, condition := range conditions {
		if condition.Type != conditionType {
			continue
		}
		return strings.TrimSpace(condition.Reason), string(condition.Status)
	}
	return "", ""
}

func (o *ReleaseRolloutObserver) postStep(ctx context.Context, releaseID uuid.UUID, stepCode string, status releasedomain.StepStatus, progress int32, message string) error {
	payload := map[string]any{
		"release_id": releaseID.String(),
		"step_code":  strings.TrimSpace(stepCode),
		"status":     string(status),
		"progress":   progress,
		"message":    strings.TrimSpace(message),
	}
	return o.postJSON(ctx, "/api/v1/verify/release/steps", payload)
}

func (o *ReleaseRolloutObserver) postJSON(ctx context.Context, path string, payload any) error {
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
		req.Header.Set(releaseObserverTokenHeader, token)
	}
	resp, err := o.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("release rollout writeback failed: path=%s status=%d", path, resp.StatusCode)
}

func (o *ReleaseRolloutObserver) isProcessed(releaseID, stateKey string) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.processed[releaseID] == stateKey
}

func (o *ReleaseRolloutObserver) markProcessed(releaseID, stateKey string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.processed[releaseID] = stateKey
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
