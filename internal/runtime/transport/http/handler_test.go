package http

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	runtimedomain "github.com/bsonger/devflow-service/internal/runtime/domain"
	runtimeservice "github.com/bsonger/devflow-service/internal/runtime/service"
	sharederrs "github.com/bsonger/devflow-service/internal/shared/errs"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type mockRuntimeService struct {
	getObservedWorkloadByApplicationEnvFunc func(context.Context, uuid.UUID, string) (*runtimedomain.RuntimeObservedWorkload, error)
	syncObservedWorkloadFunc                func(context.Context, runtimeservice.SyncObservedWorkloadInput) (*runtimedomain.RuntimeObservedWorkload, error)
	deleteObservedWorkloadFunc              func(context.Context, runtimeservice.DeleteObservedWorkloadInput) error
	listObservedPodsByApplicationEnvFunc    func(context.Context, uuid.UUID, string) ([]*runtimedomain.RuntimeObservedPod, error)
	syncObservedPodFunc                     func(context.Context, runtimeservice.SyncObservedPodInput) (*runtimedomain.RuntimeObservedPod, error)
	deleteObservedPodFunc                   func(context.Context, runtimeservice.DeleteObservedPodInput) error
	deletePodByApplicationEnvFunc           func(context.Context, uuid.UUID, string, string, string) (*runtimedomain.RuntimeActionAcknowledgement, error)
	restartDeploymentByApplicationEnvFunc   func(context.Context, uuid.UUID, string, string, string) (*runtimedomain.RuntimeActionAcknowledgement, error)
}

func (m *mockRuntimeService) GetObservedWorkloadByApplicationEnv(ctx context.Context, applicationID uuid.UUID, environment string) (*runtimedomain.RuntimeObservedWorkload, error) {
	if m.getObservedWorkloadByApplicationEnvFunc != nil {
		return m.getObservedWorkloadByApplicationEnvFunc(ctx, applicationID, environment)
	}
	return nil, sql.ErrNoRows
}

func (m *mockRuntimeService) SyncObservedWorkload(ctx context.Context, in runtimeservice.SyncObservedWorkloadInput) (*runtimedomain.RuntimeObservedWorkload, error) {
	if m.syncObservedWorkloadFunc != nil {
		return m.syncObservedWorkloadFunc(ctx, in)
	}
	return nil, nil
}

func (m *mockRuntimeService) DeleteObservedWorkload(ctx context.Context, in runtimeservice.DeleteObservedWorkloadInput) error {
	if m.deleteObservedWorkloadFunc != nil {
		return m.deleteObservedWorkloadFunc(ctx, in)
	}
	return nil
}

func (m *mockRuntimeService) ListObservedPodsByApplicationEnv(ctx context.Context, applicationID uuid.UUID, environment string) ([]*runtimedomain.RuntimeObservedPod, error) {
	if m.listObservedPodsByApplicationEnvFunc != nil {
		return m.listObservedPodsByApplicationEnvFunc(ctx, applicationID, environment)
	}
	return nil, nil
}

func (m *mockRuntimeService) SyncObservedPod(ctx context.Context, in runtimeservice.SyncObservedPodInput) (*runtimedomain.RuntimeObservedPod, error) {
	if m.syncObservedPodFunc != nil {
		return m.syncObservedPodFunc(ctx, in)
	}
	return nil, nil
}

func (m *mockRuntimeService) DeleteObservedPod(ctx context.Context, in runtimeservice.DeleteObservedPodInput) error {
	if m.deleteObservedPodFunc != nil {
		return m.deleteObservedPodFunc(ctx, in)
	}
	return nil
}

func (m *mockRuntimeService) DeletePodByApplicationEnv(ctx context.Context, applicationID uuid.UUID, environment, podName, operator string) (*runtimedomain.RuntimeActionAcknowledgement, error) {
	if m.deletePodByApplicationEnvFunc != nil {
		return m.deletePodByApplicationEnvFunc(ctx, applicationID, environment, podName, operator)
	}
	return nil, nil
}

