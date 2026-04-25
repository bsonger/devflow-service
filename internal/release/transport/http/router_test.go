package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bsonger/devflow-service/internal/platform/logger"
)

func TestNewRouterWithOptionsRegistersReleaseSwaggerRoutes(t *testing.T) {
	logger.InitZapLogger(&logger.Config{Level: "info", Format: "console"})
	r := NewRouterWithOptions(Options{
		ServiceName:   "release-service",
		EnableSwagger: true,
	})

	cases := []struct {
		path string
		want int
	}{
		{path: "/healthz", want: http.StatusOK},
		{path: "/readyz", want: http.StatusOK},
		{path: "/swagger/index.html", want: http.StatusOK},
		{path: "/api/v1/release/swagger/index.html", want: http.StatusOK},
	}

	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Fatalf("path %s: got %d want %d body=%s", tc.path, rec.Code, tc.want, rec.Body.String())
		}
	}
}
