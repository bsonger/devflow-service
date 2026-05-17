package writeback

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	platformobs "github.com/bsonger/devflow-service/internal/platform/runtime/observability"
	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const ReleaseObserverTokenHeader = "X-Devflow-Observer-Token"

type ReleaseStepWrite struct {
	StepCode string
	Status   releasedomain.StepStatus
	Progress int32
	Message  string
}

type WriteReleaseStepsInput struct {
	ReleaseID            uuid.UUID
	ApplicationID        uuid.UUID
	EnvironmentID        string
	Namespace            string
	ObservedWorkloadKind string
	ObservedWorkloadName string
	Phase                releasedomain.StepStatus
	Progress             int32
	Message              string
	StepWrites           []ReleaseStepWrite
}

type ReleaseWriter struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

type WritebackError struct {
	Path       string
	StatusCode int
}

func (e *WritebackError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("release rollout writeback failed: path=%s status=%d", e.Path, e.StatusCode)
}

func (e *WritebackError) NotFound() bool {
	return e != nil && e.StatusCode == http.StatusNotFound
}

func IsNotFound(err error) bool {
	var target *WritebackError
	return errors.As(err, &target) && target.NotFound()
}

func NewReleaseWriter(baseURL, token string, httpClient *http.Client) *ReleaseWriter {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &ReleaseWriter{
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token:      strings.TrimSpace(token),
		httpClient: httpClient,
	}
}

func (w *ReleaseWriter) WriteReleaseSteps(ctx context.Context, input WriteReleaseStepsInput) error {
	for _, step := range input.StepWrites {
		if err := w.postStep(ctx, input.ReleaseID, step.StepCode, step.Status, step.Progress, step.Message); err != nil {
			return err
		}
	}
	return nil
}

func (w *ReleaseWriter) PostJSON(ctx context.Context, path string, payload any) error {
	return w.PostJSONWithCall(ctx, path, payload, platformobs.DependencyCall{
		Kind:      "http",
		Target:    "release_service",
		Operation: "release_rollout_writeback",
	})
}

func (w *ReleaseWriter) PostJSONWithCall(ctx context.Context, path string, payload any, call platformobs.DependencyCall) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if strings.TrimSpace(call.Kind) == "" {
		call.Kind = "http"
	}
	if strings.TrimSpace(call.Target) == "" {
		call.Target = "release_service"
	}
	return platformobs.ObserveDependency(ctx, call, func(depCtx context.Context) error {
		req, err := http.NewRequestWithContext(depCtx, http.MethodPost, w.baseURL+path, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		if w.token != "" {
			req.Header.Set(ReleaseObserverTokenHeader, w.token)
		}
		resp, err := w.httpClient.Do(req)
		if err != nil {
			return err
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		return &WritebackError{Path: path, StatusCode: resp.StatusCode}
	})
}

func (w *ReleaseWriter) postStep(ctx context.Context, releaseID uuid.UUID, stepCode string, status releasedomain.StepStatus, progress int32, message string) error {
	payload := map[string]any{
		"release_id": releaseID.String(),
		"step_code":  strings.TrimSpace(stepCode),
		"status":     string(status),
		"progress":   progress,
		"message":    strings.TrimSpace(message),
	}
	return w.PostJSONWithCall(ctx, "/api/v1/verify/release/steps", payload, platformobs.DependencyCall{
		Kind:      "http",
		Target:    "release_service",
		Operation: "release_rollout_writeback",
		LogFields: []zap.Field{
			zap.String("release_id", releaseID.String()),
			zap.String("step_code", strings.TrimSpace(stepCode)),
		},
	})
}