func (m *mockRuntimeService) RestartDeploymentByApplicationEnv(ctx context.Context, applicationID uuid.UUID, environment, deploymentName, operator string) (*runtimedomain.RuntimeActionAcknowledgement, error) {
	if m.restartDeploymentByApplicationEnvFunc != nil {
		return m.restartDeploymentByApplicationEnvFunc(ctx, applicationID, environment, deploymentName, operator)
	}
	return nil, nil
}

func setupRuntimeTestRouter(h *Handler, token string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api/v1")
	h.RegisterRoutes(api)
	internal := api.Group("")
	internal.Use(RequireObserverToken(token))
	h.RegisterInternalRoutes(internal)
	return r
}

func TestObserverEndpointsRequireToken(t *testing.T) {
	h := NewHandler(&mockRuntimeService{})
	r := setupRuntimeTestRouter(h, "secret")

	body, _ := json.Marshal(SyncObservedPodRequest{ApplicationID: uuid.New(), Environment: "staging", PodName: "demo-0", Phase: "Running", ObservedAt: time.Now().UTC()})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/internal/runtime-pods/sync", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSyncObservedPodReturnsInvalidArgument(t *testing.T) {
	h := NewHandler(&mockRuntimeService{
		syncObservedPodFunc: func(context.Context, runtimeservice.SyncObservedPodInput) (*runtimedomain.RuntimeObservedPod, error) {
			return nil, runtimeservice.ErrNamespaceMismatch
		},
	})
	r := setupRuntimeTestRouter(h, "secret")

	body, _ := json.Marshal(SyncObservedPodRequest{ApplicationID: uuid.New(), Environment: "staging", Namespace: "wrong", PodName: "demo-0", Phase: "Running", ObservedAt: time.Now().UTC()})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/internal/runtime-pods/sync", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(ObserverTokenHeader, "secret")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteObservedPodReturnsNotFound(t *testing.T) {
	h := NewHandler(&mockRuntimeService{
		deleteObservedPodFunc: func(context.Context, runtimeservice.DeleteObservedPodInput) error {
			return runtimeservice.ErrRuntimeSpecNotFound
		},
	})
	r := setupRuntimeTestRouter(h, "secret")

	body, _ := json.Marshal(DeleteObservedPodRequest{ApplicationID: uuid.New(), Environment: "staging", Namespace: "ns", PodName: "demo-0", ObservedAt: time.Now().UTC()})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/internal/runtime-pods/delete", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(ObserverTokenHeader, "secret")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetRuntimeWorkloadReturnsIdentityNotFound(t *testing.T) {
	applicationID := uuid.New()
	h := NewHandler(&mockRuntimeService{
		getObservedWorkloadByApplicationEnvFunc: func(context.Context, uuid.UUID, string) (*runtimedomain.RuntimeObservedWorkload, error) {
			return nil, runtimeservice.ErrRuntimeIdentityMissing
		},
	})
	r := setupRuntimeTestRouter(h, "secret")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/runtime/workload?application_id="+applicationID.String()+"&environment_id=env-1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetRuntimeWorkload(t *testing.T) {
	applicationID := uuid.New()
	h := NewHandler(&mockRuntimeService{
		getObservedWorkloadByApplicationEnvFunc: func(_ context.Context, gotApplicationID uuid.UUID, environment string) (*runtimedomain.RuntimeObservedWorkload, error) {
			if gotApplicationID != applicationID {
				t.Fatalf("applicationID = %s, want %s", gotApplicationID, applicationID)
			}
			if environment != "env-1" {
				t.Fatalf("environment = %s, want env-1", environment)
			}
			return &runtimedomain.RuntimeObservedWorkload{WorkloadKind: "Deployment", WorkloadName: "meta-service"}, nil
		},
	})
	r := setupRuntimeTestRouter(h, "secret")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/runtime/workload?application_id="+applicationID.String()+"&environment_id=env-1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestListRuntimePodsReturnsIdentityNotFound(t *testing.T) {
	applicationID := uuid.New()
	h := NewHandler(&mockRuntimeService{
		listObservedPodsByApplicationEnvFunc: func(context.Context, uuid.UUID, string) ([]*runtimedomain.RuntimeObservedPod, error) {
			return nil, runtimeservice.ErrRuntimeIdentityMissing
		},
	})
	r := setupRuntimeTestRouter(h, "secret")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/runtime/pods?application_id="+applicationID.String()+"&environment_id=env-1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestListRuntimePods(t *testing.T) {
	applicationID := uuid.New()
	h := NewHandler(&mockRuntimeService{
		listObservedPodsByApplicationEnvFunc: func(_ context.Context, gotApplicationID uuid.UUID, environment string) ([]*runtimedomain.RuntimeObservedPod, error) {
			if gotApplicationID != applicationID {
				t.Fatalf("applicationID = %s, want %s", gotApplicationID, applicationID)
			}
			if environment != "env-1" {
				t.Fatalf("environment = %s, want env-1", environment)
			}
			return []*runtimedomain.RuntimeObservedPod{{PodName: "demo-0", Phase: "Running"}}, nil
		},
	})
	r := setupRuntimeTestRouter(h, "secret")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/runtime/pods?application_id="+applicationID.String()+"&environment_id=env-1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteRuntimePodReturnsAcknowledgement(t *testing.T) {
	applicationID := uuid.New()
	operationID := uuid.New()
	acceptedAt := time.Now().UTC().Truncate(time.Second)
	h := NewHandler(&mockRuntimeService{
		deletePodByApplicationEnvFunc: func(_ context.Context, gotApplicationID uuid.UUID, environment, podName, operator string) (*runtimedomain.RuntimeActionAcknowledgement, error) {
			if gotApplicationID != applicationID {
				t.Fatalf("applicationID = %s, want %s", gotApplicationID, applicationID)
			}
			if environment != "env-1" {
				t.Fatalf("environment = %s, want env-1", environment)
			}
			if podName != "demo-0" {
				t.Fatalf("podName = %s, want demo-0", podName)
			}
			if operator != "tester" {
				t.Fatalf("operator = %s, want tester", operator)
			}
			return &runtimedomain.RuntimeActionAcknowledgement{
				OperationID:      operationID,
				ApplicationID:    applicationID,
				Environment:      "env-1",
				OperationType:    runtimeservice.RuntimeOperationPodDelete,
				TargetKind:       "pod",
				TargetName:       "demo-0",
				TargetNamespace:  "devflow-staging",
				MutationState:    runtimeservice.RuntimeMutationAccepted,
				ConvergenceState: runtimeservice.RuntimeConvergencePending,
				AcceptedAt:       acceptedAt,
				Operator:         "tester",
			}, nil
		},
	})
	r := setupRuntimeTestRouter(h, "secret")

	body, _ := json.Marshal(DeleteRuntimePodRequest{ApplicationID: applicationID, EnvironmentID: "env-1", Operator: "tester"})
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/runtime/pods/demo-0", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var envelope map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	data, ok := envelope["data"].(map[string]any)
	if !ok {
		t.Fatalf("data payload missing or wrong type: %#v", envelope["data"])
	}
	if got := data["operation_id"]; got != operationID.String() {
		t.Fatalf("operation_id = %#v, want %q", got, operationID.String())
	}
	if got := data["mutation_state"]; got != runtimeservice.RuntimeMutationAccepted {
		t.Fatalf("mutation_state = %#v, want %q", got, runtimeservice.RuntimeMutationAccepted)
	}
	if got := data["convergence_state"]; got != runtimeservice.RuntimeConvergencePending {
		t.Fatalf("convergence_state = %#v, want %q", got, runtimeservice.RuntimeConvergencePending)
	}
	if got := data["accepted_at"]; got != acceptedAt.Format(time.RFC3339) {
		t.Fatalf("accepted_at = %#v, want %q", got, acceptedAt.Format(time.RFC3339))
	}
	var resp struct {
		Data runtimedomain.RuntimeActionAcknowledgement `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Data.TargetName != "demo-0" {
		t.Fatalf("target_name = %q, want demo-0", resp.Data.TargetName)
	}
	if resp.Data.ConvergenceState != runtimeservice.RuntimeConvergencePending {
		t.Fatalf("convergence_state = %q, want %q", resp.Data.ConvergenceState, runtimeservice.RuntimeConvergencePending)
	}
}

func TestDeleteRuntimePodReturnsIdentityNotFound(t *testing.T) {
	applicationID := uuid.New()
	h := NewHandler(&mockRuntimeService{
		deletePodByApplicationEnvFunc: func(_ context.Context, gotApplicationID uuid.UUID, environment, podName, operator string) (*runtimedomain.RuntimeActionAcknowledgement, error) {
			if gotApplicationID != applicationID {
				t.Fatalf("applicationID = %s, want %s", gotApplicationID, applicationID)
			}
			return nil, runtimeservice.ErrRuntimeIdentityMissing
		},
	})
	r := setupRuntimeTestRouter(h, "secret")

	body, _ := json.Marshal(DeleteRuntimePodRequest{ApplicationID: applicationID, EnvironmentID: "env-1", Operator: "tester"})
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/runtime/pods/demo-0", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteRuntimePodReturnsTargetMissingPrecondition(t *testing.T) {
	applicationID := uuid.New()
	h := NewHandler(&mockRuntimeService{
		deletePodByApplicationEnvFunc: func(_ context.Context, gotApplicationID uuid.UUID, environment, podName, operator string) (*runtimedomain.RuntimeActionAcknowledgement, error) {
			if gotApplicationID != applicationID {
				t.Fatalf("applicationID = %s, want %s", gotApplicationID, applicationID)
			}
			return nil, runtimeservice.ErrRuntimePodTargetMissing
		},
	})
	r := setupRuntimeTestRouter(h, "secret")

	body, _ := json.Marshal(DeleteRuntimePodRequest{ApplicationID: applicationID, EnvironmentID: "env-1", Operator: "tester"})
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/runtime/pods/demo-0", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("expected 412, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSyncObservedWorkloadReturnsInvalidArgument(t *testing.T) {
	h := NewHandler(&mockRuntimeService{
		syncObservedWorkloadFunc: func(context.Context, runtimeservice.SyncObservedWorkloadInput) (*runtimedomain.RuntimeObservedWorkload, error) {
			return nil, runtimeservice.ErrNamespaceMismatch
		},
	})
	r := setupRuntimeTestRouter(h, "secret")

	body, _ := json.Marshal(SyncObservedWorkloadRequest{ApplicationID: uuid.New(), Environment: "staging", Namespace: "wrong", WorkloadKind: "Deployment", WorkloadName: "demo", ObservedAt: time.Now().UTC()})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/internal/runtime-workloads/sync", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(ObserverTokenHeader, "secret")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRolloutRuntimeReturnsAcknowledgement(t *testing.T) {
	applicationID := uuid.New()
	operationID := uuid.New()
	acceptedAt := time.Now().UTC().Truncate(time.Second)
	h := NewHandler(&mockRuntimeService{
		restartDeploymentByApplicationEnvFunc: func(_ context.Context, gotApplicationID uuid.UUID, environment, deploymentName, operator string) (*runtimedomain.RuntimeActionAcknowledgement, error) {
			if gotApplicationID != applicationID {
				t.Fatalf("applicationID = %s, want %s", gotApplicationID, applicationID)
			}
			if environment != "env-1" {
				t.Fatalf("environment = %s, want env-1", environment)
			}
			if deploymentName != "demo-api" {
				t.Fatalf("deploymentName = %s, want demo-api", deploymentName)
			}
			if operator != "tester" {
				t.Fatalf("operator = %s, want tester", operator)
			}
			return &runtimedomain.RuntimeActionAcknowledgement{
				OperationID:      operationID,
				ApplicationID:    applicationID,
				Environment:      "env-1",
				OperationType:    runtimeservice.RuntimeOperationDeploymentRestart,
				TargetKind:       "deployment",
				TargetName:       "demo-api",
				TargetNamespace:  "devflow-staging",
				MutationState:    runtimeservice.RuntimeMutationAccepted,
				ConvergenceState: runtimeservice.RuntimeConvergencePending,
				ObservedWorkload: "demo-api",
				AcceptedAt:       acceptedAt,
				Operator:         "tester",
			}, nil
		},
	})
	r := setupRuntimeTestRouter(h, "secret")

	body, _ := json.Marshal(RolloutRequest{ApplicationID: applicationID, EnvironmentID: "env-1", DeploymentName: "demo-api", Operator: "tester"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/runtime/rollouts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var envelope map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	data, ok := envelope["data"].(map[string]any)
	if !ok {
		t.Fatalf("data payload missing or wrong type: %#v", envelope["data"])
	}
	if got := data["operation_id"]; got != operationID.String() {
		t.Fatalf("operation_id = %#v, want %q", got, operationID.String())
	}
	if got := data["target_kind"]; got != "deployment" {
		t.Fatalf("target_kind = %#v, want %q", got, "deployment")
	}
	if got := data["observed_workload"]; got != "demo-api" {
		t.Fatalf("observed_workload = %#v, want %q", got, "demo-api")
	}
	if got := data["convergence_state"]; got != runtimeservice.RuntimeConvergencePending {
		t.Fatalf("convergence_state = %#v, want %q", got, runtimeservice.RuntimeConvergencePending)
	}
	if got := data["accepted_at"]; got != acceptedAt.Format(time.RFC3339) {
		t.Fatalf("accepted_at = %#v, want %q", got, acceptedAt.Format(time.RFC3339))
	}
	var resp struct {
		Data runtimedomain.RuntimeActionAcknowledgement `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Data.TargetKind != "deployment" {
		t.Fatalf("target_kind = %q, want deployment", resp.Data.TargetKind)
	}
	if resp.Data.ConvergenceState != runtimeservice.RuntimeConvergencePending {
		t.Fatalf("convergence_state = %q, want %q", resp.Data.ConvergenceState, runtimeservice.RuntimeConvergencePending)
	}
}

func TestRolloutRuntimeReturnsIdentityNotFound(t *testing.T) {
	applicationID := uuid.New()
	h := NewHandler(&mockRuntimeService{
		restartDeploymentByApplicationEnvFunc: func(_ context.Context, gotApplicationID uuid.UUID, environment, deploymentName, operator string) (*runtimedomain.RuntimeActionAcknowledgement, error) {
			if gotApplicationID != applicationID {
				t.Fatalf("applicationID = %s, want %s", gotApplicationID, applicationID)
			}
			return nil, runtimeservice.ErrRuntimeIdentityMissing
		},
	})
	r := setupRuntimeTestRouter(h, "secret")

	body, _ := json.Marshal(RolloutRequest{ApplicationID: applicationID, EnvironmentID: "env-1", Operator: "tester"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/runtime/rollouts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRolloutRuntimeReturnsAmbiguousWorkloadPrecondition(t *testing.T) {
	applicationID := uuid.New()
	h := NewHandler(&mockRuntimeService{
		restartDeploymentByApplicationEnvFunc: func(_ context.Context, gotApplicationID uuid.UUID, environment, deploymentName, operator string) (*runtimedomain.RuntimeActionAcknowledgement, error) {
			if gotApplicationID != applicationID {
				t.Fatalf("applicationID = %s, want %s", gotApplicationID, applicationID)
			}
			return nil, runtimeservice.ErrRuntimeWorkloadAmbiguous
		},
	})
	r := setupRuntimeTestRouter(h, "secret")

	body, _ := json.Marshal(RolloutRequest{ApplicationID: applicationID, EnvironmentID: "env-1", Operator: "tester"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/runtime/rollouts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("expected 412, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestWriteRuntimeErrorMapsFailedPrecondition(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	writeRuntimeError(c, runtimeservice.ErrRuntimeNamespaceUnresolved)

	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("expected 412, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestWriteRuntimeErrorMapsSqlNoRowsToNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	writeRuntimeError(c, sql.ErrNoRows)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestWriteRuntimeErrorPrefersStructuredCodeOverWrappedSqlNoRows(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	coded := sharederrs.Wrap(sharederrs.CodeFailedPrecondition, "namespace unresolved", sql.ErrNoRows)
	if !errors.Is(coded, sql.ErrNoRows) {
		t.Fatal("expected wrapped sql.ErrNoRows to be discoverable")
	}

	writeRuntimeError(c, coded)

	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("expected 412, got %d: %s", rec.Code, rec.Body.String())
	}
}
