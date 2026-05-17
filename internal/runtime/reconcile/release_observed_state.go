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
	return NormalizeReleaseObservedStateFromRollout(
		workloadNamespace(workload),
		workloadName(workload),
		workloadPrimaryName(workload),
		rolloutFromObservedWorkload(workload),
	)
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
	generationObserved := deployment.Generation > 0 && deployment.Status.ObservedGeneration >= deployment.Generation
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

func rolloutFromObservedWorkload(workload *runtimedomain.RuntimeObservedWorkload) *unstructured.Unstructured {
	if workload == nil {
		return nil
	}
	object := map[string]any{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Rollout",
		"metadata": map[string]any{
			"name": workloadName(workload),
		},
		"spec": map[string]any{
			"replicas": int64(workloadDesiredReplicas(workload)),
			"strategy": rolloutStrategyFromObservedWorkload(workload),
		},
		"status": map[string]any{
			"phase":               rolloutPhaseFromObservedWorkload(workload),
			"message":             rolloutMessageFromObservedWorkload(workload),
			"observedGeneration":  workload.ObservedGeneration,
			"updatedReplicas":     int64(workload.UpdatedReplicas),
			"readyReplicas":       int64(workload.ReadyReplicas),
			"availableReplicas":   int64(workload.AvailableReplicas),
			"unavailableReplicas": int64(workload.UnavailableReplicas),
		},
	}
	if workload.ObservedGeneration > 0 {
		object["metadata"].(map[string]any)["generation"] = workload.ObservedGeneration
	}
	if stepIndex, ok := rolloutCurrentStepIndexFromObservedWorkload(workload); ok {
		object["status"].(map[string]any)["currentStepIndex"] = stepIndex
	}
	return &unstructured.Unstructured{Object: object}
}

func rolloutStrategyFromObservedWorkload(workload *runtimedomain.RuntimeObservedWorkload) map[string]any {
	if observedWorkloadLooksBlueGreen(workload) {
		return map[string]any{"blueGreen": map[string]any{}}
	}
	return map[string]any{"canary": map[string]any{}}
}

func rolloutPhaseFromObservedWorkload(workload *runtimedomain.RuntimeObservedWorkload) string {
	switch strings.ToLower(workloadSummaryStatus(workload)) {
	case "healthy", "completed":
		return "Healthy"
	case "degraded", "error", "failed":
		return "Degraded"
	case "paused":
		return "Paused"
	case "progressing":
		return "Progressing"
	default:
		return "Progressing"
	}
}

func rolloutMessageFromObservedWorkload(workload *runtimedomain.RuntimeObservedWorkload) string {
	for _, condition := range workloadConditions(workload) {
		if msg := strings.TrimSpace(condition.Message); msg != "" {
			return msg
		}
	}
	return ""
}

func rolloutCurrentStepIndexFromObservedWorkload(workload *runtimedomain.RuntimeObservedWorkload) (int64, bool) {
	for _, condition := range workloadConditions(workload) {
		value := strings.TrimSpace(condition.Reason)
		if value == "" {
			value = strings.TrimSpace(condition.Message)
		}
		if value == "" {
			continue
		}
		stepIndex, ok := canaryStepIndexFromText(value)
		if ok {
			return stepIndex, true
		}
	}
	return 0, false
}

func observedWorkloadLooksBlueGreen(workload *runtimedomain.RuntimeObservedWorkload) bool {
	for _, condition := range workloadConditions(workload) {
		if strings.Contains(strings.ToLower(strings.TrimSpace(condition.Type)), "bluegreen") {
			return true
		}
		if strings.Contains(strings.ToLower(strings.TrimSpace(condition.Reason)), "bluegreen") {
			return true
		}
		if strings.Contains(strings.ToLower(strings.TrimSpace(condition.Message)), "blue-green") {
			return true
		}
	}
	for _, value := range []string{
		workloadAnnotationValue(workload, "devflow.io/rollout-strategy"),
		workloadAnnotationValue(workload, "devflow.io/release-strategy"),
		workloadLabelValue(workload, "devflow.io/rollout-strategy"),
		workloadLabelValue(workload, "devflow.io/release-strategy"),
	} {
		if strings.Contains(strings.ToLower(strings.TrimSpace(value)), "blue") {
			return true
		}
	}
	return false
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

func workloadPrimaryName(workload *runtimedomain.RuntimeObservedWorkload) string {
	if workload == nil {
		return "application"
	}
	if value := workloadLabelValue(workload, "app.kubernetes.io/name"); value != "" {
		return value
	}
	return workloadName(workload)
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

func workloadConditions(workload *runtimedomain.RuntimeObservedWorkload) []runtimedomain.RuntimeObservedWorkloadCondition {
	if workload == nil {
		return nil
	}
	return workload.Conditions
}

func workloadAnnotationValue(workload *runtimedomain.RuntimeObservedWorkload, key string) string {
	if workload == nil || len(workload.Annotations) == 0 {
		return ""
	}
	return strings.TrimSpace(workload.Annotations[key])
}

func workloadLabelValue(workload *runtimedomain.RuntimeObservedWorkload, key string) string {
	if workload == nil || len(workload.Labels) == 0 {
		return ""
	}
	return strings.TrimSpace(workload.Labels[key])
}

func canaryStepIndexFromText(value string) (int64, bool) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch {
	case strings.Contains(normalized, "100"):
		return 6, true
	case strings.Contains(normalized, "60"):
		return 4, true
	case strings.Contains(normalized, "30"):
		return 2, true
	case strings.Contains(normalized, "10"):
		return 0, true
	default:
		return 0, false
	}
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
