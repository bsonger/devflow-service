package downstream

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFindAppConfigUsesEnvironmentScopedEntryOnly(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/app-configs":
			switch r.URL.RawQuery {
			case "application_id=app-1&environment_id=env-1":
				_, _ = io.WriteString(w, `{"data":[{"id":"cfg-env-1","application_id":"app-1","environment_id":"env-1"}]}`)
			default:
				t.Fatalf("unexpected query %s", r.URL.RawQuery)
			}
		case "/api/v1/app-configs/cfg-env-1":
			_, _ = io.WriteString(w, `{"data":{"id":"cfg-env-1","application_id":"app-1","environment_id":"env-1","mount_path":"/etc/config","source_directory":"checkout/web/staging","files":[{"name":"configuration.yaml","content":"foo: bar"}],"source_commit":"abc123"}}`)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer ts.Close()

	client := New(ts.URL)
	got, err := client.FindAppConfig(context.Background(), "app-1", "env-1")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.ID != "cfg-env-1" || got.EnvironmentID != "env-1" {
		t.Fatalf("unexpected config %+v", got)
	}
	if got.MountPath != "/etc/config" {
		t.Fatalf("expected mount path, got %+v", got)
	}
}

func TestFindAppConfigReturnsNilWhenEnvironmentEntryMissing(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/app-configs" || r.URL.RawQuery != "application_id=app-1&environment_id=env-1" {
			t.Fatalf("unexpected request path=%s query=%s", r.URL.Path, r.URL.RawQuery)
		}
		_, _ = io.WriteString(w, `{"data":[]}`)
	}))
	defer ts.Close()

	client := New(ts.URL)
	got, err := client.FindAppConfig(context.Background(), "app-1", "env-1")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("expected nil config, got %+v", got)
	}
}

func TestFindWorkloadConfigUsesApplicationScopedEntry(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/workload-configs":
			switch r.URL.RawQuery {
			case "application_id=app-1":
				_, _ = io.WriteString(w, `{"data":[{"id":"wc-base","application_id":"app-1","replicas":2}]}`)
			default:
				t.Fatalf("unexpected query %s", r.URL.RawQuery)
			}
		case "/api/v1/workload-configs/wc-base":
			_, _ = io.WriteString(w, `{"data":{"id":"wc-base","application_id":"app-1","replicas":2,"service_account_name":"default","labels":{"team":"platform"},"annotations":{"sidecar.istio.io/inject":"true"}}}`)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer ts.Close()

	client := New(ts.URL)
	got, err := client.FindWorkloadConfig(context.Background(), "app-1")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.ID != "wc-base" {
		t.Fatalf("unexpected config %+v", got)
	}
}
