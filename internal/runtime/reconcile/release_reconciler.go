package reconcile

import (
	"context"
	"fmt"
	"strings"

	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
	runtimedomain "github.com/bsonger/devflow-service/internal/runtime/domain"
	runtimerepo "github.com/bsonger/devflow-service/internal/runtime/repository"
	"github.com/google/uuid"
)

type ReleaseStateSource interface {
	GetRunningRelease(ctx context.Context, releaseID uuid.UUID) (*RunningRelease, error)
}

type RunningRelease struct {
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

	release, err := r.releases.GetRunningRelease(ctx, id)
	if err != nil {
		return err
	}
	if release == nil {
		return nil
	}
	if strings.TrimSpace(release.ControlPlaneID) != r.controlPlaneID {
		return nil
	}
	if strings.ToLower(strings.TrimSpace(release.Status)) != "running" {
		return nil
	}

	workload, err := r.getObservedWorkloadByRelease(ctx, id, release.ApplicationID, release.EnvironmentID)
	if err != nil {
		return err
	}
	if workload == nil {
		return nil
	}

	phase, progress, message, stepWrites := deriveWritebackState(workload)
	if phase == releasedomain.StepSucceeded && r.labelUpdater != nil {
		if err := r.labelUpdater.UpdateReleaseStatusLabel(ctx, workload, releasedomain.ReleaseSucceeded); err != nil {
			return err
		}
	}
	return r.stepsWriter.WriteReleaseSteps(ctx, WriteReleaseStepsInput{
		ReleaseID:            id,
		ApplicationID:        release.ApplicationID,
		EnvironmentID:        release.EnvironmentID,
		Namespace:            strings.TrimSpace(workload.Namespace),
		ObservedWorkloadKind: strings.TrimSpace(workload.WorkloadKind),
		ObservedWorkloadName: strings.TrimSpace(workload.WorkloadName),
		Phase:                phase,
		Progress:             progress,
		Message:              message,
		StepWrites:           stepWrites,
	})
}

func (r *ReleaseReconciler) getObservedWorkloadByRelease(ctx context.Context, releaseID, applicationID uuid.UUID, environmentID string) (*runtimedomain.RuntimeObservedWorkload, error) {
	specs, err := r.runtimeStore.ListRuntimeSpecs(ctx)
	if err != nil {
		return nil, err
	}
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
		if err != nil || workload == nil {
			return nil, err
		}
		if strings.TrimSpace(workload.Labels[releasedomain.ReleaseIDLabel]) != releaseID.String() {
			return nil, nil
		}
		return workload, nil
	}
	return nil, nil
}

func deriveWritebackState(workload *runtimedomain.RuntimeObservedWorkload) (releasedomain.StepStatus, int32, string, []ReleaseStepWrite) {
	namespace := strings.TrimSpace(workload.Namespace)
	workloadName := strings.TrimSpace(workload.WorkloadName)
	if workloadName == "" {
		workloadName = "application"
	}
	if namespace == "" {
		namespace = "unknown"
	}

	runningMessage := fmt.Sprintf("waiting for workload %s in namespace %s", workloadName, namespace)
	progress := int32(10)
	if workload != nil && workload.DesiredReplicas > 0 {
		calculated := int32((workload.ReadyReplicas * 90) / workload.DesiredReplicas)
		if calculated < 10 {
			calculated = 10
		}
		if calculated > 99 {
			calculated = 99
		}
		progress = calculated
	}

	if workload != nil &&
		workload.DesiredReplicas > 0 &&
		workload.ReadyReplicas >= workload.DesiredReplicas &&
		workload.UnavailableReplicas == 0 {
		successMessage := fmt.Sprintf("workload %s in namespace %s is healthy", workloadName, namespace)
		return releasedomain.StepSucceeded, 100, successMessage, []ReleaseStepWrite{
			{
				StepCode: "observe_rollout",
				Status:   releasedomain.StepSucceeded,
				Progress: 100,
				Message:  successMessage,
			},
			{
				StepCode: "finalize_release",
				Status:   releasedomain.StepSucceeded,
				Progress: 100,
				Message:  "release finalized after deployment became healthy",
			},
		}
	}

	return releasedomain.StepRunning, progress, runningMessage, []ReleaseStepWrite{{
		StepCode: "observe_rollout",
		Status:   releasedomain.StepRunning,
		Progress: progress,
		Message:  runningMessage,
	}}
}
