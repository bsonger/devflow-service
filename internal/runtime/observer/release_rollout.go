package observer

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bsonger/devflow-service/internal/platform/logger"
	"github.com/bsonger/devflow-service/internal/platform/observer"
	platformobs "github.com/bsonger/devflow-service/internal/platform/runtime/observability"
	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
	releasedownstream "github.com/bsonger/devflow-service/internal/release/transport/downstream"
	runtimedomain "github.com/bsonger/devflow-service/internal/runtime/domain"
	"github.com/bsonger/devflow-service/internal/runtime/repository"
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

const releaseObserverTokenHeader = "X-Devflow-Observer-Token"
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

type releaseStepWrite struct {
	StepCode string
	Status   releasedomain.StepStatus
	Progress int32
	Message  string
}

type releaseObservedState struct {
	Phase         releasedomain.StepStatus
	Progress      int32
	Message       string
	StateKey      string
	StepWrites    []releaseStepWrite
	FinalizeState *releaseStepWrite
}

type releaseRolloutWritebackError struct {
	Path       string
	StatusCode int
}

func (e *releaseRolloutWritebackError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("release rollout writeback failed: path=%s status=%d", e.Path, e.StatusCode)
}

func (e *releaseRolloutWritebackError) NotFound() bool {
	return e != nil && e.StatusCode == http.StatusNotFound
}

func isReleaseRolloutWritebackNotFound(err error) bool {
	var target *releaseRolloutWritebackError
	return errors.As(err, &target) && target.NotFound()
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
		phase, message, progress, stateKey := deriveReleaseRolloutState(rollout.Namespace, rollout.ObservedWorkloadName, deployment)
		state := &releaseObservedState{
			Phase:    phase,
			Progress: progress,
			Message:  message,
			StateKey: stateKey,
			StepWrites: []releaseStepWrite{{
				StepCode: "observe_rollout",
				Status:   phase,
				Progress: progress,
				Message:  message,
			}},
		}
		switch phase {
		case releasedomain.StepSucceeded:
			state.FinalizeState = &releaseStepWrite{
				StepCode: "finalize_release",
				Status:   releasedomain.StepSucceeded,
				Progress: 100,
				Message:  "release finalized after deployment became healthy",
			}
		case releasedomain.StepFailed:
			state.FinalizeState = &releaseStepWrite{
				StepCode: "finalize_release",
				Status:   releasedomain.StepFailed,
				Progress: 100,
				Message:  "release finalized after deployment failure",
			}
		}
		return state, nil
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
	return deriveReleaseObservedStateFromRollout(rollout.Namespace, rollout.ObservedWorkloadName, rollout.PrimaryWorkloadName, item), nil
}

func (o *ReleaseRolloutObserver) writeReleaseSteps(ctx context.Context, rollout releaseRolloutContext, state *releaseObservedState) error {
	if state == nil {
		return nil
	}
	for _, step := range state.StepWrites {
		if err := o.postStep(ctx, rollout.ReleaseID, step.StepCode, step.Status, step.Progress, step.Message); err != nil {
			return err
		}
	}
	if state.FinalizeState == nil {
		return nil
	}
	finalize := *state.FinalizeState
	if finalize.Status == releasedomain.StepSucceeded {
		if err := o.verifyMetricsEndpoint(ctx, rollout); err != nil {
			finalize.Status = releasedomain.StepFailed
			finalize.Message = fmt.Sprintf("metrics endpoint verification failed: %v", err)
		}
	}
	if err := o.postStep(ctx, rollout.ReleaseID, finalize.StepCode, finalize.Status, finalize.Progress, finalize.Message); err != nil {
		return err
	}
	return nil
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
	workloadName := firstNonEmptyString(observedName, primaryName, "application")
	if rollout == nil {
		message := fmt.Sprintf("waiting for rollout %s in namespace %s", workloadName, firstNonEmptyString(namespace, "unknown"))
		return &releaseObservedState{
			Phase:    releasedomain.StepRunning,
			Progress: 10,
			Message:  message,
			StateKey: "rollout|missing",
			StepWrites: []releaseStepWrite{{
				StepCode: "deploy_canary",
				Status:   releasedomain.StepRunning,
				Progress: 10,
				Message:  message,
			}},
		}
	}

	strategyType := rolloutStrategyType(rollout)
	switch strategyType {
	case "bluegreen":
		return deriveBlueGreenObservedState(namespace, workloadName, rollout)
	default:
		return deriveCanaryObservedState(namespace, workloadName, rollout)
	}
}

