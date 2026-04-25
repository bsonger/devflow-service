package downstream

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientGetsCluster(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/clusters/cluster-1" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"data":{"id":"cluster-1","name":"staging","server":"https://k8s.staging.example.com","onboarding_ready":true,"onboarding_checked_at":"2026-04-19T00:00:00Z"}}`)
	}))
	defer ts.Close()

	client := New(ts.URL)
	cluster, err := client.GetCluster(context.Background(), "cluster-1")
	if err != nil {
		t.Fatal(err)
	}
	if !cluster.OnboardingReady || cluster.Server != "https://k8s.staging.example.com" {
		t.Fatalf("unexpected payload %+v", cluster)
	}
}
