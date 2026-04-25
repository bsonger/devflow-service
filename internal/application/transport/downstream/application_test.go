package downstream

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientGetsApplication(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/applications/app-1" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"data":{"id":"app-1","project_id":"proj-1","name":"checkout"}}`)
	}))
	defer ts.Close()

	client := New(ts.URL)
	app, err := client.GetApplication(context.Background(), "app-1")
	if err != nil {
		t.Fatal(err)
	}
	if app.ProjectID != "proj-1" || app.Name != "checkout" {
		t.Fatalf("unexpected payload %+v", app)
	}
}
