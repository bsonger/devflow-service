package writeback

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/google/uuid"
)

func TestReleaseWriterPostsExpectedObserveRolloutSteps(t *testing.T) {
	type stepPayload struct {
		ReleaseID string `json:"release_id"`
		StepCode  string `json:"step_code"`
		Status    string `json:"status"`
		Progress  int32  `json:"progress"`
		Message   string `json:"message"`
	}

	var (
		serverCalls []string
		headers     []string
		payloads    []stepPayload
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalls = append(serverCalls, r.URL.Path)
		headers = append(headers, r.Header.Get(ReleaseObserverTokenHeader))
		defer r.Body.Close()
		var payload stepPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		payloads = append(payloads, payload)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	writer := NewReleaseWriter(server.URL, "token-a", &http.Client{Timeout: 2 * time.Second})

	input := WriteReleaseStepsInput{
		ReleaseID:            uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		ApplicationID:        uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		EnvironmentID:        "env-1",
		Namespace:            "devflow",
		ObservedWorkloadKind: "Deployment",
		ObservedWorkloadName: "demo-api",
		Phase:                releasedomain.StepRunning,
		Progress:             10,
		Message:              "waiting for workload demo-api in namespace devflow",
		StepWrites: []ReleaseStepWrite{{
			StepCode: "observe_rollout",
			Status:   releasedomain.StepRunning,
			Progress: 10,
			Message:  "waiting for workload demo-api in namespace devflow",
		}},
	}

	if err := writer.WriteReleaseSteps(context.Background(), input); err != nil {
		t.Fatalf("WriteReleaseSteps failed: %v", err)
	}
	if len(serverCalls) == 0 {
		t.Fatal("expected writeback HTTP calls")
	}
	if got, want := serverCalls[0], "/api/v1/verify/release/steps"; got != want {
		t.Fatalf("path = %q want %q", got, want)
	}
	if got, want := headers[0], "token-a"; got != want {
		t.Fatalf("header = %q want %q", got, want)
	}
	if len(payloads) != 1 {
		t.Fatalf("payload count = %d want 1", len(payloads))
	}
	if got, want := payloads[0].ReleaseID, input.ReleaseID.String(); got != want {
		t.Fatalf("release_id = %q want %q", got, want)
	}
	if got, want := payloads[0].StepCode, "observe_rollout"; got != want {
		t.Fatalf("step_code = %q want %q", got, want)
	}
	if got, want := payloads[0].Status, string(releasedomain.StepRunning); got != want {
		t.Fatalf("status = %q want %q", got, want)
	}
	if got, want := payloads[0].Progress, int32(10); got != want {
		t.Fatalf("progress = %d want %d", got, want)
	}
	if got, want := payloads[0].Message, "waiting for workload demo-api in namespace devflow"; got != want {
		t.Fatalf("message = %q want %q", got, want)
	}
}

func TestReleaseWriterPostJSONReturnsTypedNotFoundError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":{"code":"not_found","message":"not found"}}`)
	}))
	defer server.Close()

	writer := NewReleaseWriter(server.URL, "", server.Client())
	err := writer.PostJSON(context.Background(), "/api/v1/verify/release/steps", map[string]any{"release_id": uuid.NewString()})
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsNotFound(err) {
		t.Fatalf("expected typed not-found writeback error, got %v", err)
	}
}
