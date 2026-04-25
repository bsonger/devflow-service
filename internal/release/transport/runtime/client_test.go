package runtimeclient

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestClientGetRuntimeSpecRevision(t *testing.T) {
	revisionID := uuid.New()
	runtimeSpecID := uuid.New()
	client := New("http://runtime.example")
	client.http = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/api/v1/runtime-spec-revisions/"+revisionID.String() {
				t.Fatalf("unexpected path %s", r.URL.Path)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"id":"` + revisionID.String() + `","runtime_spec_id":"` + runtimeSpecID.String() + `"}`)),
			}, nil
		}),
	}
	got, err := client.GetRuntimeSpecRevision(context.Background(), revisionID)
	if err != nil {
		t.Fatalf("GetRuntimeSpecRevision: %v", err)
	}
	if got.ID != revisionID || got.RuntimeSpecID != runtimeSpecID {
		t.Fatalf("unexpected revision payload: %+v", got)
	}
}
