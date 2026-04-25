package downstream

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientGetsEnvironment(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/environments/env-1" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"id":"env-1","name":"staging","cluster_id":"cluster-1"}`)
	}))
	defer ts.Close()

	client := New(ts.URL)
	env, err := client.GetEnvironment(context.Background(), "env-1")
	if err != nil {
		t.Fatal(err)
	}
	if env.ID != "env-1" || env.ClusterID != "cluster-1" {
		t.Fatalf("unexpected payload %+v", env)
	}
}
