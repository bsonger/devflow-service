package http

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	runtimedomain "github.com/bsonger/devflow-service/internal/runtime/domain"
	runtimeservice "github.com/bsonger/devflow-service/internal/runtime/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type mockRuntimeService struct {
	createRuntimeSpecFunc                   func(context.Context, runtimeservice.CreateRuntimeSpecInput) (*runtimedomain.RuntimeSpec, error)
	listRuntimeSpecsFunc                    func(context.Context) ([]*runtimedomain.RuntimeSpec, error)
	getRuntimeSpecFunc                      func(context.Context, uuid.UUID) (*runtimedomain.RuntimeSpec, error)
	deleteRuntimeSpecFunc                   func(context.Context, uuid.UUID, string) error
	createRuntimeSpecRevisionFunc           func(context.Context, uuid.UUID, runtimeservice.CreateRuntimeSpecRevisionInput) (*runtimedomain.RuntimeSpecRevision, error)
	listRuntimeSpecRevisionsFunc            func(context.Context, uuid.UUID) ([]*runtimedomain.RuntimeSpecRevision, error)
	getRuntimeSpecRevisionFunc              func(context.Context, uuid.UUID) (*runtimedomain.RuntimeSpecRevision, error)
	getObservedWorkloadFunc                 func(context.Context, uuid.UUID) (*runtimedomain.RuntimeObservedWorkload, error)
	getObservedWorkloadByApplicationEnvFunc func(context.Context, uuid.UUID, string) (*runtimedomain.RuntimeObservedWorkload, error)
	syncObservedWorkloadFunc                func(context.Context, runtimeservice.SyncObservedWorkloadInput) (*runtimedomain.RuntimeObservedWorkload, error)
	deleteObservedWorkloadFunc              func(context.Context, runtimeservice.DeleteObservedWorkloadInput) error
	listObservedPodsFunc                    func(context.Context, uuid.UUID) ([]*runtimedomain.RuntimeObservedPod, error)
	listObservedPodsByApplicationEnvFunc    func(context.Context, uuid.UUID, string) ([]*runtimedomain.RuntimeObservedPod, error)
	syncObservedPodFunc                     func(context.Context, runtimeservice.SyncObservedPodInput) (*runtimedomain.RuntimeObservedPod, error)
	deleteObservedPodFunc                   func(context.Context, runtimeservice.DeleteObservedPodInput) error
	deletePodFunc                           func(context.Context, uuid.UUID, string, string) error
	deletePodByApplicationEnvFunc           func(context.Context, uuid.UUID, string, string, string) error
	restartDeploymentFunc                   func(context.Context, uuid.UUID, string, string) error
	restartDeploymentByApplicationEnvFunc   func(context.Context, uuid.UUID, string, string, string) error
	listRuntimeOperationsFunc               func(context.Context, uuid.UUID) ([]*runtimedomain.RuntimeOperation, error)
}

func (m *mockRuntimeService) CreateRuntimeSpec(ctx context.Context, in runtimeservice.CreateRuntimeSpecInput) (*runtimedomain.RuntimeSpec, error) {
	if m.createRuntimeSpecFunc != nil {
		return m.createRuntimeSpecFunc(ctx, in)
	}
	return nil, nil
}

func (m *mockRuntimeService) ListRuntimeSpecs(ctx context.Context) ([]*runtimedomain.RuntimeSpec, error) {
	if m.listRuntimeSpecsFunc != nil {
		return m.listRuntimeSpecsFunc(ctx)
	}
	return nil, nil
}

func (m *mockRuntimeService) GetRuntimeSpec(ctx context.Context, id uuid.UUID) (*runtimedomain.RuntimeSpec, error) {
	if m.getRuntimeSpecFunc != nil {
		return m.getRuntimeSpecFunc(ctx, id)
	}
	return nil, sql.ErrNoRows
}

func (m *mockRuntimeService) DeleteRuntimeSpecByApplicationEnv(ctx context.Context, applicationId uuid.UUID, environment string) error {
	if m.deleteRuntimeSpecFunc != nil {
		return m.deleteRuntimeSpecFunc(ctx, applicationId, environment)
	}
	return nil
}

