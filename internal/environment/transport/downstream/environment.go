package downstream

import (
	"context"
	"fmt"

	"github.com/bsonger/devflow-service/internal/shared/downstreamhttp"
)

type Environment struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ClusterID string `json:"cluster_id"`
}

type Client struct{ *downstreamhttp.Client }

func New(baseURL string) *Client {
	return &Client{Client: downstreamhttp.New(baseURL)}
}

func (c *Client) GetEnvironment(ctx context.Context, id string) (*Environment, error) {
	var out Environment
	if err := c.GetEnvelopeData(ctx, fmt.Sprintf("/api/v1/environments/%s", id), &out); err != nil {
		return nil, err
	}
	return &out, nil
}
