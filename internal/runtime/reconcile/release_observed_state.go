package reconcile

import (
	"fmt"
	"strings"

	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
	runtimedomain "github.com/bsonger/devflow-service/internal/runtime/domain"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type NormalizedReleaseObservedState struct {
	Phase         releasedomain.StepStatus
	Progress      int32
	Message       string
	StateKey      string
	StepWrites    []ReleaseStepWrite
	FinalizeState *ReleaseStepWrite
}

func NormalizeReleaseObservedState(workload *runtimedomain.RuntimeObservedWorkload) NormalizedReleaseObservedState {
	if strings.EqualFold(strings.TrimSpace(workloadKind(workload)), "rollout") {
		return normalizeReleaseObservedWorkloadRollout(workload)
	}
	return normalizeReleaseObservedWorkloadDeployment(workload)
}

func NormalizeReleaseObservedStateFromDeployment(namespace, workloadName string, deployment *appsv1.Deployment) NormalizedReleaseObservedState {
	phase, message, progress, stateKey := deriveReleaseObservedDeploymentState(namespace, workloadName, deployment)
	state := NormalizedReleaseObservedState{
		Phase:    phase,
		Progress: progress,
		Message:  message,
		StateKey: stateKey,
		StepWrites: []ReleaseStepWrite{{
			StepCode: "observe_rollout",
			Status:   phase,
			Progress: progress,
			Message:  message,
		}},
	}
	switch phase {
	case releasedomain.StepSucceeded:
		state.FinalizeState = &ReleaseStepWrite{
			StepCode: "finalize_release",
			Status:   releasedomain.StepSucceeded,
			Progress: 100,
			Message:  "release finalized after deployment became healthy",
		}
	case releasedomain.StepFailed:
		state.FinalizeState = &ReleaseStepWrite{
			StepCode: "finalize_release",
			Status:   releasedomain.StepFailed,
			Progress: 100,
			Message:  "release finalized after deployment failure",
		}
	}
	return state
}

func NormalizeReleaseObservedStateFromRollout(namespace, observedName, primaryName string, rollout *unstructured.Unstructured) NormalizedReleaseObservedState {
	workloadName := firstNonEmptyString(observedName, primaryName, "application")
	if rollout == nil {
		message := fmt.Sprintf("waiting for rollout %s in namespace %s", workloadName, firstNonEmptyString(namespace, "unknown"))
		return NormalizedReleaseObservedState{
			Phase:    releasedomain.StepRunning,
			Progress: 10,
			Message:  message,
			StateKey: "rollout|missing",
			StepWrites: []ReleaseStepWrite{{
				StepCode: "deploy_canary",
				Status:   releasedomain.StepRunning,
				Progress: 10,
				Message:  message,
			}},
		}
	}

	switch rolloutStrategyType(rollout) {
	case "bluegreen":
		return deriveBlueGreenObservedState(namespace, workloadName, rollout)
	default:
		return deriveCanaryObservedState(namespace, workloadName, rollout)
	}
}

func normalizeReleaseObservedWorkloadDeployment(workload *runtimedomain.RuntimeObservedWorkload) NormalizedReleaseObservedState {
	return NormalizeReleaseObservedStateFromDeployment(workloadNamespace(workload), workloadName(workload), deploymentFromObservedWorkload(workload))
}

func normalizeReleaseObservedWorkloadRollout(workload *runtimedomain.RuntimeObservedWorkload) NormalizedReleaseObservedState {
	summary := strings.ToLower(strings.TrimSpace(workloadSummaryStatus(workload)))
	name := workloadName(workload)
	namespace := workloadNamespace(workload)
	ready := int32(workloadReadyReplicas(workload))
	available := int32(workloadAvailableReplicas(workload))
	desired := int32(workloadDesiredReplicas(workload))

	switch summary {
	case "healthy", "completed":
		return NormalizedReleaseObservedState{
			Phase:    releasedomain.StepSucceeded,
			Progress: 100,
			Message:  fmt.Sprintf("rollout %s is healthy", name),
			StateKey: fmt.Sprintf("rollout|succeeded|%s|%d|%d|%d", summary, desired, ready, available),
			StepWrites: []ReleaseStepWrite{{
				StepCode: "observe_rollout",
				Status:   releasedomain.StepSucceeded,
				Progress: 100,
				Message:  fmt.Sprintf("rollout %s is healthy", name),
			}},
			FinalizeState: &ReleaseStepWrite{
				StepCode: "finalize_release",
				Status:   releasedomain.StepSucceeded,
				Progress: 100,
				Message:  "release finalized after rollout became healthy",
			},
		}
	case "degraded", "error", "failed":
		return NormalizedReleaseObservedState{
			Phase:    releasedomain.StepFailed,
			Progress: 100,
			Message:  fmt.Sprintf("rollout %s failed", name),
			StateKey: fmt.Sprintf("rollout|failed|%s|%d|%d|%d", summary, desired, ready, available),
			StepWrites: []ReleaseStepWrite{{
				StepCode: "observe_rollout",
				Status:   releasedomain.StepFailed,
				Progress: 100,
				Message:  fmt.Sprintf("rollout %s failed", name),
			}},
			FinalizeState: &ReleaseStepWrite{
				StepCode: "finalize_release",
				Status:   releasedomain.StepFailed,
				Progress: 100,
				Message:  "release finalized after rollout failure",
			},
		}
	default:
		progress := rolloutProgressCandidate(ready, available, desired, 20)
		message := fmt.Sprintf("waiting for rollout %s in namespace %s", name, namespace)
		return NormalizedReleaseObservedState{
			Phase:    releasedomain.StepRunning,
			Progress: progress,
			Message:  message,
			StateKey: fmt.Sprintf("rollout|running|%s|%d|%d|%d", summary, desired, ready, available),
			StepWrites: []ReleaseStepWrite{{
				StepCode: "observe_rollout",
				Status:   releasedomain.StepRunning,
				Progress: progress,
				Message:  message,
			}},
		}
	}
}

func deploymentFromObservedWorkload(workload *runtimedomain.RuntimeObservedWorkload) *appsv1.Deployment {
	if workload == nil {
		return nil
	}
	replicas := int32(workload.DesiredReplicas)
	deployment := &appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
		},
		Status: appsv1.DeploymentStatus{
			ObservedGeneration:  workload.ObservedGeneration,
			UpdatedReplicas:     int32(workload.UpdatedReplicas),
			ReadyReplicas:       int32(workload.ReadyReplicas),
			AvailableReplicas:   int32(workload.AvailableReplicas),
			UnavailableReplicas: int32(workload.UnavailableReplicas),
			Conditions:          deploymentConditionsFromObservedWorkload(workload.Conditions),
		},
	}
	if workload.ObservedGeneration > 0 {
		deployment.Generation = workload.ObservedGeneration
	}
	return deployment
}

