package reconcile

import (
	"context"
	"strings"
	"time"

	"github.com/bsonger/devflow-service/internal/platform/logger"
	platformobserver "github.com/bsonger/devflow-service/internal/platform/observer"
	platformobs "github.com/bsonger/devflow-service/internal/platform/runtime/observability"
	releasecontrol "github.com/bsonger/devflow-service/internal/release/control"
	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
	rollingstrategy "github.com/bsonger/devflow-service/internal/release/strategy/rolling"
	runtimedomain "github.com/bsonger/devflow-service/internal/runtime/domain"
	runtimerepo "github.com/bsonger/devflow-service/internal/runtime/repository"
	"github.com/google/uuid"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ReleaseStateSource interface {
	GetRelease(ctx context.Context, releaseID uuid.UUID) (*ReleaseRecord, error)
}

type ReleaseRecord struct {
	ReleaseID      uuid.UUID
	ApplicationID  uuid.UUID
	EnvironmentID  string
	ControlPlaneID string
	Status         string
}

type ReleaseStepsWriter interface {
	WriteReleaseSteps(ctx context.Context, input WriteReleaseStepsInput) error
}

type ReleaseStatusLabelUpdater interface {
	UpdateReleaseStatusLabel(ctx context.Context, workload *runtimedomain.RuntimeObservedWorkload, status releasedomain.ReleaseStatus) error
}

const defaultObservedReleaseTTL = 45 * time.Minute
const defaultRunningReleaseProgressTimeout = 10 * time.Minute

var releaseReconcileLogf = defaultReleaseReconcileLogf

func defaultReleaseReconcileLogf(event string, fields ...zap.Field) {
	logger.RootLogger.Named("runtime.state").Info(event, fields...)
}

type ReleaseStepWrite struct {
	StepCode string
	Status   releasedomain.StepStatus
	Progress int32
	Message  string
}

type WriteReleaseStepsInput struct {
	ReleaseID            uuid.UUID
	ApplicationID        uuid.UUID
	EnvironmentID        string
	Namespace            string
	ObservedWorkloadKind string
	ObservedWorkloadName string
	Phase                releasedomain.StepStatus
	Progress             int32
	Message              string
	StepWrites           []ReleaseStepWrite
}

type ReleaseReconciler struct {
	releases       ReleaseStateSource
	runtimeStore   runtimerepo.Store
	stepsWriter    ReleaseStepsWriter
	labelUpdater   ReleaseStatusLabelUpdater
	controlPlaneID string
}

type releaseRequeueAfterError struct {
	after time.Duration
}

func (e releaseRequeueAfterError) Error() string {
	return "release reconcile pending workload health"
}

func (e releaseRequeueAfterError) RequeueAfter() time.Duration {
	if e.after <= 0 {
		return 5 * time.Second
	}
	return e.after
}

func NewReleaseReconciler(releases ReleaseStateSource, runtimeStore runtimerepo.Store, stepsWriter ReleaseStepsWriter, labelUpdater ReleaseStatusLabelUpdater, controlPlaneID string) *ReleaseReconciler {
	return &ReleaseReconciler{
		releases:       releases,
		runtimeStore:   runtimeStore,
		stepsWriter:    stepsWriter,
		labelUpdater:   labelUpdater,
		controlPlaneID: strings.TrimSpace(controlPlaneID),
	}
}

func (r *ReleaseReconciler) Reconcile(ctx context.Context, releaseID string) error {
	id, err := uuid.Parse(strings.TrimSpace(releaseID))
	if err != nil {
		return nil
	}

	release, err := r.releases.GetRelease(ctx, id)
	if err != nil {
		return err
	}
	if release == nil {
		return nil
	}
	if strings.TrimSpace(release.ControlPlaneID) != r.controlPlaneID {
		return nil
	}

	workload, err := r.getObservedWorkloadByRelease(ctx, id, release.ApplicationID, release.EnvironmentID)
	if err != nil {
		return err
	}
	baseFields := releaseReconcileFields(id, release.ApplicationID, release.EnvironmentID, nil, "", 0)
	releaseReconcileLogf("release_reconcile_start", baseFields...)
	if workload == nil {
		releaseReconcileLogf("release_reconcile_workload_missing", baseFields...)
		platformobs.RecordRuntimeReleaseReconcile(ctx, "", "ok")
		return nil
	}

	state := normalizeObservedStateForReconcile(workload)
	state = r.applyRunningReleaseTimeouts(release, workload, state)
	stateFields := releaseReconcileFields(id, release.ApplicationID, release.EnvironmentID, workload, state.Phase, state.Progress)
	releaseReconcileLogf("release_reconcile_state_computed", stateFields...)
	platformobs.RecordRuntimeReleaseReconcile(ctx, string(state.Phase), "ok")
	stepWrites := append([]ReleaseStepWrite{}, state.StepWrites...)
	if state.FinalizeState != nil {
		stepWrites = append(stepWrites, *state.FinalizeState)
	}
	var terminalLabelUpdateErr error
	var terminalStatus releasedomain.ReleaseStatus
	if status, ok := terminalReleaseStatus(state.Phase); ok {
		terminalStatus = status
	}
	if terminalStatus != "" && r.labelUpdater != nil {
		terminalLabelUpdateErr = r.labelUpdater.UpdateReleaseStatusLabel(ctx, workload, terminalStatus)
		if terminalLabelUpdateErr != nil {
			platformobs.RecordRuntimeTerminalLabelUpdate(ctx, workload.WorkloadKind, "error", "update_failed")
			releaseReconcileLogf("release_reconcile_terminal_label_update_failed",
				append(stateFields,
					zap.String("action", "update_release_status_label"),
					zap.String("target_status", string(terminalStatus)),
					zap.Error(terminalLabelUpdateErr),
				)...,
			)
		} else {
			platformobs.RecordRuntimeTerminalLabelUpdate(ctx, workload.WorkloadKind, "ok", "")
		}
	}
	if terminalStatus != "" {
		clearObservedWorkloadReleaseTracking(workload, terminalStatus)
		if err := r.runtimeStore.UpsertObservedWorkload(ctx, workload); err != nil {
			return err
		}
		releaseReconcileLogf("release_reconcile_cleanup_completed", stateFields...)
	}
	// Queue/event sources own duplicate suppression. A repeated reconcile should
	// re-emit the currently observed state, including terminal compensation.
	if err := r.stepsWriter.WriteReleaseSteps(ctx, WriteReleaseStepsInput{
		ReleaseID:            id,
		ApplicationID:        release.ApplicationID,
		EnvironmentID:        release.EnvironmentID,
		Namespace:            strings.TrimSpace(workload.Namespace),
		ObservedWorkloadKind: strings.TrimSpace(workload.WorkloadKind),
		ObservedWorkloadName: strings.TrimSpace(workload.WorkloadName),
		Phase:                state.Phase,
		Progress:             state.Progress,
		Message:              state.Message,
		StepWrites:           stepWrites,
	}); err != nil {
		return err
	}
	releaseReconcileLogf("release_reconcile_writeback_completed", stateFields...)
	if terminalLabelUpdateErr != nil {
		return nil
	}
	if state.Phase == releasedomain.StepRunning {
		releaseReconcileLogf("release_reconcile_requeue_scheduled", stateFields...)
		platformobs.RecordRuntimeReleaseReconcile(ctx, string(state.Phase), "requeue")
		return releaseRequeueAfterError{after: 5 * time.Second}
	}
	return nil
}

func (r *ReleaseReconciler) applyRunningReleaseTimeouts(
	release *ReleaseRecord,
	workload *runtimedomain.RuntimeObservedWorkload,
	state NormalizedReleaseObservedState,
) NormalizedReleaseObservedState {
	if release == nil || workload == nil {
		return state
	}
	if !strings.EqualFold(strings.TrimSpace(workload.WorkloadKind), "deployment") {
		return state
	}
	if !strings.EqualFold(strings.TrimSpace(release.Status), string(releasedomain.ReleaseRunning)) {
		return state
	}
	if state.Phase != releasedomain.StepRunning {
		return state
	}

	now := time.Now().UTC()
	lastObservation := MapNormalizedDeploymentStateToRuntimeObservation(state, workload.ObservedAt)
	controlState := releasedomain.ReleaseControlState{
		LifecycleStatus: releasedomain.LifecycleRunning,
		Strategy:        releasedomain.ReleaseStrategyRolling,
		StrategyPhase:   releasedomain.PhaseRollingProgressing,
		UpdatedAt:       workload.ObservedAt,
	}
	evaluator := releasecontrol.TimeoutEvaluator{}
	policy := rollingstrategy.NewController().TimeoutPolicy(controlState)
	if policy.ProgressTimeout <= 0 {
		policy.ProgressTimeout = defaultRunningReleaseProgressTimeout
	}
	decision := evaluator.Evaluate(controlState, &lastObservation, policy, now)
	if decision.NextState == nil || decision.NextState.FailureReason == "" {
		return state
	}
	for _, write := range decision.StepWrites {
		if write.StepCode == "observe_rollout" {
			state.Phase = write.Status
			state.Progress = int32(write.Progress)
			state.Message = write.Message
			state.StepWrites = []ReleaseStepWrite{{
				StepCode: write.StepCode,
				Status:   write.Status,
				Progress: int32(write.Progress),
				Message:  write.Message,
			}}
		}
	}
	state.FinalizeState = &ReleaseStepWrite{
		StepCode: "finalize_release",
		Status:   releasedomain.StepFailed,
		Progress: 100,
		Message:  "release finalized after timeout or observation stall",
	}
	return state
}

func releaseReconcileFields(releaseID, applicationID uuid.UUID, environmentID string, workload *runtimedomain.RuntimeObservedWorkload, phase releasedomain.StepStatus, progress int32) []zap.Field {
	workloadKind := ""
	workloadName := ""
	if workload != nil {
		workloadKind = strings.TrimSpace(workload.WorkloadKind)
		workloadName = strings.TrimSpace(workload.WorkloadName)
	}
	return []zap.Field{
		zap.String("release_id", releaseID.String()),
		zap.String("application_id", applicationID.String()),
		zap.String("environment_id", strings.TrimSpace(environmentID)),
		zap.String("workload_kind", workloadKind),
		zap.String("workload_name", workloadName),
		zap.String("phase", string(phase)),
		zap.Int32("progress", progress),
	}
}

func terminalReleaseStatus(phase releasedomain.StepStatus) (releasedomain.ReleaseStatus, bool) {
	switch phase {
	case releasedomain.StepSucceeded:
		return releasedomain.ReleaseSucceeded, true
	case releasedomain.StepFailed:
		return releasedomain.ReleaseFailed, true
	default:
		return "", false
	}
}

func normalizeObservedStateForReconcile(workload *runtimedomain.RuntimeObservedWorkload) NormalizedReleaseObservedState {
	if workload == nil || !strings.EqualFold(strings.TrimSpace(workload.WorkloadKind), "deployment") {
		return NormalizeReleaseObservedState(workload)
	}

	replicas := int32(workload.DesiredReplicas)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:       strings.TrimSpace(workload.WorkloadName),
			Generation: workload.ObservedGeneration,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
		},
		Status: appsv1.DeploymentStatus{
			ObservedGeneration:  workload.ObservedGeneration,
			UpdatedReplicas:     int32(workload.UpdatedReplicas),
			ReadyReplicas:       int32(workload.ReadyReplicas),
			AvailableReplicas:   int32(workload.AvailableReplicas),
			UnavailableReplicas: int32(workload.UnavailableReplicas),
			Conditions:          deploymentConditionsForReconcile(workload.Conditions),
		},
	}
	return NormalizeReleaseObservedStateFromDeployment(strings.TrimSpace(workload.Namespace), strings.TrimSpace(workload.WorkloadName), deployment)
}

