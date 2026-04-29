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

	platformdb "github.com/bsonger/devflow-service/internal/platform/db"
	"github.com/bsonger/devflow-service/internal/platform/logger"
	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
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

type activeReleaseSnapshot struct {
	ID            uuid.UUID
	ApplicationID uuid.UUID
	EnvironmentID string
	Strategy      string
	Type          string
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
		store:       repository.NewPostgresStore(),
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
	releases, err := listActiveRollingReleases(ctx)
	if err != nil {
		log.Warn("list active rolling releases failed", zap.Error(err))
		return
	}
	for _, release := range releases {
		if err := o.syncRelease(ctx, release); err != nil {
			log.Warn("sync active rolling release failed",
				zap.String("release_id", release.ID.String()),
				zap.String("application_id", release.ApplicationID.String()),
				zap.String("environment_id", release.EnvironmentID),
				zap.Error(err),
			)
		}
	}
}

func listActiveRollingReleases(ctx context.Context) ([]activeReleaseSnapshot, error) {
	rows, err := platformdb.Postgres().QueryContext(ctx, `
		select id, application_id, env, strategy, type
		from releases
		where deleted_at is null
		  and status in ($1, $2)
		order by created_at desc
	`, releasedomain.ReleaseRunning, releasedomain.ReleaseSyncing)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]activeReleaseSnapshot, 0)
	for rows.Next() {
		var item activeReleaseSnapshot
		if err := rows.Scan(&item.ID, &item.ApplicationID, &item.EnvironmentID, &item.Strategy, &item.Type); err != nil {
			return nil, err
		}
		if releasedomain.NormalizeReleaseStrategy(item.Strategy) != string(releasedomain.ReleaseStrategyRolling) {
			continue
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (o *ReleaseRolloutObserver) syncRelease(ctx context.Context, release activeReleaseSnapshot) error {
	appName, err := o.store.GetApplicationName(ctx, release.ApplicationID)
	if err == sql.ErrNoRows || strings.TrimSpace(appName) == "" {
		return nil
	}
	if err != nil {
		return err
	}
	namespace, err := o.store.ResolveTargetNamespace(ctx, release.ApplicationID, release.EnvironmentID)
	if err == sql.ErrNoRows || strings.TrimSpace(namespace) == "" {
		return nil
	}
	if err != nil {
		return err
	}

	deployments, err := o.clientset.AppsV1().Deployments(strings.TrimSpace(namespace)).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=" + strings.TrimSpace(appName),
	})
	if err != nil {
		return err
	}
	deployment := pickPrimaryDeployment(strings.TrimSpace(appName), deployments.Items)
	phase, message, progress, stateKey := deriveReleaseRolloutState(strings.TrimSpace(namespace), strings.TrimSpace(appName), deployment)
	if o.isProcessed(release.ID.String(), stateKey) {
		return nil
	}
	switch phase {
	case releasedomain.StepSucceeded:
		if err := o.postStep(ctx, release.ID, "start_deployment", releasedomain.StepSucceeded, 100, message); err != nil {
			return err
		}
		if err := o.postStep(ctx, release.ID, "observe_rollout", releasedomain.StepSucceeded, 100, message); err != nil {
			return err
		}
		if err := o.postStep(ctx, release.ID, "finalize_release", releasedomain.StepSucceeded, 100, "release finalized after deployment became healthy"); err != nil {
			return err
		}
	case releasedomain.StepFailed:
		if err := o.postStep(ctx, release.ID, "start_deployment", releasedomain.StepFailed, 100, message); err != nil {
			return err
		}
		if err := o.postStep(ctx, release.ID, "observe_rollout", releasedomain.StepFailed, 100, message); err != nil {
			return err
		}
		if err := o.postStep(ctx, release.ID, "finalize_release", releasedomain.StepFailed, 100, "release finalized after deployment failure"); err != nil {
			return err
		}
	default:
		if err := o.postStep(ctx, release.ID, "start_deployment", releasedomain.StepRunning, progress, message); err != nil {
			return err
		}
		if err := o.postStep(ctx, release.ID, "observe_rollout", releasedomain.StepRunning, progress, message); err != nil {
			return err
		}
	}
	o.markProcessed(release.ID.String(), stateKey)
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