func deploymentConditionsFromObservedWorkload(conditions []runtimedomain.RuntimeObservedWorkloadCondition) []appsv1.DeploymentCondition {
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

func deriveReleaseObservedDeploymentState(namespace, appName string, deployment *appsv1.Deployment) (releasedomain.StepStatus, string, int32, string) {
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

	if desired > 0 && generationObserved && updated >= int(desired) && ready >= int(desired) && available >= int(desired) && unavailable == 0 {
		message := fmt.Sprintf("deployment healthy (ready=%d/%d, updated=%d/%d, available=%d/%d)", ready, desired, updated, desired, available, desired)
		return releasedomain.StepSucceeded, message, 100, fmt.Sprintf("succeeded|%d|%d|%d|%d", desired, updated, ready, available)
	}

	progress := int32(25)
	if desired > 0 {
		candidate := int32((available * 100) / int(desired))
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

func deriveBlueGreenObservedState(namespace, workloadName string, rollout *unstructured.Unstructured) NormalizedReleaseObservedState {
	phase := strings.ToLower(strings.TrimSpace(nestedString(rollout.Object, "status", "phase")))
	message := firstNonEmptyString(nestedString(rollout.Object, "status", "message"))
	ready, available, desired := rolloutReplicaSummary(rollout)
	stateKey := fmt.Sprintf("bluegreen|%s|%d|%d|%d|%s", phase, desired, ready, available, nestedString(rollout.Object, "metadata", "generation"))
	switch phase {
	case "healthy", "completed":
		stepWrites := []ReleaseStepWrite{
			{StepCode: "deploy_preview", Status: releasedomain.StepSucceeded, Progress: 100, Message: blueGreenStepMessage("preview deployment ready", message)},
			{StepCode: "observe_preview", Status: releasedomain.StepSucceeded, Progress: 100, Message: blueGreenStepMessage("preview rollout observed healthy", message)},
			{StepCode: "switch_traffic", Status: releasedomain.StepSucceeded, Progress: 100, Message: blueGreenStepMessage("traffic switched to active workload", message)},
			{StepCode: "verify_active", Status: releasedomain.StepSucceeded, Progress: 100, Message: blueGreenStepMessage("active workload verified healthy", message)},
		}
		return NormalizedReleaseObservedState{
			Phase:      releasedomain.StepSucceeded,
			Progress:   100,
			Message:    blueGreenStepMessage(fmt.Sprintf("blue-green rollout healthy (ready=%d/%d, available=%d/%d)", ready, desired, available, desired), message),
			StateKey:   stateKey,
			StepWrites: stepWrites,
			FinalizeState: &ReleaseStepWrite{
				StepCode: "finalize_release",
				Status:   releasedomain.StepSucceeded,
				Progress: 100,
				Message:  "release finalized after blue-green rollout became healthy",
			},
		}
	case "degraded", "error", "failed":
		return NormalizedReleaseObservedState{
			Phase:    releasedomain.StepFailed,
			Progress: 100,
			Message:  blueGreenStepMessage(fmt.Sprintf("blue-green rollout failed (ready=%d/%d, available=%d/%d)", ready, desired, available, desired), message),
			StateKey: stateKey,
			StepWrites: []ReleaseStepWrite{{
				StepCode: "observe_preview",
				Status:   releasedomain.StepFailed,
				Progress: 100,
				Message:  blueGreenStepMessage("preview rollout failed", message),
			}},
			FinalizeState: &ReleaseStepWrite{
				StepCode: "finalize_release",
				Status:   releasedomain.StepFailed,
				Progress: 100,
				Message:  "release finalized after blue-green rollout failure",
			},
		}
	default:
		progress := rolloutProgressCandidate(ready, available, desired, 25)
		stepWrites := []ReleaseStepWrite{
			{StepCode: "deploy_preview", Status: releasedomain.StepSucceeded, Progress: 100, Message: "preview deployment created"},
			{StepCode: "observe_preview", Status: releasedomain.StepRunning, Progress: progress, Message: blueGreenStepMessage(fmt.Sprintf("preview rollout progressing (ready=%d/%d, available=%d/%d)", ready, desired, available, desired), message)},
		}
		return NormalizedReleaseObservedState{
			Phase:      releasedomain.StepRunning,
			Progress:   progress,
			Message:    blueGreenStepMessage(fmt.Sprintf("blue-green rollout progressing (ready=%d/%d, available=%d/%d)", ready, desired, available, desired), message),
			StateKey:   stateKey,
			StepWrites: stepWrites,
		}
	}
}

func deriveCanaryObservedState(namespace, workloadName string, rollout *unstructured.Unstructured) NormalizedReleaseObservedState {
	phase := strings.ToLower(strings.TrimSpace(nestedString(rollout.Object, "status", "phase")))
	message := firstNonEmptyString(nestedString(rollout.Object, "status", "message"))
	stepIndex, hasStepIndex := nestedInt64(rollout.Object, "status", "currentStepIndex")
	ready, available, desired := rolloutReplicaSummary(rollout)
	stateKey := fmt.Sprintf("canary|%s|%t|%d|%d|%d|%d|%s", phase, hasStepIndex, stepIndex, desired, ready, available, nestedString(rollout.Object, "metadata", "generation"))
	activeStep := canaryStepForIndex(stepIndex)

	switch phase {
	case "healthy", "completed":
		stepWrites := []ReleaseStepWrite{
			{StepCode: "deploy_canary", Status: releasedomain.StepSucceeded, Progress: 100, Message: "canary workload deployed"},
			{StepCode: "canary_10", Status: releasedomain.StepSucceeded, Progress: 100, Message: "canary 10% traffic completed"},
			{StepCode: "canary_30", Status: releasedomain.StepSucceeded, Progress: 100, Message: "canary 30% traffic completed"},
			{StepCode: "canary_60", Status: releasedomain.StepSucceeded, Progress: 100, Message: "canary 60% traffic completed"},
			{StepCode: "canary_100", Status: releasedomain.StepSucceeded, Progress: 100, Message: canaryStepMessage("canary 100% traffic completed", message)},
		}
		return NormalizedReleaseObservedState{
			Phase:      releasedomain.StepSucceeded,
			Progress:   100,
			Message:    canaryStepMessage(fmt.Sprintf("canary rollout healthy (ready=%d/%d, available=%d/%d)", ready, desired, available, desired), message),
			StateKey:   stateKey,
			StepWrites: stepWrites,
			FinalizeState: &ReleaseStepWrite{
				StepCode: "finalize_release",
				Status:   releasedomain.StepSucceeded,
				Progress: 100,
				Message:  "release finalized after canary rollout became healthy",
			},
		}
	case "degraded", "error", "failed":
		failedStep := firstNonEmptyString(activeStep, "deploy_canary")
		return NormalizedReleaseObservedState{
			Phase:    releasedomain.StepFailed,
			Progress: 100,
			Message:  canaryStepMessage(fmt.Sprintf("canary rollout failed at %s (ready=%d/%d, available=%d/%d)", failedStep, ready, desired, available, desired), message),
			StateKey: stateKey,
			StepWrites: append(canarySucceededWritesBefore(activeStep), ReleaseStepWrite{
				StepCode: failedStep,
				Status:   releasedomain.StepFailed,
				Progress: 100,
				Message:  canaryStepMessage(fmt.Sprintf("%s failed", strings.ReplaceAll(failedStep, "_", " ")), message),
			}),
			FinalizeState: &ReleaseStepWrite{
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
			return NormalizedReleaseObservedState{
				Phase:    releasedomain.StepRunning,
				Progress: progress,
				Message:  msg,
				StateKey: stateKey,
				StepWrites: []ReleaseStepWrite{{
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
			stepWrites = append(stepWrites, ReleaseStepWrite{
				StepCode: activeStep,
				Status:   releasedomain.StepSucceeded,
				Progress: 100,
				Message:  canaryStepMessage(fmt.Sprintf("%s completed", strings.ReplaceAll(activeStep, "_", " ")), message),
			})
			nextStep := canaryNextStep(activeStep)
			if nextStep != "" {
				stepWrites = append(stepWrites, ReleaseStepWrite{
					StepCode: nextStep,
					Status:   releasedomain.StepRunning,
					Progress: progress,
					Message:  canaryStepMessage(fmt.Sprintf("%s pending promotion", strings.ReplaceAll(nextStep, "_", " ")), message),
				})
			}
		} else {
			stepWrites = append(stepWrites, ReleaseStepWrite{
				StepCode: activeStep,
				Status:   releasedomain.StepRunning,
				Progress: progress,
				Message:  canaryStepMessage(fmt.Sprintf("%s in progress", strings.ReplaceAll(activeStep, "_", " ")), message),
			})
		}
		return NormalizedReleaseObservedState{
			Phase:      releasedomain.StepRunning,
			Progress:   progress,
			Message:    canaryStepMessage(fmt.Sprintf("canary rollout progressing at %s (ready=%d/%d, available=%d/%d)", activeStep, ready, desired, available, desired), message),
			StateKey:   stateKey,
			StepWrites: stepWrites,
		}
	}
}

func canarySucceededWritesBefore(activeStep string) []ReleaseStepWrite {
	all := []string{"deploy_canary", "canary_10", "canary_30", "canary_60", "canary_100"}
	out := make([]ReleaseStepWrite, 0, len(all))
	for _, step := range all {
		if step == activeStep {
			break
		}
		out = append(out, ReleaseStepWrite{
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

func int32Value(value *int32) int32 {
	if value == nil {
		return 0
	}
	return *value
}

func workloadKind(workload *runtimedomain.RuntimeObservedWorkload) string {
	if workload == nil {
		return ""
	}
	return strings.TrimSpace(workload.WorkloadKind)
}

func workloadName(workload *runtimedomain.RuntimeObservedWorkload) string {
	if workload == nil {
		return "application"
	}
	if value := strings.TrimSpace(workload.WorkloadName); value != "" {
		return value
	}
	return "application"
}

func workloadNamespace(workload *runtimedomain.RuntimeObservedWorkload) string {
	if workload == nil {
		return "unknown"
	}
	if value := strings.TrimSpace(workload.Namespace); value != "" {
		return value
	}
	return "unknown"
}

func workloadSummaryStatus(workload *runtimedomain.RuntimeObservedWorkload) string {
	if workload == nil {
		return ""
	}
	return strings.TrimSpace(workload.SummaryStatus)
}

func workloadDesiredReplicas(workload *runtimedomain.RuntimeObservedWorkload) int {
	if workload == nil {
		return 0
	}
	return workload.DesiredReplicas
}

func workloadReadyReplicas(workload *runtimedomain.RuntimeObservedWorkload) int {
	if workload == nil {
		return 0
	}
	return workload.ReadyReplicas
}

func workloadAvailableReplicas(workload *runtimedomain.RuntimeObservedWorkload) int {
	if workload == nil {
		return 0
	}
	return workload.AvailableReplicas
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
