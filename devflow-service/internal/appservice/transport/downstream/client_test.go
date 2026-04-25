package downstream

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNetworkClientTreatsNullEnvelopeDataAsEmptyList(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/applications/app-1/routes" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"data":null,"pagination":{"total":0}}`)
	}))
	defer ts.Close()

	client := New(ts.URL)
	got, err := client.ListRoutes(context.Background(), "app-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("routes = %+v, want empty", got)
	}
}
