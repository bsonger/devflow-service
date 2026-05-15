package http

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sharederrs "github.com/bsonger/devflow-service/internal/shared/errs"
	"github.com/bsonger/devflow-service/internal/workloadconfig/domain"
	workloadconfig "github.com/bsonger/devflow-service/internal/workloadconfig/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type mockWorkloadConfigService struct {
	createFunc func(ctx context.Context, item *domain.WorkloadConfig) (uuid.UUID, error)
	getFunc    func(ctx context.Context, id uuid.UUID) (*domain.WorkloadConfig, error)
	updateFunc func(ctx context.Context, item *domain.WorkloadConfig) error
	deleteFunc func(ctx context.Context, id uuid.UUID) error
	listFunc   func(ctx context.Context, filter workloadconfig.WorkloadConfigListFilter) ([]domain.WorkloadConfig, error)
}

func (m *mockWorkloadConfigService) Create(ctx context.Context, item *domain.WorkloadConfig) (uuid.UUID, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, item)
	}
	return uuid.New(), nil
}

func (m *mockWorkloadConfigService) Get(ctx context.Context, id uuid.UUID) (*domain.WorkloadConfig, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, id)
	}
	return nil, sql.ErrNoRows
}

func (m *mockWorkloadConfigService) Update(ctx context.Context, item *domain.WorkloadConfig) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, item)
	}
	return nil
}

func (m *mockWorkloadConfigService) Delete(ctx context.Context, id uuid.UUID) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, id)
	}
	return nil
}

func (m *mockWorkloadConfigService) List(ctx context.Context, filter workloadconfig.WorkloadConfigListFilter) ([]domain.WorkloadConfig, error) {
	if m.listFunc != nil {
		return m.listFunc(ctx, filter)
	}
	return nil, nil
}

func setupTestRouter(h *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api/v1")
	h.RegisterRoutes(api)
	return r
}

type apiErrorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func TestCreateWorkloadConfig(t *testing.T) {
	applicationID := uuid.New()
	var created *domain.WorkloadConfig
	wlSvc := &mockWorkloadConfigService{
		createFunc: func(ctx context.Context, item *domain.WorkloadConfig) (uuid.UUID, error) {
			created = item
			return uuid.New(), nil
		},
	}
	h := NewHandler(wlSvc)
	r := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workload-configs", bytes.NewReader(mustJSON(t, domain.WorkloadConfigInput{
		ApplicationID:      applicationID,
		Replicas:           1,
		ServiceAccountName: "runner",
		Resources: domain.WorkloadResourceRequirements{
			SizeClass: domain.WorkloadSizeClassMedium,
		},
		Probes: domain.WorkloadProbes{
			Liveness: &domain.WorkloadProbe{Path: "/healthz", Port: "http", PeriodSeconds: 10},
		},
		Metrics: domain.WorkloadMetrics{
			Enabled: true,
			Port:    9090,
		},
		EmptyDirs: []domain.WorkloadEmptyDir{{
			Name:      "tmp",
			MountPath: "/tmp",
		}},
		Env:         []domain.EnvVar{{Name: "LOG_LEVEL", Value: "info"}},
		Labels:      map[string]string{"team": "platform"},
		Annotations: map[string]string{"sidecar.istio.io/inject": "true"},
	})))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if created == nil {
		t.Fatal("expected create to receive workload config")
	}
	if created.ApplicationID != applicationID {
		t.Fatalf("application_id = %s, want %s", created.ApplicationID, applicationID)
	}
	if created.Resources.SizeClass != domain.WorkloadSizeClassMedium {
		t.Fatalf("size_class = %q, want %q", created.Resources.SizeClass, domain.WorkloadSizeClassMedium)
	}
	if !created.Metrics.Enabled || created.Metrics.Port != 9090 {
		t.Fatalf("unexpected metrics payload: %#v", created.Metrics)
	}
	if len(created.EmptyDirs) != 1 || created.EmptyDirs[0].MountPath != "/tmp" {
		t.Fatalf("unexpected empty_dirs payload: %#v", created.EmptyDirs)
	}
	if len(created.Env) != 1 || created.Env[0].Name != "LOG_LEVEL" || created.Env[0].Value != "info" {
		t.Fatalf("unexpected env payload: %#v", created.Env)
	}
}

