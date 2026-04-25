package runtimeclient

import (
	"context"
	"fmt"
	"time"

	"github.com/bsonger/devflow-service/internal/shared/downstreamhttp"
	"github.com/google/uuid"
)

var ErrRuntimeServiceUnavailable = downstreamhttp.ErrServiceUnavailable

type Lookup interface {
	GetRuntimeSpec(context.Context, uuid.UUID) (*RuntimeSpec, error)
	GetRuntimeSpecRevision(context.Context, uuid.UUID) (*RuntimeSpecRevision, error)
}

type Client struct{ *downstreamhttp.Client }

type RuntimeSpec struct {
	ID            uuid.UUID `json:"id"`
	ApplicationID uuid.UUID `json:"application_id"`
	Environment   string    `json:"environment"`
}

type RuntimeSpecRevision struct {
	ID            uuid.UUID `json:"id"`
	RuntimeSpecID uuid.UUID `json:"runtime_spec_id"`
}

func New(baseURL string) *Client {
	return &Client{Client: downstreamhttp.NewWithOptions(baseURL, downstreamhttp.WithTimeout(5*time.Second))}
}

func (c *Client) GetRuntimeSpec(ctx context.Context, id uuid.UUID) (*RuntimeSpec, error) {
	var out RuntimeSpec
	if err := c.GetEnvelopeData(ctx, fmt.Sprintf("/api/v1/runtime-specs/%s", id), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetRuntimeSpecRevision(ctx context.Context, id uuid.UUID) (*RuntimeSpecRevision, error) {
	var out RuntimeSpecRevision
	if err := c.GetEnvelopeData(ctx, fmt.Sprintf("/api/v1/runtime-spec-revisions/%s", id), &out); err != nil {
		return nil, err
	}
	return &out, nil
}
