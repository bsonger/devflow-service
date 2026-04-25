package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	intentdomain "github.com/bsonger/devflow-service/internal/intent/domain"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	intentservice "github.com/bsonger/devflow-service/internal/intent/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type stubIntentService struct {
	getFn  func(context.Context, uuid.UUID) (*intentdomain.Intent, error)
	listFn func(context.Context, intentservice.IntentListFilter) ([]*intentdomain.Intent, error)
}

func (s stubIntentService) Get(ctx context.Context, id uuid.UUID) (*intentdomain.Intent, error) {
	return s.getFn(ctx, id)
}

func (s stubIntentService) List(ctx context.Context, filter intentservice.IntentListFilter) ([]*intentdomain.Intent, error) {
	return s.listFn(ctx, filter)
}

func TestBuildIntentFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	resourceID := uuid.New()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet,
		"/api/v1/intents?kind=build&status=Pending&resource_id="+resourceID.String()+"&claimed_by=worker-1",
		nil,
	)

	filter, err := buildIntentFilter(ctx)
	if err != nil {
		t.Fatalf("buildIntentFilter returned error: %v", err)
	}

	if got := filter.Kind; got != string(model.IntentKindBuild) {
		t.Fatalf("unexpected kind: got %#v want %#v", got, model.IntentKindBuild)
	}
	if got := filter.Status; got != string(model.IntentPending) {
		t.Fatalf("unexpected status: got %#v want %#v", got, model.IntentPending)
	}
	if filter.ResourceID == nil || *filter.ResourceID != resourceID {
		t.Fatalf("unexpected resource_id: got %#v want %#v", filter.ResourceID, resourceID)
	}
	if got := filter.ClaimedBy; got != "worker-1" {
		t.Fatalf("unexpected claimed_by: got %#v want %q", got, "worker-1")
	}
}

func TestBuildIntentFilterInvalidObjectID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/intents?resource_id=invalid-id", nil)

	_, err := buildIntentFilter(ctx)
	if err == nil {
		t.Fatal("expected error but got nil")
	}
	if err.Error() != "invalid resource_id" {
		t.Fatalf("unexpected error: got %q want %q", err.Error(), "invalid resource_id")
	}
}

func TestListIntentReturnsEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &IntentHandler{
		svc: stubIntentService{
			listFn: func(_ context.Context, filter intentservice.IntentListFilter) ([]*intentdomain.Intent, error) {
				if filter.Kind != string(model.IntentKindBuild) {
					t.Fatalf("unexpected filter: %#v", filter)
				}
				return []*intentdomain.Intent{{Kind: model.IntentKindBuild, Status: model.IntentPending, ResourceType: "manifest", ResourceID: uuid.New()}}, nil
			},
		},
	}

	r := gin.New()
	r.GET("/api/v1/intents", handler.List)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/intents?kind=build&page=1&page_size=20", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d want %d", rec.Code, http.StatusOK)
	}

	var payload struct {
		Data       []intentdomain.Intent `json:"data"`
		Pagination struct {
			Page     int `json:"page"`
			PageSize int `json:"page_size"`
			Total    int `json:"total"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if len(payload.Data) != 1 || payload.Pagination.Total != 1 {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}
