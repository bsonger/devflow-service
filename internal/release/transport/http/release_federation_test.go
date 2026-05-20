package http

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/bsonger/devflow-service/internal/release/support"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestReleaseShouldProxyByManifestForRemoteProductionTarget(t *testing.T) {
	originalGetLocalManifest := releaseGetLocalManifest
	originalResolve := releaseResolveDeployTarget
	originalRuntimeConfig := releaseCurrentRuntimeConfig
	defer func() {
		releaseGetLocalManifest = originalGetLocalManifest
		releaseResolveDeployTarget = originalResolve
		releaseCurrentRuntimeConfig = originalRuntimeConfig
	}()

	appID := uuid.New()
	manifestID := uuid.New()
	releaseCurrentRuntimeConfig = func() support.RuntimeConfig {
		return support.RuntimeConfig{
			ControlPlaneID: "devflow-staging",
			Downstream: model.DownstreamConfig{
				ReleaseServiceBaseURL: "http://release-service.devflow.svc.cluster.local",
			},
		}
	}
	releaseGetLocalManifest = func(_ context.Context, id uuid.UUID) (*manifestdomain.Manifest, error) {
		if id != manifestID {
			t.Fatalf("manifest id = %s want %s", id, manifestID)
		}
		return &manifestdomain.Manifest{ApplicationID: appID}, nil
	}
	releaseResolveDeployTarget = func(_ context.Context, applicationID, environmentID string) (*support.DeployTarget, error) {
		if applicationID != appID.String() || environmentID != "env-prod" {
			t.Fatalf("unexpected target lookup application=%q environment=%q", applicationID, environmentID)
		}
		return &support.DeployTarget{EnvironmentName: "production"}, nil
	}

	if !releaseShouldProxyByManifest(context.Background(), manifestID, "env-prod") {
		t.Fatal("expected release request to proxy to production control plane")
	}
}

func TestReleaseShouldNotProxyByManifestForLocalStagingTarget(t *testing.T) {
	originalGetLocalManifest := releaseGetLocalManifest
	originalResolve := releaseResolveDeployTarget
	originalRuntimeConfig := releaseCurrentRuntimeConfig
	defer func() {
		releaseGetLocalManifest = originalGetLocalManifest
		releaseResolveDeployTarget = originalResolve
		releaseCurrentRuntimeConfig = originalRuntimeConfig
	}()

	appID := uuid.New()
	manifestID := uuid.New()
	releaseCurrentRuntimeConfig = func() support.RuntimeConfig {
		return support.RuntimeConfig{
			ControlPlaneID: "devflow-staging",
			Downstream: model.DownstreamConfig{
				ReleaseServiceBaseURL: "http://release-service.devflow.svc.cluster.local",
			},
		}
	}
	releaseGetLocalManifest = func(context.Context, uuid.UUID) (*manifestdomain.Manifest, error) {
		return &manifestdomain.Manifest{ApplicationID: appID}, nil
	}
	releaseResolveDeployTarget = func(context.Context, string, string) (*support.DeployTarget, error) {
		return &support.DeployTarget{EnvironmentName: "staging"}, nil
	}

	if releaseShouldProxyByManifest(context.Background(), manifestID, "env-pre") {
		t.Fatal("expected local staging target to stay local")
	}
}

func TestGetReleaseProxiesLocalNotFound(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	originalHTTPClient := releaseFederationHTTPClient
	originalRuntimeConfig := releaseCurrentRuntimeConfig
	defer func() {
		releaseFederationHTTPClient = originalHTTPClient
		releaseCurrentRuntimeConfig = originalRuntimeConfig
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
	releaseCurrentRuntimeConfig = func() support.RuntimeConfig {
		return support.RuntimeConfig{
			Downstream: model.DownstreamConfig{ReleaseServiceBaseURL: remote.URL},
		}
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