func rolloutStrategyType(rollout *unstructured.Unstructured) string {
	if rollout == nil {
		return "canary"
	}
	if _, ok, _ := unstructured.NestedMap(rollout.Object, "spec", "strategy", "blueGreen"); ok {
		return "bluegreen"
	}
	if _, ok, _ := unstructured.NestedMap(rollout.Object, "spec", "strategy", "canary"); ok {
		return "canary"
	}
	return "canary"
}

func deriveBlueGreenObservedState(namespace, workloadName string, rollout *unstructured.Unstructured) *releaseObservedState {
	phase := strings.ToLower(strings.TrimSpace(nestedString(rollout.Object, "status", "phase")))
	message := firstNonEmptyString(nestedString(rollout.Object, "status", "message"))
	ready, available, desired := rolloutReplicaSummary(rollout)
	stateKey := fmt.Sprintf("bluegreen|%s|%d|%d|%d|%s", phase, desired, ready, available, nestedString(rollout.Object, "metadata", "generation"))
	switch phase {
	case "healthy", "completed":
		stepWrites := []releaseStepWrite{
			{StepCode: "deploy_preview", Status: releasedomain.StepSucceeded, Progress: 100, Message: blueGreenStepMessage("preview deployment ready", message)},
			{StepCode: "observe_preview", Status: releasedomain.StepSucceeded, Progress: 100, Message: blueGreenStepMessage("preview rollout observed healthy", message)},
			{StepCode: "switch_traffic", Status: releasedomain.StepSucceeded, Progress: 100, Message: blueGreenStepMessage("traffic switched to active workload", message)},
			{StepCode: "verify_active", Status: releasedomain.StepSucceeded, Progress: 100, Message: blueGreenStepMessage("active workload verified healthy", message)},
		}
		return &releaseObservedState{
			Phase:      releasedomain.StepSucceeded,
			Progress:   100,
			Message:    blueGreenStepMessage(fmt.Sprintf("blue-green rollout healthy (ready=%d/%d, available=%d/%d)", ready, desired, available, desired), message),
			StateKey:   stateKey,
			StepWrites: stepWrites,
			FinalizeState: &releaseStepWrite{
				StepCode: "finalize_release",
				Status:   releasedomain.StepSucceeded,
				Progress: 100,
				Message:  "release finalized after blue-green rollout became healthy",
			},
		}
	case "degraded", "error", "failed":
		return &releaseObservedState{
			Phase:    releasedomain.StepFailed,
			Progress: 100,
			Message:  blueGreenStepMessage(fmt.Sprintf("blue-green rollout failed (ready=%d/%d, available=%d/%d)", ready, desired, available, desired), message),
			StateKey: stateKey,
			StepWrites: []releaseStepWrite{{
				StepCode: "observe_preview",
				Status:   releasedomain.StepFailed,
				Progress: 100,
				Message:  blueGreenStepMessage("preview rollout failed", message),
			}},
			FinalizeState: &releaseStepWrite{
				StepCode: "finalize_release",
				Status:   releasedomain.StepFailed,
				Progress: 100,
				Message:  "release finalized after blue-green rollout failure",
			},
		}
	default:
		progress := rolloutProgressCandidate(ready, available, desired, 25)
		stepWrites := []releaseStepWrite{
			{StepCode: "deploy_preview", Status: releasedomain.StepSucceeded, Progress: 100, Message: "preview deployment created"},
			{StepCode: "observe_preview", Status: releasedomain.StepRunning, Progress: progress, Message: blueGreenStepMessage(fmt.Sprintf("preview rollout progressing (ready=%d/%d, available=%d/%d)", ready, desired, available, desired), message)},
		}
		return &releaseObservedState{
			Phase:      releasedomain.StepRunning,
			Progress:   progress,
			Message:    blueGreenStepMessage(fmt.Sprintf("blue-green rollout progressing (ready=%d/%d, available=%d/%d)", ready, desired, available, desired), message),
			StateKey:   stateKey,
			StepWrites: stepWrites,
		}
	}
}

