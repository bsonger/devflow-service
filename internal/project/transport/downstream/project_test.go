package downstream

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientGetsProject(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/projects/proj-1" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"data":{"id":"proj-1","name":"checkout"}}`)
	}))
	defer ts.Close()

	client := New(ts.URL)
	project, err := client.GetProject(context.Background(), "proj-1")
	if err != nil {
		t.Fatal(err)
	}
	if project.Name != "checkout" {
		t.Fatalf("unexpected payload %+v", project)
	}
}
