package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type environmentResolver interface {
	ResolveName(ctx context.Context, environmentID string) (string, error)
}

type httpEnvironmentResolver struct {
	baseURL string
	client  *http.Client
}

type environmentEnvelope struct {
	Data struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"data"`
}

func newHTTPEnvironmentResolver(baseURL string) environmentResolver {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if trimmed == "" {
		return nil
	}
	return &httpEnvironmentResolver{
		baseURL: trimmed,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func ResolveEnvironmentResolver(baseURL string) environmentResolver {
	return newHTTPEnvironmentResolver(baseURL)
}

func (r *httpEnvironmentResolver) ResolveName(ctx context.Context, environmentID string) (string, error) {
	trimmedID := strings.TrimSpace(environmentID)
	if trimmedID == "" {
		return "", fmt.Errorf("environment id is required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.baseURL+"/api/v1/environments/"+trimmedID, nil)
	if err != nil {
		return "", err
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("resolve environment %s: unexpected status %d", trimmedID, resp.StatusCode)
	}
	var envelope environmentEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return "", err
	}
	name := strings.TrimSpace(envelope.Data.Name)
	if name == "" {
		return "", fmt.Errorf("resolve environment %s: empty environment name", trimmedID)
	}
	return name, nil
}
