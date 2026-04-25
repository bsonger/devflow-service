package downstream

import (
	"context"
	"fmt"

	"github.com/bsonger/devflow-service/internal/shared/downstreamhttp"
)

type Application struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	Name      string `json:"name"`
}

type Client struct{ *downstreamhttp.Client }

func New(baseURL string) *Client {
	return &Client{Client: downstreamhttp.New(baseURL)}
}

func (c *Client) GetApplication(ctx context.Context, id string) (*Application, error) {
	var out Application
	if err := c.GetEnvelopeData(ctx, fmt.Sprintf("/api/v1/applications/%s", id), &out); err != nil {
		return nil, err
	}
	return &out, nil
}
