package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bsonger/devflow-service/internal/platform/logger"
)

func TestNewRouterWithOptionsRegistersNetworkSwaggerRoutes(t *testing.T) {
	logger.InitZapLogger(&logger.Config{Level: "info", Format: "console"})
	r := NewRouterWithOptions(Options{
		ServiceName:   "network-service",
		EnableSwagger: true,
		Modules: []Module{
			ModuleAppService,
			ModuleAppRoute,
		},
	})

	cases := []struct {
		path       string
		want       int
		assertBody bool
	}{
		{path: "/healthz", want: http.StatusOK, assertBody: true},
		{path: "/readyz", want: http.StatusOK, assertBody: true},
		{path: "/internal/status", want: http.StatusOK, assertBody: true},
		{path: "/swagger/index.html", want: http.StatusOK},
		{path: "/api/v1/network/swagger/index.html", want: http.StatusOK},
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

		var payload struct {
			Service   string `json:"service"`
			RequestID string `json:"request_id"`
			HTTP      struct {
				Modules []string `json:"modules"`
			} `json:"http"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("path %s: decode body: %v", tc.path, err)
		}
		if payload.Service != "network-service" {
			t.Fatalf("path %s: unexpected service %q", tc.path, payload.Service)
		}
		if payload.RequestID == "" {
			t.Fatalf("path %s: expected request_id in payload", tc.path)
		}
		if tc.path == "/internal/status" && len(payload.HTTP.Modules) == 0 {
			t.Fatalf("path %s: expected modules in payload", tc.path)
		}
	}
}