func deriveCanaryObservedState(namespace, workloadName string, rollout *unstructured.Unstructured) *releaseObservedState {
	phase := strings.ToLower(strings.TrimSpace(nestedString(rollout.Object, "status", "phase")))
	message := firstNonEmptyString(nestedString(rollout.Object, "status", "message"))
	stepIndex, hasStepIndex := nestedInt64(rollout.Object, "status", "currentStepIndex")
	ready, available, desired := rolloutReplicaSummary(rollout)
	stateKey := fmt.Sprintf("canary|%s|%t|%d|%d|%d|%d|%s", phase, hasStepIndex, stepIndex, desired, ready, available, nestedString(rollout.Object, "metadata", "generation"))
	activeStep := canaryStepForIndex(stepIndex)

	switch phase {
	case "healthy", "completed":
		stepWrites := []releaseStepWrite{
			{StepCode: "deploy_canary", Status: releasedomain.StepSucceeded, Progress: 100, Message: "canary workload deployed"},
			{StepCode: "canary_10", Status: releasedomain.StepSucceeded, Progress: 100, Message: "canary 10% traffic completed"},
			{StepCode: "canary_30", Status: releasedomain.StepSucceeded, Progress: 100, Message: "canary 30% traffic completed"},
			{StepCode: "canary_60", Status: releasedomain.StepSucceeded, Progress: 100, Message: "canary 60% traffic completed"},
			{StepCode: "canary_100", Status: releasedomain.StepSucceeded, Progress: 100, Message: canaryStepMessage("canary 100% traffic completed", message)},
		}
		return &releaseObservedState{
			Phase:      releasedomain.StepSucceeded,
			Progress:   100,
			Message:    canaryStepMessage(fmt.Sprintf("canary rollout healthy (ready=%d/%d, available=%d/%d)", ready, desired, available, desired), message),
			StateKey:   stateKey,
			StepWrites: stepWrites,
			FinalizeState: &releaseStepWrite{
				StepCode: "finalize_release",
				Status:   releasedomain.StepSucceeded,
				Progress: 100,
				Message:  "release finalized after canary rollout became healthy",
			},
		}
	case "degraded", "error", "failed":
		failedStep := firstNonEmptyString(activeStep, "deploy_canary")
		return &releaseObservedState{
			Phase:    releasedomain.StepFailed,
			Progress: 100,
			Message:  canaryStepMessage(fmt.Sprintf("canary rollout failed at %s (ready=%d/%d, available=%d/%d)", failedStep, ready, desired, available, desired), message),
			StateKey: stateKey,
			StepWrites: append(canarySucceededWritesBefore(activeStep), releaseStepWrite{
				StepCode: failedStep,
				Status:   releasedomain.StepFailed,
				Progress: 100,
				Message:  canaryStepMessage(fmt.Sprintf("%s failed", strings.ReplaceAll(failedStep, "_", " ")), message),
			}),
			FinalizeState: &releaseStepWrite{
				StepCode: "finalize_release",
				Status:   releasedomain.StepFailed,
				Progress: 100,
				Message:  "release finalized after canary rollout failure",
			},
		}
	default:
		if !hasStepIndex || stepIndex < 0 {
			progress := rolloutProgressCandidate(ready, available, desired, 20)
			msg := canaryStepMessage(fmt.Sprintf("deploying canary workload (ready=%d/%d, available=%d/%d)", ready, desired, available, desired), message)
			return &releaseObservedState{
				Phase:    releasedomain.StepRunning,
				Progress: progress,
				Message:  msg,
				StateKey: stateKey,
				StepWrites: []releaseStepWrite{{
					StepCode: "deploy_canary",
					Status:   releasedomain.StepRunning,
					Progress: progress,
					Message:  msg,
				}},
			}
		}
		progress := canaryProgressForIndex(stepIndex)
		stepWrites := canarySucceededWritesBefore(activeStep)
		if stepIndex == 1 || stepIndex == 3 || stepIndex == 5 {
			stepWrites = append(stepWrites, releaseStepWrite{
				StepCode: activeStep,
				Status:   releasedomain.StepSucceeded,
				Progress: 100,
				Message:  canaryStepMessage(fmt.Sprintf("%s completed", strings.ReplaceAll(activeStep, "_", " ")), message),
			})
			nextStep := canaryNextStep(activeStep)
			if nextStep != "" {
				stepWrites = append(stepWrites, releaseStepWrite{
					StepCode: nextStep,
					Status:   releasedomain.StepRunning,
					Progress: progress,
					Message:  canaryStepMessage(fmt.Sprintf("%s pending promotion", strings.ReplaceAll(nextStep, "_", " ")), message),
				})
			}
		} else {
			stepWrites = append(stepWrites, releaseStepWrite{
				StepCode: activeStep,
				Status:   releasedomain.StepRunning,
				Progress: progress,
				Message:  canaryStepMessage(fmt.Sprintf("%s in progress", strings.ReplaceAll(activeStep, "_", " ")), message),
			})
		}
		return &releaseObservedState{
			Phase:      releasedomain.StepRunning,
			Progress:   progress,
			Message:    canaryStepMessage(fmt.Sprintf("canary rollout progressing at %s (ready=%d/%d, available=%d/%d)", activeStep, ready, desired, available, desired), message),
			StateKey:   stateKey,
			StepWrites: stepWrites,
		}
	}
}

