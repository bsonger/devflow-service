package observer

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bsonger/devflow-service/internal/platform/observer"
	platformobs "github.com/bsonger/devflow-service/internal/platform/runtime/observability"
	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
	releasedownstream "github.com/bsonger/devflow-service/internal/release/transport/downstream"
	runtimedomain "github.com/bsonger/devflow-service/internal/runtime/domain"
	runtimereconcile "github.com/bsonger/devflow-service/internal/runtime/reconcile"
	"github.com/bsonger/devflow-service/internal/runtime/repository"
	"github.com/bsonger/devflow-service/internal/runtime/writeback"
	"github.com/google/uuid"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const releaseMetricsPortName = "metrics"

var releaseRolloutGVR = schema.GroupVersionResource{
	Group:    "argoproj.io",
	Version:  "v1alpha1",
	Resource: "rollouts",
}

var releaseMetricsEndpointProber = releasedownstream.ProbeMetricsEndpoint

type ReleaseRolloutObserverConfig struct {
	Enabled               bool
	PollInterval          time.Duration
	ReleaseServiceBaseURL string
	ObserverToken         string
	HTTPTimeout           time.Duration
	ControlPlaneID        string
}

type releaseRolloutContext struct {
	ReleaseID            uuid.UUID
	ApplicationID        uuid.UUID
	EnvironmentID        string
	Namespace            string
	PrimaryWorkloadName  string
	ObservedWorkloadKind string
	ObservedWorkloadName string
}

type ReleaseRolloutObserver struct {
	cfg         ReleaseRolloutObserverConfig
	clientset   kubernetes.Interface
	dynamic     dynamic.Interface
	httpClient  *http.Client
	releaseBase string
	store       repository.Store
	mu          sync.Mutex
	processed   map[string]string
}

type releaseStepWrite = runtimereconcile.ReleaseStepWrite
type releaseObservedState = runtimereconcile.NormalizedReleaseObservedState

func isReleaseRolloutWritebackNotFound(err error) bool {
	var target *writeback.WritebackError
	return errors.As(err, &target) && target.NotFound()
}

