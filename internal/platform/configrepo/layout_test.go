package configrepo

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestResolveLayoutFromServicePathAndEnv(t *testing.T) {
	resolved, err := resolveLayout(filepath.Join("testdata", "config-repo"), "applications/devflow-platform/services/devflow-app-service", "staging")
	if err != nil {
		t.Fatalf("resolveLayout returned error: %v", err)
	}
	if got := filepath.ToSlash(resolved.SourcePath); got != "applications/devflow-platform/services/devflow-app-service" {
		t.Fatalf("SourcePath = %q", got)
	}
	want := []layoutEntry{
		{Name: "configuration.yaml", DiskPath: filepath.Join("testdata", "config-repo", "applications/devflow-platform/services/devflow-app-service", "configuration.yaml")},
	}
	if !reflect.DeepEqual(resolved.Entries, want) {
		t.Fatalf("Entries = %#v, want %#v", resolved.Entries, want)
	}
}

func TestResolveLayoutFromServicePathBaseFilesOnly(t *testing.T) {
	resolved, err := resolveLayout(filepath.Join("testdata", "config-repo"), "applications/devflow-platform/services/devflow-app-service", "base")
	if err != nil {
		t.Fatalf("resolveLayout returned error: %v", err)
	}
	want := []layoutEntry{
		{Name: "configuration.yaml", DiskPath: filepath.Join("testdata", "config-repo", "applications/devflow-platform/services/devflow-app-service", "configuration.yaml")},
	}
	if !reflect.DeepEqual(resolved.Entries, want) {
		t.Fatalf("Entries = %#v, want %#v", resolved.Entries, want)
	}
}

func TestResolveLayoutLegacyFlatDirectoryStillWorks(t *testing.T) {
	resolved, err := resolveLayout(filepath.Join("testdata", "config-repo"), "apps/11111111-1111-1111-1111-111111111111/staging/configmap", "")
	if err != nil {
		t.Fatalf("resolveLayout returned error: %v", err)
	}
	want := []layoutEntry{
		{Name: "app.yaml", DiskPath: filepath.Join("testdata", "config-repo", "apps/11111111-1111-1111-1111-111111111111/staging/configmap", "app.yaml")},
		{Name: "logging.yaml", DiskPath: filepath.Join("testdata", "config-repo", "apps/11111111-1111-1111-1111-111111111111/staging/configmap", "logging.yaml")},
	}
	if !reflect.DeepEqual(resolved.Entries, want) {
		t.Fatalf("Entries = %#v, want %#v", resolved.Entries, want)
	}
}

func TestResolveLayoutIncludesAllRuntimeFilesAndEnvDirectoryOverlays(t *testing.T) {
	rootDir := t.TempDir()
	sourceDir := filepath.Join(rootDir, "applications/devflow-platform/services/devflow-demo-service")
	if err := os.MkdirAll(filepath.Join(sourceDir, "environments", "staging"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"config.yaml":  "server:\n  port: 8080\n",
		"logging.yaml": "log:\n  level: info\n",
		filepath.Join("environments", "staging", "config.yaml"):  "server:\n  port: 8081\n",
		filepath.Join("environments", "staging", "feature.yaml"): "feature:\n  enabled: true\n",
	} {
		path := filepath.Join(sourceDir, filepath.FromSlash(name))
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	resolved, err := resolveLayout(rootDir, "applications/devflow-platform/services/devflow-demo-service", "staging")
	if err != nil {
		t.Fatalf("resolveLayout returned error: %v", err)
	}
	want := []layoutEntry{
		{Name: "config.yaml", DiskPath: filepath.Join(sourceDir, "environments", "staging", "config.yaml")},
		{Name: "feature.yaml", DiskPath: filepath.Join(sourceDir, "environments", "staging", "feature.yaml")},
		{Name: "logging.yaml", DiskPath: filepath.Join(sourceDir, "logging.yaml")},
	}
	if !reflect.DeepEqual(resolved.Entries, want) {
		t.Fatalf("Entries = %#v, want %#v", resolved.Entries, want)
	}
}