func TestCreateWorkloadConfigMapsWriteErrors(t *testing.T) {
	applicationID := uuid.New()
	tests := []struct {
		name       string
		body       string
		serviceErr error
		wantStatus int
		wantCode   string
		wantBody   string
	}{
		{
			name:       "malformed body",
			body:       `{"application_id":`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_argument",
			wantBody:   "invalid request body",
		},
		{
			name: "duplicate env names remain invalid_argument",
			body: string(mustJSON(t, domain.WorkloadConfigInput{
				ApplicationID: applicationID,
				Replicas:      1,
				Resources:     domain.WorkloadResourceRequirements{SizeClass: domain.WorkloadSizeClassSmall},
				Env:           []domain.EnvVar{{Name: "LOG_LEVEL", Value: "info"}, {Name: "LOG_LEVEL", Value: "debug"}},
			})),
			serviceErr: sharederrs.InvalidArgument(`env[1].name duplicates env[0].name "LOG_LEVEL"`),
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_argument",
			wantBody:   `env[1].name duplicates env[0].name "LOG_LEVEL"`,
		},
		{
			name: "bad probe path remains invalid_argument",
			body: string(mustJSON(t, domain.WorkloadConfigInput{
				ApplicationID: applicationID,
				Replicas:      1,
				Resources:     domain.WorkloadResourceRequirements{SizeClass: domain.WorkloadSizeClassSmall},
				Probes: domain.WorkloadProbes{
					Readiness: &domain.WorkloadProbe{Path: "readyz", Port: "http"},
				},
			})),
			serviceErr: sharederrs.InvalidArgument("probes.readiness.path must start with '/'"),
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_argument",
			wantBody:   "probes.readiness.path must start with '/'",
		},
		{
			name: "invalid size class remains invalid_argument",
			body: string(mustJSON(t, map[string]any{
				"application_id": applicationID.String(),
				"replicas":       1,
				"resources": map[string]any{
					"size_class": "jumbo",
				},
			})),
			serviceErr: sharederrs.InvalidArgument("resources.size_class must be one of: large, medium, small, xlarge"),
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_argument",
			wantBody:   "resources.size_class must be one of:",
		},
		{
			name: "invalid metrics profile remains invalid_argument",
			body: string(mustJSON(t, domain.WorkloadConfigInput{
				ApplicationID: applicationID,
				Replicas:      1,
				Resources:     domain.WorkloadResourceRequirements{SizeClass: domain.WorkloadSizeClassSmall},
				Metrics: domain.WorkloadMetrics{
					Enabled:       true,
					Port:          9090,
					ScrapeProfile: "burst",
				},
			})),
			serviceErr: sharederrs.InvalidArgument("metrics.scrape_profile must be one of: default, fast, slow"),
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_argument",
			wantBody:   "metrics.scrape_profile must be one of:",
		},
		{
			name: "legacy wide resources become failed_precondition",
			body: string(mustJSON(t, map[string]any{
				"application_id": applicationID.String(),
				"replicas":       1,
				"resources": map[string]any{
					"size_class": "small",
					"requests":   map[string]any{"cpu": "100m", "memory": "128Mi"},
					"limits":     map[string]any{"cpu": "500m", "memory": "512Mi"},
				},
			})),
			serviceErr: sharederrs.InvalidArgument("resources.requests must not be provided on write; resources.limits must not be provided on write"),
			wantStatus: http.StatusPreconditionFailed,
			wantCode:   "failed_precondition",
			wantBody:   "resources.requests must not be provided on write",
		},
		{
			name: "service conflict preserved",
			body: string(mustJSON(t, domain.WorkloadConfigInput{
				ApplicationID: applicationID,
				Replicas:      1,
				Resources:     domain.WorkloadResourceRequirements{SizeClass: domain.WorkloadSizeClassSmall},
			})),
			serviceErr: sharederrs.Conflict("workload config already exists for application"),
			wantStatus: http.StatusConflict,
			wantCode:   "conflict",
			wantBody:   "workload config already exists for application",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wlSvc := &mockWorkloadConfigService{
				createFunc: func(ctx context.Context, item *domain.WorkloadConfig) (uuid.UUID, error) {
					if tc.serviceErr == nil {
						t.Fatal("service should not be called for malformed body")
					}
					return uuid.Nil, tc.serviceErr
				},
			}
			h := NewHandler(wlSvc)
			r := setupTestRouter(h)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/workload-configs", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			assertAPIError(t, rec, tc.wantCode, tc.wantBody)
		})
	}
}

func TestGetWorkloadConfig(t *testing.T) {
	id := uuid.New()
	wlSvc := &mockWorkloadConfigService{
		getFunc: func(ctx context.Context, uid uuid.UUID) (*domain.WorkloadConfig, error) {
			return &domain.WorkloadConfig{BaseModel: domain.BaseModel{ID: uid}, Replicas: 1}, nil
		},
	}
	h := NewHandler(wlSvc)
	r := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workload-configs/"+id.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetWorkloadConfigNotFound(t *testing.T) {
	id := uuid.New()
	wlSvc := &mockWorkloadConfigService{
		getFunc: func(ctx context.Context, uid uuid.UUID) (*domain.WorkloadConfig, error) {
			return nil, sql.ErrNoRows
		},
	}
	h := NewHandler(wlSvc)
	r := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workload-configs/"+id.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestListWorkloadConfigs(t *testing.T) {
	wlSvc := &mockWorkloadConfigService{
		listFunc: func(ctx context.Context, filter workloadconfig.WorkloadConfigListFilter) ([]domain.WorkloadConfig, error) {
			return []domain.WorkloadConfig{{BaseModel: domain.BaseModel{ID: uuid.New()}, Replicas: 1}}, nil
		},
	}
	h := NewHandler(wlSvc)
	r := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workload-configs", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateWorkloadConfig(t *testing.T) {
	id := uuid.New()
	applicationID := uuid.New()
	var updated *domain.WorkloadConfig
	wlSvc := &mockWorkloadConfigService{
		updateFunc: func(ctx context.Context, item *domain.WorkloadConfig) error {
			updated = item
			return nil
		},
	}
	h := NewHandler(wlSvc)
	r := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/workload-configs/"+id.String(), bytes.NewReader(mustJSON(t, domain.WorkloadConfigInput{
		ApplicationID: applicationID,
		Replicas:      2,
		Resources: domain.WorkloadResourceRequirements{
			SizeClass: domain.WorkloadSizeClassSmall,
		},
		Env: []domain.EnvVar{{Name: "LOG_LEVEL", Value: "info"}},
	})))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
	if updated == nil {
		t.Fatal("expected update to receive workload config")
	}
	if updated.ID != id {
		t.Fatalf("id = %s, want %s", updated.ID, id)
	}
	if updated.ApplicationID != applicationID {
		t.Fatalf("application_id = %s, want %s", updated.ApplicationID, applicationID)
	}
}

