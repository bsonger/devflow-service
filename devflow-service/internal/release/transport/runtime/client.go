package runtimeclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

var ErrRuntimeServiceUnavailable = errors.New("runtime service is not configured")

type Lookup interface {
	GetRuntimeSpec(context.Context, uuid.UUID) (*RuntimeSpec, error)
	GetRuntimeSpecRevision(context.Context, uuid.UUID) (*RuntimeSpecRevision, error)
}

type Client struct {
	baseURL string
	http    *http.Client
}

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
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *Client) GetRuntimeSpec(ctx context.Context, id uuid.UUID) (*RuntimeSpec, error) {
	if c == nil || c.baseURL == "" {
		return nil, ErrRuntimeServiceUnavailable
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/v1/runtime-specs/%s", c.baseURL, id), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("runtime spec lookup failed: %s", resp.Status)
	}
	var out RuntimeSpec
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetRuntimeSpecRevision(ctx context.Context, id uuid.UUID) (*RuntimeSpecRevision, error) {
	if c == nil || c.baseURL == "" {
		return nil, ErrRuntimeServiceUnavailable
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/v1/runtime-spec-revisions/%s", c.baseURL, id), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("runtime revision lookup failed: %s", resp.Status)
	}
	var out RuntimeSpecRevision
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}
