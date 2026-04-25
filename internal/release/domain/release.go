package domain

import "github.com/google/uuid"

type Release struct {
	BaseModel

	ExecutionIntentID *uuid.UUID    `json:"execution_intent_id,omitempty" db:"execution_intent_id"`
	ApplicationID     uuid.UUID     `json:"application_id" db:"application_id"`
	ManifestID        uuid.UUID     `json:"manifest_id" db:"manifest_id"`
	ImageID           uuid.UUID     `json:"image_id" db:"image_id"`
	Env               string        `json:"env" db:"env"`
	Type              string        `json:"type" db:"type"`
	Steps             []ReleaseStep `json:"steps,omitempty" db:"steps"`
	Status            ReleaseStatus `json:"status" db:"status"`
	ExternalRef       string        `json:"external_ref,omitempty" db:"external_ref"`
}

func (r *Release) CollectionName() string { return "releases" }

func DeriveReleaseStatusFromSteps(releaseAction string, currentStatus ReleaseStatus, steps []ReleaseStep) ReleaseStatus {
	switch currentStatus {
	case ReleaseSucceeded, ReleaseFailed, ReleaseRolledBack, ReleaseSyncFailed:
		return currentStatus
	}

	if len(steps) == 0 {
		if currentStatus == "" {
			return ReleasePending
		}
		return currentStatus
	}

	allSucceeded := true
	anyFailed := false
	anyStarted := false

	for _, step := range steps {
		switch step.Status {
		case StepFailed:
			anyFailed = true
			allSucceeded = false
		case StepSucceeded:
			anyStarted = true
		case StepRunning:
			anyStarted = true
			allSucceeded = false
		default:
			allSucceeded = false
		}
	}

	if anyFailed {
		return ReleaseFailed
	}
	if allSucceeded {
		if releaseAction == ReleaseRollback {
			return ReleaseRolledBack
		}
		return ReleaseSucceeded
	}
	if anyStarted {
		return ReleaseRunning
	}
	if currentStatus == "" {
		return ReleasePending
	}
	return currentStatus
}

func DefaultReleaseSteps(strategy ReleaseType, releaseAction string) []ReleaseStep {
	applyStepName := "apply manifests"
	switch releaseAction {
	case ReleaseRollback:
		applyStepName = "apply rollback manifests"
	case ReleaseInstall:
		applyStepName = "apply install manifests"
	}

	stepNames := []string{
		"ensure namespace",
		"ensure pull secret",
		"ensure appproject destination",
		applyStepName,
	}
	switch strategy {
	case Canary:
		stepNames = append(stepNames,
			"canary 10% traffic",
			"canary 30% traffic",
			"canary 60% traffic",
			"canary 100% traffic",
		)
	case BlueGreen:
		stepNames = append(stepNames,
			"green ready",
			"switch traffic",
		)
	default:
		stepNames = append(stepNames, "deploy ready")
	}

	steps := make([]ReleaseStep, 0, len(stepNames))
	for _, name := range stepNames {
		steps = append(steps, ReleaseStep{
			Name:     name,
			Progress: 0,
			Status:   StepPending,
		})
	}

	return steps
}
