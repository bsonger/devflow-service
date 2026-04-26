package http

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	appconfigdomain "github.com/bsonger/devflow-service/internal/appconfig/domain"
	"github.com/bsonger/devflow-service/internal/applicationenv/domain"
	applicationenvservice "github.com/bsonger/devflow-service/internal/applicationenv/service"
	envdomain "github.com/bsonger/devflow-service/internal/environment/domain"
	workloadconfigdomain "github.com/bsonger/devflow-service/internal/workloadconfig/domain"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type stubBindingService struct {
	attachFn    func(context.Context, uuid.UUID, domain.BindingInput) (*domain.Binding, error)
	getFn       func(context.Context, uuid.UUID, string) (*domain.Binding, error)
	listFn      func(context.Context, uuid.UUID) ([]applicationenvservice.BindingView, error)
	deleteFn    func(context.Context, uuid.UUID, string) error
	getDetailFn func(context.Context, uuid.UUID, string) (*applicationenvservice.BindingDetail, error)
}

func (s stubBindingService) Attach(ctx context.Context, applicationID uuid.UUID, input domain.BindingInput) (*domain.Binding, error) {
	return s.attachFn(ctx, applicationID, input)
}
func (s stubBindingService) Get(ctx context.Context, applicationID uuid.UUID, environmentID string) (*domain.Binding, error) {
	return s.getFn(ctx, applicationID, environmentID)
}
func (s stubBindingService) List(ctx context.Context, applicationID uuid.UUID) ([]applicationenvservice.BindingView, error) {
	return s.listFn(ctx, applicationID)
}
func (s stubBindingService) Delete(ctx context.Context, applicationID uuid.UUID, environmentID string) error {
	return s.deleteFn(ctx, applicationID, environmentID)
}
func (s stubBindingService) GetDetail(ctx context.Context, applicationID uuid.UUID, environmentID string) (*applicationenvservice.BindingDetail, error) {
	return s.getDetailFn(ctx, applicationID, environmentID)
}

func TestAttachReturnsEnvelope(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := NewHandler(stubBindingService{
		attachFn: func(_ context.Context, applicationID uuid.UUID, input domain.BindingInput) (*domain.Binding, error) {
			item := &domain.Binding{ApplicationID: applicationID, EnvironmentID: input.EnvironmentID}
			item.WithCreateDefault()
			return item, nil
		},
	})

	r := gin.New()
	r.POST("/api/v1/applications/:id/environments", handler.Attach)

	body := bytes.NewBufferString(`{"environment_id":"11111111-1111-1111-1111-111111111111"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/applications/"+uuid.New().String()+"/environments", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("got %d want %d body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var payload struct {
		Data domain.Binding `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if payload.Data.EnvironmentID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("unexpected payload: %#v", payload.Data)
	}
}

func TestListReturnsEnvelope(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := NewHandler(stubBindingService{
		listFn: func(_ context.Context, _ uuid.UUID) ([]applicationenvservice.BindingView, error) {
			item := domain.Binding{ApplicationID: uuid.New(), EnvironmentID: uuid.NewString()}
			item.WithCreateDefault()
			return []applicationenvservice.BindingView{{
				Binding: item,
				Environment: &envdomain.Environment{
					Name: "staging",
				},
			}}, nil
		},
	})

	r := gin.New()
	r.GET("/api/v1/applications/:id/environments", handler.List)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/applications/"+uuid.New().String()+"/environments?page=1&page_size=20", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestGetReturnsDetailEnvelope(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := NewHandler(stubBindingService{
		getDetailFn: func(_ context.Context, applicationID uuid.UUID, environmentID string) (*applicationenvservice.BindingDetail, error) {
			item := domain.Binding{ApplicationID: applicationID, EnvironmentID: environmentID}
			item.WithCreateDefault()
			return &applicationenvservice.BindingDetail{
				BindingView: applicationenvservice.BindingView{
					Binding: item,
					Environment: &envdomain.Environment{
						Name: "staging",
					},
				},
				AppConfigs: []appconfigdomain.AppConfig{{Name: "base-config", EnvironmentID: "base"}},
				WorkloadConfigs: []workloadconfigdomain.WorkloadConfig{{
					Name: "web",
				}},
			}, nil
		},
	})

	r := gin.New()
	r.GET("/api/v1/applications/:id/environments/:environment_id", handler.Get)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/applications/"+uuid.New().String()+"/environments/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestDeleteNotFoundReturnsEnvelope(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := NewHandler(stubBindingService{
		deleteFn: func(_ context.Context, _ uuid.UUID, _ string) error {
			return sql.ErrNoRows
		},
	})

	r := gin.New()
	r.DELETE("/api/v1/applications/:id/environments/:environment_id", handler.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/applications/"+uuid.New().String()+"/environments/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("got %d want %d", rec.Code, http.StatusNotFound)
	}
}
