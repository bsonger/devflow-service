package reconcile

import (
	"context"
	"strings"
	"time"

	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
	runtimedomain "github.com/bsonger/devflow-service/internal/runtime/domain"
	runtimerepo "github.com/bsonger/devflow-service/internal/runtime/repository"
	"github.com/google/uuid"
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
	if workload == nil {
		return nil
	}

	state := normalizeObservedStateForReconcile(workload)
	stepWrites := append([]ReleaseStepWrite{}, state.StepWrites...)
	if state.FinalizeState != nil {
		stepWrites = append(stepWrites, *state.FinalizeState)
	}
	if terminalStatus, ok := terminalReleaseStatus(state.Phase); ok && r.labelUpdater != nil {
		if err := r.labelUpdater.UpdateReleaseStatusLabel(ctx, workload, terminalStatus); err != nil {
			return err
		}
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
	if state.Phase == releasedomain.StepRunning {
		return releaseRequeueAfterError{after: 5 * time.Second}
	}
	return nil
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
		if matched == nil || workload.ObservedAt.After(matched.ObservedAt) {
			matched = workload
		}
	}
	return matched, nil
}
