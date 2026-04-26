package downstream

import (
	"context"
	"fmt"

	"github.com/bsonger/devflow-service/internal/shared/downstreamhttp"
)

type ApplicationEnvironment struct {
	ID            string                    `json:"id"`
	ApplicationID string                    `json:"application_id"`
	Environment   ApplicationEnvironmentRef `json:"environment,omitempty"`
}

type ApplicationEnvironmentRef struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type OrchestratorManifestClient struct{ *downstreamhttp.Client }

func NewOrchestratorManifestClient(baseURL string) *OrchestratorManifestClient {
	return &OrchestratorManifestClient{Client: downstreamhttp.New(baseURL)}
}

func (c *OrchestratorManifestClient) GetApplicationEnvironment(ctx context.Context, applicationId, environmentId string) (*ApplicationEnvironment, error) {
	var out ApplicationEnvironment
	if err := c.GetEnvelopeData(ctx, fmt.Sprintf("/api/v1/applications/%s/environments/%s", applicationId, environmentId), &out); err != nil {
		return nil, err
	}
	return &out, nil
}
