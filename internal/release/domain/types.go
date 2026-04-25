package domain

import "time"

type ReleaseType string
type Internet string
type StepStatus string
type ImageStatus string
type ManifestStatus string
type ReleaseStatus string
type IntentKind string
type IntentStatus string

const (
	Normal    ReleaseType = "normal"
	Canary    ReleaseType = "canary"
	BlueGreen ReleaseType = "blue-green"
)

const (
	Internal Internet = "internal"
	External Internet = "external"
)

const (
	StepPending   StepStatus = "Pending"
	StepRunning   StepStatus = "Running"
	StepSucceeded StepStatus = "Succeeded"
	StepFailed    StepStatus = "Failed"
)

const (
	ImagePending   ImageStatus = "Pending"
	ImageRunning   ImageStatus = "Running"
	ImageSucceeded ImageStatus = "Succeeded"
	ImageFailed    ImageStatus = "Failed"
)

const (
	ManifestPending ManifestStatus = "Pending"
	ManifestReady   ManifestStatus = "Ready"
	ManifestFailed  ManifestStatus = "Failed"
)

const (
	ReleasePending     ReleaseStatus = "Pending"
	ReleaseRunning     ReleaseStatus = "Running"
	ReleaseSucceeded   ReleaseStatus = "Succeeded"
	ReleaseFailed      ReleaseStatus = "Failed"
	ReleaseRollingBack ReleaseStatus = "RollingBack"
	ReleaseRolledBack  ReleaseStatus = "RolledBack"
	ReleaseSyncing     ReleaseStatus = "Syncing"
	ReleaseSyncFailed  ReleaseStatus = "SyncFailed"
)

const (
	IntentKindBuild   IntentKind = "build"
	IntentKindRelease IntentKind = "release"
)

const (
	IntentPending   IntentStatus = "Pending"
	IntentRunning   IntentStatus = "Running"
	IntentSucceeded IntentStatus = "Succeeded"
	IntentFailed    IntentStatus = "Failed"
)

const (
	ReleaseInstall  string = "Install"
	ReleaseUpgrade  string = "Upgrade"
	ReleaseRollback string = "Rollback"

	ReleaseIDLabel = "devflow.io/release-id"
)

type Port struct {
	Name       string `json:"name"`
	Port       int    `json:"port"`
	TargetPort int    `json:"target_port"`
}

type EnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type ImageTask struct {
	TaskName  string     `json:"task_name"`
	TaskRun   string     `json:"task_run,omitempty"`
	Status    StepStatus `json:"status"`
	StartTime *time.Time `json:"start_time,omitempty"`
	EndTime   *time.Time `json:"end_time,omitempty"`
	Message   string     `json:"message,omitempty"`
}

type ReleaseStep struct {
	Name      string     `json:"name"`
	Progress  int32      `json:"progress"`
	Status    StepStatus `json:"status"`
	Message   string     `json:"message,omitempty"`
	StartTime *time.Time `json:"start_time,omitempty"`
	EndTime   *time.Time `json:"end_time,omitempty"`
}
