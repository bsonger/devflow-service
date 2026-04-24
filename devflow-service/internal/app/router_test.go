package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bsonger/devflow-service/internal/platform/logger"
)

type statusResponse struct {
	Service string `json:"service"`
	Status  string `json:"status"`
}

func TestNewRouterWithOptionsRegistersMetaServiceRoutesAndIdentity(t *testing.T) {
	loggingx.InitZapLogger(&loggingx.Config{Level: "info", Format: "console"})
	r := NewRouterWithOptions(Options{
		ServiceName:   "meta-service",
		EnableSwagger: true,
		Modules: []Module{
			ModuleProject,
			ModuleApplication,
			ModuleCluster,
			ModuleEnvironment,
		},
	})

	cases := []struct {
		path       string
		want       int
		wantStatus string
		assertBody bool
	}{
		{path: "/healthz", want: http.StatusOK, wantStatus: "ok", assertBody: true},
		{path: "/readyz", want: http.StatusOK, wantStatus: "ready", assertBody: true},
		{path: "/swagger/index.html", want: http.StatusOK},
		{path: "/api/v1/app/swagger/index.html", want: http.StatusOK},
	}

	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Fatalf("path %s: got %d want %d body=%s", tc.path, rec.Code, tc.want, rec.Body.String())
		}
		if !tc.assertBody {
			continue
		}

		var payload statusResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("path %s: decode body: %v", tc.path, err)
		}
		if payload.Service != "meta-service" || payload.Status != tc.wantStatus {
			t.Fatalf("path %s: unexpected payload %#v", tc.path, payload)
		}
	}
}

func TestNewRouterWithOptionsRegistersClusterAndEnvironmentRoutesAlongsideExistingModules(t *testing.T) {
	loggingx.InitZapLogger(&loggingx.Config{Level: "info", Format: "console"})
	r := NewRouterWithOptions(Options{
		ServiceName:   "meta-service",
		EnableSwagger: false,
		Modules: []Module{
			ModuleProject,
			ModuleApplication,
			ModuleCluster,
			ModuleEnvironment,
			ModuleCluster,
			ModuleEnvironment,
		},
	})

	routes := make(map[string]int)
	for _, route := range r.Routes() {
		routes[route.Method+" "+route.Path]++
	}

	for _, path := range []string{
		"GET /api/v1/projects",
		"GET /api/v1/applications",
		"GET /api/v1/clusters",
		"GET /api/v1/environments",
		"POST /api/v1/clusters",
		"POST /api/v1/environments",
		"PUT /api/v1/clusters/:id",
		"PUT /api/v1/environments/:id",
		"DELETE /api/v1/clusters/:id",
		"DELETE /api/v1/environments/:id",
	} {
		if routes[path] != 1 {
			t.Fatalf("route %s registered %d times, want 1", path, routes[path])
		}
	}
}
