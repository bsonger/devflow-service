package downstream

import (
	"context"
	"fmt"

	"github.com/bsonger/devflow-service/internal/shared/downstreamhttp"
)

type ServicePort struct {
	Name        string `json:"name,omitempty"`
	ServicePort int    `json:"service_port"`
	TargetPort  int    `json:"target_port"`
	Protocol    string `json:"protocol,omitempty"`
}

type Service struct {
	ID    string        `json:"id"`
	Name  string        `json:"name"`
	Ports []ServicePort `json:"ports,omitempty"`
}

type Route struct {
	EnvironmentID string `json:"environment_id,omitempty"`
	ID            string `json:"id"`
	Name          string `json:"name"`
	Host          string `json:"host"`
	Path          string `json:"path"`
	ServiceName   string `json:"service_name"`
	ServicePort   int    `json:"service_port"`
}

type Client struct{ *downstreamhttp.Client }

func New(baseURL string) *Client {
	return &Client{Client: downstreamhttp.New(baseURL)}
}

func (c *Client) ListServices(ctx context.Context, applicationId string) ([]Service, error) {
	var out []Service
	if err := c.GetEnvelopeData(ctx, fmt.Sprintf("/api/v1/services?application_id=%s", applicationId), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) ListRoutes(ctx context.Context, applicationId string) ([]Route, error) {
	var out []Route
	if err := c.GetEnvelopeData(ctx, fmt.Sprintf("/api/v1/routes?application_id=%s", applicationId), &out); err != nil {
		return nil, err
	}
	return out, nil
}
