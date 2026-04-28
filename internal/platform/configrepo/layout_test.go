package configrepo

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestResolveLayoutFromDirectEnvironmentDirectory(t *testing.T) {
	resolved, err := resolveLayout(filepath.Join("testdata", "config-repo"), "devflow/devflow-app-service/staging", "")
	if err != nil {
		t.Fatalf("resolveLayout returned error: %v", err)
	}
	if got := filepath.ToSlash(resolved.SourcePath); got != "devflow/devflow-app-service/staging" {
		t.Fatalf("SourcePath = %q", got)
	}
	want := []layoutEntry{
		{Name: "configuration.yaml", DiskPath: filepath.Join("testdata", "config-repo", "devflow", "devflow-app-service", "staging", "configuration.yaml")},
	}
	if !reflect.DeepEqual(resolved.Entries, want) {
		t.Fatalf("Entries = %#v, want %#v", resolved.Entries, want)
	}
}

func TestResolveLayoutAppendsEnvironmentWhenSourcePathIsApplicationDirectory(t *testing.T) {
	resolved, err := resolveLayout(filepath.Join("testdata", "config-repo"), "devflow/devflow-app-service", "staging")
	if err != nil {
		t.Fatalf("resolveLayout returned error: %v", err)
	}
	want := []layoutEntry{
		{Name: "configuration.yaml", DiskPath: filepath.Join("testdata", "config-repo", "devflow", "devflow-app-service", "staging", "configuration.yaml")},
	}
	if !reflect.DeepEqual(resolved.Entries, want) {
		t.Fatalf("Entries = %#v, want %#v", resolved.Entries, want)
	}
}

func TestResolveLayoutReturnsAllFilesInEnvironmentDirectory(t *testing.T) {
	rootDir := t.TempDir()
	sourceDir := filepath.Join(rootDir, "devflow", "devflow-demo-service", "staging")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"config.yaml":  "server:\n  port: 8081\n",
		"feature.yaml": "feature:\n  enabled: true\n",
		"logging.yaml": "log:\n  level: info\n",
	} {
		path := filepath.Join(sourceDir, filepath.FromSlash(name))
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	resolved, err := resolveLayout(rootDir, "devflow/devflow-demo-service/staging", "")
	if err != nil {
		t.Fatalf("resolveLayout returned error: %v", err)
	}
	want := []layoutEntry{
		{Name: "config.yaml", DiskPath: filepath.Join(sourceDir, "config.yaml")},
		{Name: "feature.yaml", DiskPath: filepath.Join(sourceDir, "feature.yaml")},
		{Name: "logging.yaml", DiskPath: filepath.Join(sourceDir, "logging.yaml")},
	}
	if !reflect.DeepEqual(resolved.Entries, want) {
		t.Fatalf("Entries = %#v, want %#v", resolved.Entries, want)
	}
}
