package downstream

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFindAppConfigFallsBackToBaseEnvironmentEntry(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/app-configs":
			switch r.URL.RawQuery {
			case "application_id=app-1&environment_id=env-1":
				_, _ = io.WriteString(w, `{"data":[]}`)
			case "application_id=app-1":
				_, _ = io.WriteString(w, `{"data":[{"id":"cfg-base","application_id":"app-1","environment_id":"base","name":"base"}]}`)
			default:
				t.Fatalf("unexpected query %s", r.URL.RawQuery)
			}
		case "/api/v1/app-configs/cfg-base":
			_, _ = io.WriteString(w, `{"data":{"id":"cfg-base","application_id":"app-1","environment_id":"base","name":"base","mount_path":"/etc/devflow/config","files":[{"name":"configuration.yaml","content":"foo: bar"}],"rendered_configmap":{"data":{"configuration.yaml":"foo: bar"}}}}`)
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
	if got == nil || got.ID != "cfg-base" || got.EnvironmentID != "base" {
		t.Fatalf("unexpected config %+v", got)
	}
	if got.MountPath != "/etc/devflow/config" {
		t.Fatalf("expected mount path, got %+v", got)
	}
}

func TestFindWorkloadConfigFallsBackToBaseEnvironmentEntry(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/workload-configs":
			switch r.URL.RawQuery {
			case "application_id=app-1&environment_id=env-1":
				_, _ = io.WriteString(w, `{"data":[]}`)
			case "application_id=app-1":
				_, _ = io.WriteString(w, `{"data":[{"id":"wc-base","application_id":"app-1","environment_id":"base","name":"base-workload","replicas":2,"workload_type":"deployment"}]}`)
			default:
				t.Fatalf("unexpected query %s", r.URL.RawQuery)
			}
		case "/api/v1/workload-configs/wc-base":
			_, _ = io.WriteString(w, `{"data":{"id":"wc-base","application_id":"app-1","environment_id":"base","name":"base-workload","replicas":2,"workload_type":"deployment","strategy":"rolling-update"}}`)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer ts.Close()

	client := New(ts.URL)
	got, err := client.FindWorkloadConfig(context.Background(), "app-1", "env-1")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.ID != "wc-base" || got.EnvironmentID != "base" {
		t.Fatalf("unexpected config %+v", got)
	}
}