func (m *mockRuntimeService) CreateRuntimeSpecRevision(ctx context.Context, runtimeSpecID uuid.UUID, in runtimeservice.CreateRuntimeSpecRevisionInput) (*runtimedomain.RuntimeSpecRevision, error) {
	if m.createRuntimeSpecRevisionFunc != nil {
		return m.createRuntimeSpecRevisionFunc(ctx, runtimeSpecID, in)
	}
	return nil, nil
}

func (m *mockRuntimeService) ListRuntimeSpecRevisions(ctx context.Context, runtimeSpecID uuid.UUID) ([]*runtimedomain.RuntimeSpecRevision, error) {
	if m.listRuntimeSpecRevisionsFunc != nil {
		return m.listRuntimeSpecRevisionsFunc(ctx, runtimeSpecID)
	}
	return nil, nil
}

func (m *mockRuntimeService) GetRuntimeSpecRevision(ctx context.Context, id uuid.UUID) (*runtimedomain.RuntimeSpecRevision, error) {
	if m.getRuntimeSpecRevisionFunc != nil {
		return m.getRuntimeSpecRevisionFunc(ctx, id)
	}
	return nil, sql.ErrNoRows
}

func (m *mockRuntimeService) GetObservedWorkload(ctx context.Context, runtimeSpecID uuid.UUID) (*runtimedomain.RuntimeObservedWorkload, error) {
	if m.getObservedWorkloadFunc != nil {
		return m.getObservedWorkloadFunc(ctx, runtimeSpecID)
	}
	return nil, sql.ErrNoRows
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

func (m *mockRuntimeService) ListObservedPods(ctx context.Context, runtimeSpecID uuid.UUID) ([]*runtimedomain.RuntimeObservedPod, error) {
	if m.listObservedPodsFunc != nil {
		return m.listObservedPodsFunc(ctx, runtimeSpecID)
	}
	return nil, nil
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

func (m *mockRuntimeService) DeletePod(ctx context.Context, runtimeSpecID uuid.UUID, podName, operator string) error {
	if m.deletePodFunc != nil {
		return m.deletePodFunc(ctx, runtimeSpecID, podName, operator)
	}
	return nil
}

func (m *mockRuntimeService) DeletePodByApplicationEnv(ctx context.Context, applicationID uuid.UUID, environment, podName, operator string) error {
	if m.deletePodByApplicationEnvFunc != nil {
		return m.deletePodByApplicationEnvFunc(ctx, applicationID, environment, podName, operator)
	}
	return nil
}

func (m *mockRuntimeService) RestartDeployment(ctx context.Context, runtimeSpecID uuid.UUID, deploymentName, operator string) error {
	if m.restartDeploymentFunc != nil {
		return m.restartDeploymentFunc(ctx, runtimeSpecID, deploymentName, operator)
	}
	return nil
}

func (m *mockRuntimeService) RestartDeploymentByApplicationEnv(ctx context.Context, applicationID uuid.UUID, environment, deploymentName, operator string) error {
	if m.restartDeploymentByApplicationEnvFunc != nil {
		return m.restartDeploymentByApplicationEnvFunc(ctx, applicationID, environment, deploymentName, operator)
	}
	return nil
}

func (m *mockRuntimeService) ListRuntimeOperations(ctx context.Context, runtimeSpecID uuid.UUID) ([]*runtimedomain.RuntimeOperation, error) {
	if m.listRuntimeOperationsFunc != nil {
		return m.listRuntimeOperationsFunc(ctx, runtimeSpecID)
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

func TestCreateRuntimeSpec(t *testing.T) {
	applicationId := uuid.New()
	h := NewHandler(&mockRuntimeService{
		createRuntimeSpecFunc: func(_ context.Context, in runtimeservice.CreateRuntimeSpecInput) (*runtimedomain.RuntimeSpec, error) {
			return &runtimedomain.RuntimeSpec{ID: uuid.New(), ApplicationID: in.ApplicationID, Environment: in.Environment}, nil
		},
	})
	r := setupRuntimeTestRouter(h, "secret")

	body, _ := json.Marshal(CreateRuntimeSpecRequest{ApplicationID: applicationId, Environment: "staging"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/runtime-specs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestListRuntimeSpecs(t *testing.T) {
	h := NewHandler(&mockRuntimeService{
		listRuntimeSpecsFunc: func(context.Context) ([]*runtimedomain.RuntimeSpec, error) {
			return []*runtimedomain.RuntimeSpec{{ID: uuid.New(), ApplicationID: uuid.New(), Environment: "staging"}}, nil
		},
	})
	r := setupRuntimeTestRouter(h, "secret")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/runtime-specs", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetRuntimeSpec(t *testing.T) {
	id := uuid.New()
	h := NewHandler(&mockRuntimeService{
		getRuntimeSpecFunc: func(context.Context, uuid.UUID) (*runtimedomain.RuntimeSpec, error) {
			return &runtimedomain.RuntimeSpec{ID: id, ApplicationID: uuid.New(), Environment: "staging"}, nil
		},
	})
	r := setupRuntimeTestRouter(h, "secret")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/runtime-specs/"+id.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteRuntimeSpec(t *testing.T) {
	applicationID := uuid.New()
	h := NewHandler(&mockRuntimeService{
		deleteRuntimeSpecFunc: func(_ context.Context, gotApplicationID uuid.UUID, environment string) error {
			if gotApplicationID != applicationID {
				t.Fatalf("applicationID = %s, want %s", gotApplicationID, applicationID)
			}
			if environment != "staging" {
				t.Fatalf("environment = %s, want staging", environment)
			}
			return nil
		},
	})
	r := setupRuntimeTestRouter(h, "secret")

	body, _ := json.Marshal(DeleteRuntimeSpecRequest{ApplicationID: applicationID, Environment: "staging"})
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/runtime-specs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateRuntimeSpecRevision(t *testing.T) {
	runtimeSpecID := uuid.New()
	h := NewHandler(&mockRuntimeService{
		createRuntimeSpecRevisionFunc: func(_ context.Context, gotRuntimeSpecID uuid.UUID, in runtimeservice.CreateRuntimeSpecRevisionInput) (*runtimedomain.RuntimeSpecRevision, error) {
			return &runtimedomain.RuntimeSpecRevision{ID: uuid.New(), RuntimeSpecID: gotRuntimeSpecID, Revision: 1, CreatedBy: in.CreatedBy}, nil
		},
	})
	r := setupRuntimeTestRouter(h, "secret")

	body, _ := json.Marshal(CreateRuntimeSpecRevisionRequest{Replicas: 1, CreatedBy: "tester"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/runtime-specs/"+runtimeSpecID.String()+"/revisions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestListRuntimeSpecRevisions(t *testing.T) {
	runtimeSpecID := uuid.New()
	h := NewHandler(&mockRuntimeService{
		listRuntimeSpecRevisionsFunc: func(context.Context, uuid.UUID) ([]*runtimedomain.RuntimeSpecRevision, error) {
			return []*runtimedomain.RuntimeSpecRevision{{ID: uuid.New(), RuntimeSpecID: runtimeSpecID, Revision: 2}}, nil
		},
	})
	r := setupRuntimeTestRouter(h, "secret")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/runtime-specs/"+runtimeSpecID.String()+"/revisions", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetRuntimeSpecRevision(t *testing.T) {
	revisionID := uuid.New()
	h := NewHandler(&mockRuntimeService{
		getRuntimeSpecRevisionFunc: func(context.Context, uuid.UUID) (*runtimedomain.RuntimeSpecRevision, error) {
			return &runtimedomain.RuntimeSpecRevision{ID: revisionID, RuntimeSpecID: uuid.New(), Revision: 1}, nil
		},
	})
	r := setupRuntimeTestRouter(h, "secret")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/runtime-spec-revisions/"+revisionID.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
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

func TestDeleteRuntimePod(t *testing.T) {
	applicationID := uuid.New()
	h := NewHandler(&mockRuntimeService{
		deletePodByApplicationEnvFunc: func(_ context.Context, gotApplicationID uuid.UUID, environment, podName, operator string) error {
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
			return nil
		},
	})
	r := setupRuntimeTestRouter(h, "secret")

	body, _ := json.Marshal(DeleteRuntimePodRequest{ApplicationID: applicationID, EnvironmentID: "env-1", Operator: "tester"})
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/runtime/pods/demo-0", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
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

func TestRolloutRuntime(t *testing.T) {
	applicationID := uuid.New()
	h := NewHandler(&mockRuntimeService{
		restartDeploymentByApplicationEnvFunc: func(_ context.Context, gotApplicationID uuid.UUID, environment, deploymentName, operator string) error {
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
			return nil
		},
	})
	r := setupRuntimeTestRouter(h, "secret")

	body, _ := json.Marshal(RolloutRequest{ApplicationID: applicationID, EnvironmentID: "env-1", DeploymentName: "demo-api", Operator: "tester"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/runtime/rollouts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRolloutRuntimeAllowsServerResolvedDeploymentName(t *testing.T) {
	applicationID := uuid.New()
	h := NewHandler(&mockRuntimeService{
		restartDeploymentByApplicationEnvFunc: func(_ context.Context, gotApplicationID uuid.UUID, environment, deploymentName, operator string) error {
			if gotApplicationID != applicationID {
				t.Fatalf("applicationID = %s, want %s", gotApplicationID, applicationID)
			}
			if environment != "env-1" {
				t.Fatalf("environment = %s, want env-1", environment)
			}
			if deploymentName != "" {
				t.Fatalf("deploymentName = %s, want empty", deploymentName)
			}
			if operator != "tester" {
				t.Fatalf("operator = %s, want tester", operator)
			}
			return nil
		},
	})
	r := setupRuntimeTestRouter(h, "secret")

	body, _ := json.Marshal(RolloutRequest{ApplicationID: applicationID, EnvironmentID: "env-1", Operator: "tester"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/runtime/rollouts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDeletePod(t *testing.T) {
	runtimeSpecID := uuid.New()
	h := NewHandler(&mockRuntimeService{
		deletePodFunc: func(_ context.Context, gotRuntimeSpecID uuid.UUID, podName, operator string) error {
			if gotRuntimeSpecID != runtimeSpecID {
				t.Fatalf("runtimeSpecID = %s, want %s", gotRuntimeSpecID, runtimeSpecID)
			}
			if podName != "demo-0" {
				t.Fatalf("podName = %s, want demo-0", podName)
			}
			if operator != "tester" {
				t.Fatalf("operator = %s, want tester", operator)
			}
			return nil
		},
	})
	r := setupRuntimeTestRouter(h, "secret")

	body, _ := json.Marshal(map[string]string{"operator": "tester"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/runtime-specs/"+runtimeSpecID.String()+"/pods/demo-0/delete", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRestartDeployment(t *testing.T) {
	runtimeSpecID := uuid.New()
	h := NewHandler(&mockRuntimeService{
		restartDeploymentFunc: func(_ context.Context, gotRuntimeSpecID uuid.UUID, deploymentName, operator string) error {
			if gotRuntimeSpecID != runtimeSpecID {
				t.Fatalf("runtimeSpecID = %s, want %s", gotRuntimeSpecID, runtimeSpecID)
			}
			if deploymentName != "myapp" {
				t.Fatalf("deploymentName = %s, want myapp", deploymentName)
			}
			if operator != "tester" {
				t.Fatalf("operator = %s, want tester", operator)
			}
			return nil
		},
	})
	r := setupRuntimeTestRouter(h, "secret")

	body, _ := json.Marshal(map[string]string{"operator": "tester"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/runtime-specs/"+runtimeSpecID.String()+"/deployments/myapp/restart", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestListRuntimeOperations(t *testing.T) {
	runtimeSpecID := uuid.New()
	h := NewHandler(&mockRuntimeService{
		listRuntimeOperationsFunc: func(_ context.Context, gotRuntimeSpecID uuid.UUID) ([]*runtimedomain.RuntimeOperation, error) {
			if gotRuntimeSpecID != runtimeSpecID {
				t.Fatalf("runtimeSpecID = %s, want %s", gotRuntimeSpecID, runtimeSpecID)
			}
			return []*runtimedomain.RuntimeOperation{{ID: uuid.New(), RuntimeSpecID: runtimeSpecID, OperationType: "pod_delete", TargetName: "demo-0"}}, nil
		},
	})
	r := setupRuntimeTestRouter(h, "secret")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/runtime-specs/"+runtimeSpecID.String()+"/operations", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}