func TestUpdateWorkloadConfigMapsWriteErrors(t *testing.T) {
	id := uuid.New()
	applicationID := uuid.New()
	tests := []struct {
		name       string
		body       string
		serviceErr error
		wantStatus int
		wantCode   string
		wantBody   string
	}{
		{
			name: "duplicate env names remain invalid_argument",
			body: string(mustJSON(t, domain.WorkloadConfigInput{
				ApplicationID: applicationID,
				Replicas:      2,
				Resources:     domain.WorkloadResourceRequirements{SizeClass: domain.WorkloadSizeClassSmall},
				Env:           []domain.EnvVar{{Name: "LOG_LEVEL", Value: "info"}, {Name: "LOG_LEVEL", Value: "debug"}},
			})),
			serviceErr: sharederrs.InvalidArgument(`env[1].name duplicates env[0].name "LOG_LEVEL"`),
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_argument",
			wantBody:   `env[1].name duplicates env[0].name "LOG_LEVEL"`,
		},
		{
			name: "bad probe path remains invalid_argument",
			body: string(mustJSON(t, domain.WorkloadConfigInput{
				ApplicationID: applicationID,
				Replicas:      2,
				Resources:     domain.WorkloadResourceRequirements{SizeClass: domain.WorkloadSizeClassSmall},
				Probes: domain.WorkloadProbes{
					Startup: &domain.WorkloadProbe{Path: "startupz", Port: "http"},
				},
			})),
			serviceErr: sharederrs.InvalidArgument("probes.startup.path must start with '/'"),
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_argument",
			wantBody:   "probes.startup.path must start with '/'",
		},
		{
			name: "invalid size class remains invalid_argument",
			body: string(mustJSON(t, map[string]any{
				"application_id": applicationID.String(),
				"replicas":       2,
				"resources": map[string]any{
					"size_class": "jumbo",
				},
			})),
			serviceErr: sharederrs.InvalidArgument("resources.size_class must be one of: large, medium, small, xlarge"),
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_argument",
			wantBody:   "resources.size_class must be one of:",
		},
		{
			name: "legacy wide probes and resources become failed_precondition",
			body: string(mustJSON(t, map[string]any{
				"application_id": applicationID.String(),
				"replicas":       2,
				"resources": map[string]any{
					"size_class": "small",
					"requests":   map[string]any{"cpu": "100m", "memory": "128Mi"},
				},
				"probes": map[string]any{
					"readiness": map[string]any{"path": "/readyz", "port": "http"},
					"legacy":    map[string]any{"http_get": map[string]any{"path": "/readyz"}},
				},
			})),
			serviceErr: sharederrs.InvalidArgument("resources.requests must not be provided on write"),
			wantStatus: http.StatusPreconditionFailed,
			wantCode:   "failed_precondition",
			wantBody:   "resources.requests must not be provided on write",
		},
		{
			name: "not found preserved",
			body: string(mustJSON(t, domain.WorkloadConfigInput{
				ApplicationID: applicationID,
				Replicas:      2,
				Resources:     domain.WorkloadResourceRequirements{SizeClass: domain.WorkloadSizeClassSmall},
			})),
			serviceErr: sql.ErrNoRows,
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
			wantBody:   "not found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wlSvc := &mockWorkloadConfigService{
				updateFunc: func(ctx context.Context, item *domain.WorkloadConfig) error {
					return tc.serviceErr
				},
			}
			h := NewHandler(wlSvc)
			r := setupTestRouter(h)

			req := httptest.NewRequest(http.MethodPut, "/api/v1/workload-configs/"+id.String(), strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			assertAPIError(t, rec, tc.wantCode, tc.wantBody)
		})
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return data
}

func assertAPIError(t *testing.T, rec *httptest.ResponseRecorder, wantCode, wantMessage string) {
	t.Helper()
	var resp apiErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error body: %v", err)
	}
	if resp.Error.Code != wantCode {
		t.Fatalf("error code = %q, want %q", resp.Error.Code, wantCode)
	}
	if !strings.Contains(resp.Error.Message, wantMessage) {
		t.Fatalf("error message = %q, want substring %q", resp.Error.Message, wantMessage)
	}
}
