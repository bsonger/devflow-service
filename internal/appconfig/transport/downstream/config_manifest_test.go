package downstream

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	workloadconfigdomain "github.com/bsonger/devflow-service/internal/workloadconfig/domain"
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
			_, _ = io.WriteString(w, `{"data":{"id":"wc-base","application_id":"app-1","replicas":2,"service_account_name":"default","resources":{"size_class":"medium","requests":{"cpu":"250m","memory":"256Mi"},"limits":{"cpu":"1","memory":"1Gi"}},"probes":{"liveness":{"path":"/healthz","port":"http","period_seconds":10}},"env":[{"name":"LOG_LEVEL","value":"info"}],"labels":{"team":"platform"},"annotations":{"sidecar.istio.io/inject":"true"}}}`)
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
	if got.Resources.SizeClass != workloadconfigdomain.WorkloadSizeClassMedium {
		t.Fatalf("resources.size_class = %q", got.Resources.SizeClass)
	}
	if got.Resources.Requests.CPU != "250m" || got.Resources.Limits.Memory != "1Gi" {
		t.Fatalf("unexpected resources %+v", got.Resources)
	}
	if got.Probes.Liveness == nil || got.Probes.Liveness.Path != "/healthz" || got.Probes.Liveness.Port != "http" {
		t.Fatalf("unexpected probes %+v", got.Probes)
	}
	if len(got.Env) != 1 || got.Env[0].Name != "LOG_LEVEL" {
		t.Fatalf("unexpected env %+v", got.Env)
	}
}

func TestFindWorkloadConfigReturnsNilWhenCleanupDeletedLegacyRow(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workload-configs" || r.URL.RawQuery != "application_id=app-1" {
			t.Fatalf("unexpected request path=%s query=%s", r.URL.Path, r.URL.RawQuery)
		}
		_, _ = io.WriteString(w, `{"data":[]}`)
	}))
	defer ts.Close()

	client := New(ts.URL)
	got, err := client.FindWorkloadConfig(context.Background(), "app-1")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("expected nil config after cleanup deleted legacy row, got %+v", got)
	}
}
