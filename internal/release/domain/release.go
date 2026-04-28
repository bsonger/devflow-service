package domain

import "github.com/google/uuid"

type Release struct {
	BaseModel

	ExecutionIntentID     *uuid.UUID       `json:"execution_intent_id,omitempty" db:"execution_intent_id"`
	ApplicationID         uuid.UUID        `json:"application_id" db:"application_id"`
	ManifestID            uuid.UUID        `json:"manifest_id" db:"manifest_id"`
	EnvironmentID         string           `json:"environment_id" db:"env"`
	Strategy              string           `json:"strategy" db:"strategy"`
	RoutesSnapshot        []ReleaseRoute   `json:"routes_snapshot,omitempty" db:"routes_snapshot"`
	AppConfigSnapshot     ReleaseAppConfig `json:"app_config_snapshot" db:"app_config_snapshot"`
	ArtifactRepository    string           `json:"artifact_repository,omitempty" db:"artifact_repository"`
	ArtifactTag           string           `json:"artifact_tag,omitempty" db:"artifact_tag"`
	ArtifactDigest        string           `json:"artifact_digest,omitempty" db:"artifact_digest"`
	ArtifactRef           string           `json:"artifact_ref,omitempty" db:"artifact_ref"`
	Type                  string           `json:"type" db:"type"`
	Steps                 []ReleaseStep    `json:"steps,omitempty" db:"steps"`
	Status                ReleaseStatus    `json:"status" db:"status"`
	ArgoCDApplicationName string           `json:"argocd_application_name,omitempty" db:"argocd_application_name"`
	ExternalRef           string           `json:"external_ref,omitempty" db:"external_ref"`
}

type ReleaseRoute struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name"`
	Host        string `json:"host"`
	Path        string `json:"path"`
	ServiceName string `json:"service_name"`
	ServicePort int    `json:"service_port"`
}

type ReleaseFile struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type ReleaseAppConfig struct {
	ID           string            `json:"id,omitempty"`
	Name         string            `json:"name,omitempty"`
	MountPath    string            `json:"mount_path,omitempty"`
	Files        []ReleaseFile     `json:"files,omitempty"`
	Data         map[string]string `json:"data,omitempty"`
	SourcePath   string            `json:"source_path,omitempty"`
	RevisionID   string            `json:"revision_id,omitempty"`
	SourceCommit string            `json:"source_commit,omitempty"`
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

func NormalizeReleaseStrategy(value string) string {
	switch value {
	case "blue-green":
		return string(ReleaseStrategyBlueGreen)
	case string(ReleaseStrategyBlueGreen), string(ReleaseStrategyCanary):
		return value
	case "", string(ReleaseStrategyRolling):
		return string(ReleaseStrategyRolling)
	default:
		return value
	}
}

func ReleaseStrategyToType(value string) ReleaseType {
	switch NormalizeReleaseStrategy(value) {
	case string(ReleaseStrategyCanary):
		return Canary
	case string(ReleaseStrategyBlueGreen):
		return BlueGreen
	default:
		return Normal
	}
}

func DefaultReleaseSteps(strategy ReleaseType, releaseAction string) []ReleaseStep {
	switch strategy {
	case Canary:
		return []ReleaseStep{
			newReleaseStep("freeze_inputs", "Freeze release inputs"),
			newReleaseStep("render_deployment_bundle", "Render deployment bundle"),
			newReleaseStep("publish_bundle", "Publish bundle to OCI"),
			newReleaseStep("create_argocd_application", "Create ArgoCD application"),
			newReleaseStep("deploy_canary", "Deploy canary"),
			newReleaseStep("canary_10", "Canary 10% traffic"),
			newReleaseStep("canary_30", "Canary 30% traffic"),
			newReleaseStep("canary_60", "Canary 60% traffic"),
			newReleaseStep("canary_100", "Canary 100% traffic"),
			newReleaseStep("finalize_release", "Finalize release"),
		}
	case BlueGreen:
		return []ReleaseStep{
			newReleaseStep("freeze_inputs", "Freeze release inputs"),
			newReleaseStep("render_deployment_bundle", "Render deployment bundle"),
			newReleaseStep("publish_bundle", "Publish bundle to OCI"),
			newReleaseStep("create_argocd_application", "Create ArgoCD application"),
			newReleaseStep("deploy_preview", "Deploy preview"),
			newReleaseStep("observe_preview", "Observe preview"),
			newReleaseStep("switch_traffic", "Switch traffic"),
			newReleaseStep("verify_active", "Verify active"),
			newReleaseStep("finalize_release", "Finalize release"),
		}
	default:
		_ = releaseAction
		return []ReleaseStep{
			newReleaseStep("freeze_inputs", "Freeze release inputs"),
			newReleaseStep("render_deployment_bundle", "Render deployment bundle"),
			newReleaseStep("publish_bundle", "Publish bundle to OCI"),
			newReleaseStep("create_argocd_application", "Create ArgoCD application"),
			newReleaseStep("start_deployment", "Start deployment"),
			newReleaseStep("observe_rollout", "Observe rollout"),
			newReleaseStep("finalize_release", "Finalize release"),
		}
	}
}

func newReleaseStep(code, name string) ReleaseStep {
	return ReleaseStep{
		Code:     code,
		Name:     name,
		Progress: 0,
		Status:   StepPending,
	}
}