func StartReleaseRolloutObserver(ctx context.Context, restCfg *rest.Config, cfg ReleaseRolloutObserverConfig) error {
	cfg.ReleaseServiceBaseURL = strings.TrimRight(strings.TrimSpace(cfg.ReleaseServiceBaseURL), "/")
	if !cfg.Enabled || cfg.ReleaseServiceBaseURL == "" {
		return nil
	}
	// Legacy polling path retained as a migration fallback while queue-driven release
	// reconcile remains feature-flagged and is not yet the default execution lane.
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
	dynamicClient, err := dynamic.NewForConfig(restCfg)
	if err != nil {
		return err
	}
	observer := &ReleaseRolloutObserver{
		cfg:         cfg,
		clientset:   clientset,
		dynamic:     dynamicClient,
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
	start := time.Now()
	success := false
	defer func() { observeRuntimeObserverSync(ctx, "release_rollout", success, time.Since(start)) }()
	log := platformobs.OperationLogger(ctx, "runtime_observer", "sync_release_rollout", "runtime")

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
	success = true
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
			zap.String("workload_kind", strings.TrimSpace(workload.WorkloadKind)),
			zap.String("workload_name", strings.TrimSpace(workload.WorkloadName)),
			zap.String("reason", skipReason),
		)
		return nil
	}
	if !releaseRolloutOwnedByControlPlane(workload, strings.TrimSpace(o.cfg.ControlPlaneID)) {
		log.Debug("skip release rollout observer because workload does not belong to current control plane",
			zap.String("runtime_spec_id", spec.ID.String()),
			zap.String("namespace", strings.TrimSpace(workload.Namespace)),
			zap.String("workload_kind", strings.TrimSpace(workload.WorkloadKind)),
			zap.String("workload_name", strings.TrimSpace(workload.WorkloadName)),
			zap.String("control_plane_id", strings.TrimSpace(o.cfg.ControlPlaneID)),
		)
		return nil
	}

	state, err := o.lookupObservedState(ctx, rollout)
	if err != nil {
		return err
	}
	if state == nil {
		state = &releaseObservedState{
			Phase:    releasedomain.StepRunning,
			Progress: 10,
			Message:  fmt.Sprintf("waiting for workload %s in namespace %s", firstNonEmptyString(rollout.ObservedWorkloadName, "application"), firstNonEmptyString(rollout.Namespace, "unknown")),
			StateKey: "missing",
			StepWrites: []releaseStepWrite{{
				StepCode: "observe_rollout",
				Status:   releasedomain.StepRunning,
				Progress: 10,
				Message:  fmt.Sprintf("waiting for workload %s in namespace %s", firstNonEmptyString(rollout.ObservedWorkloadName, "application"), firstNonEmptyString(rollout.Namespace, "unknown")),
			}},
		}
	}
	if o.isProcessed(rollout.ReleaseID.String(), state.StateKey) {
		log.Debug("skip duplicate rollout writeback event",
			zap.String("release_id", rollout.ReleaseID.String()),
			zap.String("observed_workload_kind", rollout.ObservedWorkloadKind),
			zap.String("observed_workload_name", rollout.ObservedWorkloadName),
			zap.String("namespace", rollout.Namespace),
			zap.String("state_key", state.StateKey),
		)
		return nil
	}
	if err := o.writeReleaseSteps(ctx, rollout, state); err != nil {
		if isReleaseRolloutWritebackNotFound(err) {
			log.Warn("skip stale release rollout writeback because release was not found",
				zap.String("release_id", rollout.ReleaseID.String()),
				zap.String("application_id", rollout.ApplicationID.String()),
				zap.String("environment_id", rollout.EnvironmentID),
				zap.String("observed_workload_kind", rollout.ObservedWorkloadKind),
				zap.String("observed_workload_name", rollout.ObservedWorkloadName),
				zap.String("namespace", rollout.Namespace),
				zap.String("phase", string(state.Phase)),
				zap.String("state_key", state.StateKey),
			)
			o.markProcessed(rollout.ReleaseID.String(), state.StateKey)
			return nil
		}
		log.Warn("release rollout writeback failed",
			zap.String("release_id", rollout.ReleaseID.String()),
			zap.String("observed_workload_kind", rollout.ObservedWorkloadKind),
			zap.String("observed_workload_name", rollout.ObservedWorkloadName),
			zap.String("namespace", rollout.Namespace),
			zap.String("phase", string(state.Phase)),
			zap.Error(err),
		)
		return err
	}
	log.Debug("release rollout state emitted",
		zap.String("release_id", rollout.ReleaseID.String()),
		zap.String("application_id", rollout.ApplicationID.String()),
		zap.String("environment_id", rollout.EnvironmentID),
		zap.String("primary_workload_name", rollout.PrimaryWorkloadName),
		zap.String("observed_workload_kind", rollout.ObservedWorkloadKind),
		zap.String("observed_workload_name", rollout.ObservedWorkloadName),
		zap.String("namespace", rollout.Namespace),
		zap.String("phase", string(state.Phase)),
		zap.Int32("progress", state.Progress),
		zap.String("state_key", state.StateKey),
	)
	o.markProcessed(rollout.ReleaseID.String(), state.StateKey)
	return nil
}

func releaseRolloutOwnedByControlPlane(workload *runtimedomain.RuntimeObservedWorkload, controlPlaneID string) bool {
	if strings.TrimSpace(controlPlaneID) == "" {
		return true
	}
	if workload == nil {
		return false
	}
	return strings.TrimSpace(workload.Labels[releasedomain.ControlPlaneLabel]) == strings.TrimSpace(controlPlaneID)
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
	primaryWorkloadName := strings.TrimSpace(workload.Labels["app.kubernetes.io/name"])
	if primaryWorkloadName == "" {
		primaryWorkloadName = strings.TrimSpace(workload.WorkloadName)
	}
	if primaryWorkloadName == "" {
		return releaseRolloutContext{}, "missing_primary_workload_name"
	}
	observedWorkloadKind := strings.TrimSpace(workload.WorkloadKind)
	if observedWorkloadKind == "" {
		observedWorkloadKind = "Deployment"
	}
	observedWorkloadName := strings.TrimSpace(workload.WorkloadName)
	if observedWorkloadName == "" {
		observedWorkloadName = primaryWorkloadName
	}
	if observedWorkloadName == "" {
		return releaseRolloutContext{}, "missing_observed_workload_name"
	}
	return releaseRolloutContext{
		ReleaseID:            releaseID,
		ApplicationID:        applicationID,
		EnvironmentID:        environmentID,
		Namespace:            namespace,
		PrimaryWorkloadName:  primaryWorkloadName,
		ObservedWorkloadKind: observedWorkloadKind,
		ObservedWorkloadName: observedWorkloadName,
	}, ""
}

