package writeback

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
	Path         string
	StatusCode   int
	ResponseBody string
}

func (e *WritebackError) Error() string {
	if e == nil {
		return ""
	}
	if e.ResponseBody == "" {
		return fmt.Sprintf("release rollout writeback failed: path=%s status=%d", e.Path, e.StatusCode)
	}
	return fmt.Sprintf("release rollout writeback failed: path=%s status=%d body=%q", e.Path, e.StatusCode, e.ResponseBody)
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
		responseBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if readErr != nil {
			return readErr
		}
		excerpt := trimResponseBodyExcerpt(string(responseBody), 512)
		if excerpt != "" {
			call.LogFields = append(call.LogFields,
				zap.String("status_code", fmt.Sprintf("%d", resp.StatusCode)),
				zap.String("response_body_excerpt", excerpt),
			)
		} else {
			call.LogFields = append(call.LogFields,
				zap.String("status_code", fmt.Sprintf("%d", resp.StatusCode)),
			)
		}
		return &WritebackError{Path: path, StatusCode: resp.StatusCode, ResponseBody: excerpt}
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
	err := w.PostJSONWithCall(ctx, "/api/v1/verify/release/steps", payload, platformobs.DependencyCall{
		Kind:      "http",
		Target:    "release_service",
		Operation: "release_rollout_writeback",
		LogFields: []zap.Field{
			zap.String("release_id", releaseID.String()),
			zap.String("step_code", strings.TrimSpace(stepCode)),
		},
	})
	statusCode := "none"
	result := "ok"
	var writebackErr *WritebackError
	if errors.As(err, &writebackErr) {
		statusCode = fmt.Sprintf("%d", writebackErr.StatusCode)
	}
	if err != nil {
		result = "error"
	}
	platformobs.RecordRuntimeReleaseWriteback(ctx, stepCode, result, statusCode)
	return err
}

func trimResponseBodyExcerpt(body string, limit int) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}
	if limit <= 0 {
		return ""
	}
	if len(body) > limit {
		return body[:limit]
	}
	return body
}
