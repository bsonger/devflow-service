package runtimeclient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bsonger/devflow-service/internal/shared/downstreamhttp"
	"github.com/google/uuid"
)

func TestClientGetRuntimeSpecRevision(t *testing.T) {
	revisionID := uuid.New()
	runtimeSpecID := uuid.New()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/runtime-spec-revisions/"+revisionID.String() {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"id":"`+revisionID.String()+`","runtime_spec_id":"`+runtimeSpecID.String()+`"}`)
	}))
	defer ts.Close()

	got, err := New(ts.URL).GetRuntimeSpecRevision(context.Background(), revisionID)
	if err != nil {
		t.Fatalf("GetRuntimeSpecRevision: %v", err)
	}
	if got.ID != revisionID || got.RuntimeSpecID != runtimeSpecID {
		t.Fatalf("unexpected revision payload: %+v", got)
	}
}

func TestClientReturnsTypedStatusError(t *testing.T) {
	specID := uuid.New()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "missing", http.StatusNotFound)
	}))
	defer ts.Close()

	_, err := New(ts.URL).GetRuntimeSpec(context.Background(), specID)
	if !downstreamhttp.IsStatus(err, http.StatusNotFound) {
		t.Fatalf("expected downstream 404 classification, err = %v", err)
	}
}

func TestClientEmptyBaseURLReturnsRuntimeServiceUnavailable(t *testing.T) {
	_, err := New("").GetRuntimeSpec(context.Background(), uuid.New())
	if !errors.Is(err, ErrRuntimeServiceUnavailable) {
		t.Fatalf("error = %v, want %v", err, ErrRuntimeServiceUnavailable)
	}
}
