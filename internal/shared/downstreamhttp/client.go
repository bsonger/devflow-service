package downstreamhttp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var ErrServiceUnavailable = errors.New("downstream service is not configured")

type Client struct {
	baseURL string
	http    *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) GetEnvelopeData(ctx context.Context, path string, out any) error {
	if c == nil || c.baseURL == "" {
		return ErrServiceUnavailable
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downstream request failed: %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var envelope map[string]json.RawMessage
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&envelope); err != nil {
		return err
	}
	data, ok := envelope["data"]
	if !ok {
		return json.Unmarshal(body, out)
	}
	return json.Unmarshal(data, out)
}