func (o *ReleaseRolloutObserver) lookupDeployment(ctx context.Context, rollout releaseRolloutContext) (*appsv1.Deployment, error) {
	var deployments *appsv1.DeploymentList
	err := platformobs.ObserveDependency(ctx, platformobs.DependencyCall{
		Kind:      "k8s",
		Target:    "kubernetes",
		Operation: "list_release_deployments",
	}, func(depCtx context.Context) error {
		var err error
		deployments, err = o.clientset.AppsV1().Deployments(rollout.Namespace).List(depCtx, metav1.ListOptions{
			LabelSelector: releasedomain.ReleaseIDLabel + "=" + rollout.ReleaseID.String() + "," + observer.ObserveStateLabel + "=" + observer.ObserveStateRunning,
		})
		return err
	})
	if err != nil {
		return nil, err
	}
	deployment := pickPrimaryDeployment(rollout.PrimaryWorkloadName, deployments.Items)
	if deployment != nil {
		return deployment, nil
	}
	return nil, nil
}

func (o *ReleaseRolloutObserver) lookupObservedState(ctx context.Context, rollout releaseRolloutContext) (*releaseObservedState, error) {
	switch strings.ToLower(strings.TrimSpace(rollout.ObservedWorkloadKind)) {
	case "rollout":
		return o.lookupRolloutState(ctx, rollout)
	default:
		deployment, err := o.lookupDeployment(ctx, rollout)
		if err != nil {
			return nil, err
		}
		state := runtimereconcile.NormalizeReleaseObservedStateFromDeployment(rollout.Namespace, rollout.ObservedWorkloadName, deployment)
		return &state, nil
	}
}

func (o *ReleaseRolloutObserver) lookupRolloutState(ctx context.Context, rollout releaseRolloutContext) (*releaseObservedState, error) {
	if o == nil || o.dynamic == nil {
		message := fmt.Sprintf("waiting for rollout %s in namespace %s", firstNonEmptyString(rollout.ObservedWorkloadName, rollout.PrimaryWorkloadName, "application"), firstNonEmptyString(rollout.Namespace, "unknown"))
		return &releaseObservedState{
			Phase:    releasedomain.StepRunning,
			Progress: 10,
			Message:  message,
			StateKey: "rollout|missing_client",
			StepWrites: []releaseStepWrite{{
				StepCode: "deploy_canary",
				Status:   releasedomain.StepRunning,
				Progress: 10,
				Message:  message,
			}},
		}, nil
	}
	var rollouts *unstructured.UnstructuredList
	err := platformobs.ObserveDependency(ctx, platformobs.DependencyCall{
		Kind:      "k8s",
		Target:    "argo_rollouts",
		Operation: "list_release_rollouts",
	}, func(depCtx context.Context) error {
		var err error
		rollouts, err = o.dynamic.Resource(releaseRolloutGVR).Namespace(rollout.Namespace).List(depCtx, metav1.ListOptions{
			LabelSelector: releasedomain.ReleaseIDLabel + "=" + rollout.ReleaseID.String(),
		})
		return err
	})
	if err != nil {
		return nil, err
	}
	item := pickPrimaryRollout(rollout.ObservedWorkloadName, rollout.PrimaryWorkloadName, rollouts.Items)
	state := runtimereconcile.NormalizeReleaseObservedStateFromRollout(rollout.Namespace, rollout.ObservedWorkloadName, rollout.PrimaryWorkloadName, item)
	return &state, nil
}

