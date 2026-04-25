package downstream

import (
	"context"
	"fmt"

	"github.com/bsonger/devflow-service/internal/shared/downstreamhttp"
)

type Cluster struct {
	ID                  string `json:"id"`
	Name                string `json:"name"`
	Server              string `json:"server"`
	OnboardingReady     bool   `json:"onboarding_ready"`
	OnboardingError     string `json:"onboarding_error,omitempty"`
	OnboardingCheckedAt string `json:"onboarding_checked_at,omitempty"`
}

type Client struct{ *downstreamhttp.Client }

func New(baseURL string) *Client {
	return &Client{Client: downstreamhttp.New(baseURL)}
}

func (c *Client) GetCluster(ctx context.Context, id string) (*Cluster, error) {
	var out Cluster
	if err := c.GetEnvelopeData(ctx, fmt.Sprintf("/api/v1/clusters/%s", id), &out); err != nil {
		return nil, err
	}
	return &out, nil
}
