package http

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	manifestservice "github.com/bsonger/devflow-service/internal/manifest/service"
	"github.com/bsonger/devflow-service/internal/release/support"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var releaseFederationHTTPClient = &http.Client{Timeout: 30 * time.Second}
var releaseGetLocalManifest = func(ctx context.Context, id uuid.UUID) (*manifestdomain.Manifest, error) {
	return manifestservice.ManifestService.Get(ctx, id)
}
var releaseResolveDeployTarget = support.ResolveDeployTarget
var releaseCurrentRuntimeConfig = support.CurrentRuntimeConfig

func releaseProxyBaseURL() string {
	return strings.TrimSpace(releaseCurrentRuntimeConfig().Downstream.ReleaseServiceBaseURL)
}

func releaseShouldProxyByManifest(ctx context.Context, manifestID uuid.UUID, environmentID string) bool {
	if releaseProxyBaseURL() == "" || manifestID == uuid.Nil || strings.TrimSpace(environmentID) == "" {
		return false
	}
	manifest, err := releaseGetLocalManifest(ctx, manifestID)
	if err != nil || manifest == nil {
		return false
	}
	return releaseShouldProxyByTarget(ctx, manifest.ApplicationID, environmentID)
}

func releaseShouldProxyByTarget(ctx context.Context, applicationID uuid.UUID, environmentID string) bool {
	if releaseProxyBaseURL() == "" || applicationID == uuid.Nil || strings.TrimSpace(environmentID) == "" {
		return false
	}
	target, err := releaseResolveDeployTarget(ctx, applicationID.String(), environmentID)
	if err != nil || target == nil {
		return false
	}
	return !releaseControlPlaneOwnsTarget(strings.TrimSpace(releaseCurrentRuntimeConfig().ControlPlaneID), target)
}

func releaseControlPlaneOwnsTarget(controlPlaneID string, target *support.DeployTarget) bool {
	if target == nil {
		return true
	}
	expected := releaseExpectedControlPlaneIDForEnvironment(target.EnvironmentName)
	if expected == "" {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(controlPlaneID), expected)
}

func releaseExpectedControlPlaneIDForEnvironment(environmentName string) string {
	switch strings.ToLower(strings.TrimSpace(environmentName)) {
	case "":
		return ""
	case "production", "prod":
		return "devflow-production"
	case "pre-production", "preproduction", "staging":
		return "devflow-pre-production"
	default:
		return ""
	}
}

func proxyReleaseRequest(c *gin.Context, method, path string, body []byte, contentType string) bool {
	baseURL := releaseProxyBaseURL()
	if baseURL == "" {
		return false
	}
	targetURL, err := url.Parse(strings.TrimRight(baseURL, "/") + path)
	if err != nil {
		return false
	}
	if raw := c.Request.URL.RawQuery; raw != "" {
		targetURL.RawQuery = raw
	}
	reqBody := io.Reader(http.NoBody)
	if len(body) > 0 {
		reqBody = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(c.Request.Context(), method, targetURL.String(), reqBody)
	if err != nil {
		return false
	}
	for key, values := range c.Request.Header {
		if strings.EqualFold(key, "Host") || strings.EqualFold(key, "Content-Length") {
			continue
		}
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	if contentType != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := releaseFederationHTTPClient.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	for key, values := range resp.Header {
		for _, value := range values {
			c.Writer.Header().Add(key, value)
		}
	}
	c.Status(resp.StatusCode)
	_, _ = io.Copy(c.Writer, resp.Body)
	c.Abort()
	return true
}