func (o *ReleaseRolloutObserver) writeReleaseSteps(ctx context.Context, rollout releaseRolloutContext, state *releaseObservedState) error {
	if state == nil {
		return nil
	}
	writer := writeback.NewReleaseWriter(o.releaseBase, o.cfg.ObserverToken, o.httpClient)
	input := writeback.WriteReleaseStepsInput{
		ReleaseID:            rollout.ReleaseID,
		ApplicationID:        rollout.ApplicationID,
		EnvironmentID:        rollout.EnvironmentID,
		Namespace:            rollout.Namespace,
		ObservedWorkloadKind: rollout.ObservedWorkloadKind,
		ObservedWorkloadName: rollout.ObservedWorkloadName,
		Phase:                state.Phase,
		Progress:             state.Progress,
		Message:              state.Message,
		StepWrites:           make([]writeback.ReleaseStepWrite, 0, len(state.StepWrites)+1),
	}
	for _, step := range state.StepWrites {
		input.StepWrites = append(input.StepWrites, writeback.ReleaseStepWrite{
			StepCode: step.StepCode,
			Status:   step.Status,
			Progress: step.Progress,
			Message:  step.Message,
		})
	}
	if state.FinalizeState == nil {
		return writer.WriteReleaseSteps(ctx, input)
	}
	finalize := *state.FinalizeState
	if finalize.Status == releasedomain.StepSucceeded {
		if err := o.verifyMetricsEndpoint(ctx, rollout); err != nil {
			finalize.Status = releasedomain.StepFailed
			finalize.Message = fmt.Sprintf("metrics endpoint verification failed: %v", err)
		}
	}
	input.StepWrites = append(input.StepWrites, writeback.ReleaseStepWrite{
		StepCode: finalize.StepCode,
		Status:   finalize.Status,
		Progress: finalize.Progress,
		Message:  finalize.Message,
	})
	return writer.WriteReleaseSteps(ctx, input)
}

func pickPrimaryRollout(observedName, primaryName string, items []unstructured.Unstructured) *unstructured.Unstructured {
	if len(items) == 0 {
		return nil
	}
	sort.SliceStable(items, func(i, j int) bool {
		left := strings.TrimSpace(items[i].GetName())
		right := strings.TrimSpace(items[j].GetName())
		switch {
		case left == strings.TrimSpace(observedName):
			return true
		case right == strings.TrimSpace(observedName):
			return false
		case left == strings.TrimSpace(primaryName):
			return true
		case right == strings.TrimSpace(primaryName):
			return false
		default:
			return left < right
		}
	})
	return &items[0]
}

func deriveReleaseObservedStateFromRollout(namespace, observedName, primaryName string, rollout *unstructured.Unstructured) *releaseObservedState {
	state := runtimereconcile.NormalizeReleaseObservedStateFromRollout(namespace, observedName, primaryName, rollout)
	return &state
}

func (o *ReleaseRolloutObserver) verifyMetricsEndpoint(ctx context.Context, rollout releaseRolloutContext) error {
	if o == nil || o.clientset == nil {
		return nil
	}
	serviceName := strings.TrimSpace(rollout.PrimaryWorkloadName)
	namespace := strings.TrimSpace(rollout.Namespace)
	if serviceName == "" || namespace == "" {
		return nil
	}
	var service *corev1.Service
	err := platformobs.ObserveDependency(ctx, platformobs.DependencyCall{
		Kind:      "k8s",
		Target:    "kubernetes",
		Operation: "get_metrics_service",
	}, func(depCtx context.Context) error {
		var err error
		service, err = o.clientset.CoreV1().Services(namespace).Get(depCtx, serviceName, metav1.GetOptions{})
		return err
	})
	if err != nil {
		return nil
	}
	port, ok := findMetricsServicePort(service)
	if !ok || port <= 0 {
		return nil
	}
	url := fmt.Sprintf("http://%s.%s.svc.cluster.local:%d/metrics", serviceName, namespace, port)
	return releaseMetricsEndpointProber(ctx, url)
}

func findMetricsServicePort(service *corev1.Service) (int32, bool) {
	if service == nil {
		return 0, false
	}
	for _, port := range service.Spec.Ports {
		if strings.TrimSpace(port.Name) == releaseMetricsPortName && port.Port > 0 {
			return port.Port, true
		}
	}
	return 0, false
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
	state := runtimereconcile.NormalizeReleaseObservedStateFromDeployment(namespace, appName, deployment)
	return state.Phase, state.Message, state.Progress, state.StateKey
}

func nestedString(obj map[string]any, fields ...string) string {
	value, _, _ := unstructured.NestedString(obj, fields...)
	return strings.TrimSpace(value)
}

func nestedInt64(obj map[string]any, fields ...string) (int64, bool) {
	value, ok, _ := unstructured.NestedInt64(obj, fields...)
	return value, ok
}

func nestedInt64Default(obj map[string]any, fallback int64, fields ...string) int64 {
	value, ok := nestedInt64(obj, fields...)
	if !ok {
		return fallback
	}
	return value
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
	writer := writeback.NewReleaseWriter(o.releaseBase, o.cfg.ObserverToken, o.httpClient)
	return writer.PostJSON(ctx, path, payload)
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