func canarySucceededWritesBefore(activeStep string) []releaseStepWrite {
	all := []string{"deploy_canary", "canary_10", "canary_30", "canary_60", "canary_100"}
	out := make([]releaseStepWrite, 0, len(all))
	for _, step := range all {
		if step == activeStep {
			break
		}
		out = append(out, releaseStepWrite{
			StepCode: step,
			Status:   releasedomain.StepSucceeded,
			Progress: 100,
			Message:  fmt.Sprintf("%s completed", strings.ReplaceAll(step, "_", " ")),
		})
	}
	return out
}

func canaryStepForIndex(index int64) string {
	switch index {
	case 0, 1:
		return "canary_10"
	case 2, 3:
		return "canary_30"
	case 4, 5:
		return "canary_60"
	case 6:
		return "canary_100"
	default:
		return "deploy_canary"
	}
}

func canaryNextStep(step string) string {
	switch step {
	case "canary_10":
		return "canary_30"
	case "canary_30":
		return "canary_60"
	case "canary_60":
		return "canary_100"
	default:
		return ""
	}
}

func canaryProgressForIndex(index int64) int32 {
	switch index {
	case 0:
		return 30
	case 1:
		return 35
	case 2:
		return 50
	case 3:
		return 55
	case 4:
		return 70
	case 5:
		return 75
	case 6:
		return 90
	default:
		return 20
	}
}

func rolloutReplicaSummary(rollout *unstructured.Unstructured) (ready, available, desired int32) {
	if rollout == nil {
		return 0, 0, 0
	}
	desired = int32(nestedInt64Default(rollout.Object, 1, "spec", "replicas"))
	ready = int32(nestedInt64Default(rollout.Object, 0, "status", "readyReplicas"))
	available = int32(nestedInt64Default(rollout.Object, int64(ready), "status", "availableReplicas"))
	return ready, available, desired
}

func rolloutProgressCandidate(ready, available, desired int32, floor int32) int32 {
	progress := floor
	if desired > 0 {
		candidate := (maxInt32(ready, available) * 100) / desired
		if candidate > progress {
			progress = candidate
		}
	}
	if progress > 99 {
		progress = 99
	}
	return progress
}

func canaryStepMessage(base, details string) string {
	if strings.TrimSpace(details) == "" {
		return base
	}
	return fmt.Sprintf("%s (%s)", base, strings.TrimSpace(details))
}

func blueGreenStepMessage(base, details string) string {
	if strings.TrimSpace(details) == "" {
		return base
	}
	return fmt.Sprintf("%s (%s)", base, strings.TrimSpace(details))
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

func maxInt32(values ...int32) int32 {
	var max int32
	for i, value := range values {
		if i == 0 || value > max {
			max = value
		}
	}
	return max
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
	return platformobs.ObserveDependency(ctx, platformobs.DependencyCall{
		Kind:      "http",
		Target:    "release_service",
		Operation: "release_rollout_writeback",
	}, func(depCtx context.Context) error {
		req, err := http.NewRequestWithContext(depCtx, http.MethodPost, o.releaseBase+path, bytes.NewReader(body))
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
		return &releaseRolloutWritebackError{Path: path, StatusCode: resp.StatusCode}
	})
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