func deploymentConditionsForReconcile(conditions []runtimedomain.RuntimeObservedWorkloadCondition) []appsv1.DeploymentCondition {
	if len(conditions) == 0 {
		return nil
	}
	out := make([]appsv1.DeploymentCondition, 0, len(conditions))
	for _, condition := range conditions {
		out = append(out, appsv1.DeploymentCondition{
			Type:    appsv1.DeploymentConditionType(strings.TrimSpace(condition.Type)),
			Status:  corev1.ConditionStatus(strings.TrimSpace(condition.Status)),
			Reason:  strings.TrimSpace(condition.Reason),
			Message: strings.TrimSpace(condition.Message),
		})
	}
	return out
}

func (r *ReleaseReconciler) getObservedWorkloadByRelease(ctx context.Context, releaseID, applicationID uuid.UUID, environmentID string) (*runtimedomain.RuntimeObservedWorkload, error) {
	specs, err := r.runtimeStore.ListRuntimeSpecs(ctx)
	if err != nil {
		return nil, err
	}
	var matched *runtimedomain.RuntimeObservedWorkload
	now := time.Now().UTC()
	for _, spec := range specs {
		if spec == nil {
			continue
		}
		if spec.ApplicationID != applicationID {
			continue
		}
		if strings.TrimSpace(spec.Environment) != strings.TrimSpace(environmentID) {
			continue
		}
		workload, err := r.runtimeStore.GetObservedWorkload(ctx, spec.ID)
		if err != nil {
			return nil, err
		}
		if workload == nil {
			continue
		}
		if strings.TrimSpace(workload.Labels[releasedomain.ReleaseIDLabel]) != releaseID.String() {
			continue
		}
		if workload.DeletedAt != nil {
			continue
		}
		if workload.ObservedAt.IsZero() || now.Sub(workload.ObservedAt) > defaultObservedReleaseTTL {
			clearObservedWorkloadReleaseTracking(workload, "")
			if err := r.runtimeStore.UpsertObservedWorkload(ctx, workload); err != nil {
				return nil, err
			}
			continue
		}
		if matched == nil || workload.ObservedAt.After(matched.ObservedAt) {
			matched = workload
		}
	}
	return matched, nil
}

func clearObservedWorkloadReleaseTracking(workload *runtimedomain.RuntimeObservedWorkload, terminalStatus releasedomain.ReleaseStatus) {
	if workload == nil {
		return
	}
	if workload.Labels == nil {
		workload.Labels = map[string]string{}
	}
	delete(workload.Labels, releasedomain.ReleaseIDLabel)
	if terminalStatus != "" {
		workload.Labels[releasedomain.ReleaseStatusLabel] = string(terminalStatus)
	} else {
		delete(workload.Labels, releasedomain.ReleaseStatusLabel)
	}
	workload.Labels[platformobserver.ObserveStateLabel] = platformobserver.ObserveStateDone
}
