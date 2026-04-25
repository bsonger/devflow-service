package service

import (
	"context"
	"strings"
	"time"

	"github.com/bsonger/devflow-service/internal/shared/downstreamhttp"
	sharederrs "github.com/bsonger/devflow-service/internal/shared/errs"
)

type environmentResolver interface {
	ResolveName(ctx context.Context, environmentID string) (string, error)
}

type httpEnvironmentResolver struct {
	client *downstreamhttp.Client
}

type environmentDoc struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func newHTTPEnvironmentResolver(baseURL string) environmentResolver {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if trimmed == "" {
		return nil
	}
	return &httpEnvironmentResolver{
		client: downstreamhttp.NewWithOptions(trimmed, downstreamhttp.WithTimeout(10*time.Second)),
	}
}

func ResolveEnvironmentResolver(baseURL string) environmentResolver {
	return newHTTPEnvironmentResolver(baseURL)
}

func (r *httpEnvironmentResolver) ResolveName(ctx context.Context, environmentID string) (string, error) {
	trimmedID := strings.TrimSpace(environmentID)
	if trimmedID == "" {
		return "", sharederrs.Required("environment_id")
	}
	var doc environmentDoc
	if err := r.client.GetEnvelopeData(ctx, "/api/v1/environments/"+trimmedID, &doc); err != nil {
		return "", err
	}
	name := strings.TrimSpace(doc.Name)
	if name == "" {
		return "", sharederrs.FailedPrecondition("environment name is empty")
	}
	return name, nil
}
