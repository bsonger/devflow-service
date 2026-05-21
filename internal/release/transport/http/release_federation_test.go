package http

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestReleaseShouldNotProxyByManifestForCreatePath(t *testing.T) {
	originalGetLocalManifest := releaseGetLocalManifest
	originalProxyBaseURL := releaseCurrentProxyBaseURL
	defer func() {
		releaseGetLocalManifest = originalGetLocalManifest
		releaseCurrentProxyBaseURL = originalProxyBaseURL
	}()

	manifestID := uuid.New()
	releaseCurrentProxyBaseURL = func() string {
		return "http://release-service.devflow.svc.cluster.local"
	}
	releaseGetLocalManifest = func(_ context.Context, id uuid.UUID) (*manifestdomain.Manifest, error) {
		if id != manifestID {
			t.Fatalf("manifest id = %s want %s", id, manifestID)
		}
		return nil, nil
	}

	if releaseShouldProxyByManifest(context.Background(), manifestID, "env-prod") {
		t.Fatal("expected create path to stay on the current release-service")
	}
}

func TestReleaseShouldNotProxyByTargetForListPath(t *testing.T) {
	originalProxyBaseURL := releaseCurrentProxyBaseURL
	defer func() {
		releaseCurrentProxyBaseURL = originalProxyBaseURL
	}()

	releaseCurrentProxyBaseURL = func() string {
		return "http://release-service.devflow.svc.cluster.local"
	}

	if releaseShouldProxyByTarget(context.Background(), uuid.New(), "env-pre") {
		t.Fatal("expected list path to stay on the current release-service")
	}
}

func TestGetReleaseProxiesLocalNotFound(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	originalHTTPClient := releaseFederationHTTPClient
	originalProxyBaseURL := releaseCurrentProxyBaseURL
	defer func() {
		releaseFederationHTTPClient = originalHTTPClient
		releaseCurrentProxyBaseURL = originalProxyBaseURL
	}()

	releaseID := uuid.New()
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/releases/"+releaseID.String() {
			t.Fatalf("unexpected proxy request %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"id":"` + releaseID.String() + `","status":"Pending"}}`))
	}))
	defer remote.Close()

	releaseFederationHTTPClient = remote.Client()
	releaseCurrentProxyBaseURL = func() string {
		return remote.URL
	}

	handler := &ReleaseHandler{
		svc: stubReleaseService{
			getFn: func(context.Context, uuid.UUID) (*model.Release, error) {
				return nil, sql.ErrNoRows
			},
		},
	}
	router := gin.New()
	router.GET("/api/v1/releases/:id", handler.Get)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/releases/"+releaseID.String(), nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}
