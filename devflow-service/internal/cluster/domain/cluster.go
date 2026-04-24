package domain

import "time"

type Cluster struct {
	BaseModel

	Name                string      `json:"name" db:"name"`
	Server              string      `json:"server" db:"server"`
	KubeConfig          string      `json:"kubeconfig,omitempty" db:"kubeconfig"`
	ArgoCDClusterName   string      `json:"argocd_cluster_name,omitempty" db:"argocd_cluster_name"`
	Description         string      `json:"description,omitempty" db:"description"`
	Labels              []LabelItem `json:"labels,omitempty" db:"labels"`
	OnboardingReady     bool        `json:"onboarding_ready" db:"onboarding_ready"`
	OnboardingError     string      `json:"onboarding_error,omitempty" db:"onboarding_error"`
	OnboardingCheckedAt *time.Time  `json:"onboarding_checked_at,omitempty" db:"onboarding_checked_at"`
}

func (Cluster) CollectionName() string { return "clusters" }
