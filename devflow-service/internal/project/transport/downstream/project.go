package downstream

import (
	"context"
	"fmt"

	"github.com/bsonger/devflow-service/internal/shared/downstreamhttp"
)

type Project struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Client struct{ *downstreamhttp.Client }

func New(baseURL string) *Client {
	return &Client{Client: downstreamhttp.New(baseURL)}
}

func (c *Client) GetProject(ctx context.Context, id string) (*Project, error) {
	var out Project
	if err := c.GetEnvelopeData(ctx, fmt.Sprintf("/api/v1/projects/%s", id), &out); err != nil {
		return nil, err
	}
	return &out, nil
}
