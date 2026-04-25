package runtime

import (
	"testing"

	model "github.com/bsonger/devflow-service/internal/release/domain"
)

func TestManifestRegistryConfigFromConfigFallsBackToImageRegistry(t *testing.T) {
	cfg, enabled, err := ManifestRegistryConfigFromConfig(
		&model.ManifestRegistryRuntimeConfig{PlainHTTP: true},
		&model.ImageRegistryRuntimeConfig{
			Registry:  "registry.example.com",
			Namespace: "devflow",
			Username:  "image-user",
			Password:  "image-pass",
		},
	)
	if err != nil {
		t.Fatalf("ManifestRegistryConfigFromConfig() error = %v", err)
	}
	if !enabled {
		t.Fatal("expected manifest registry publishing to be enabled")
	}
	if cfg.Registry != "registry.example.com" || cfg.Namespace != "devflow" {
		t.Fatalf("unexpected cfg %+v", cfg)
	}
	if cfg.Repository != "manifests" {
		t.Fatalf("cfg.Repository = %q, want manifests", cfg.Repository)
	}
	if cfg.Username != "image-user" || cfg.Password != "image-pass" {
		t.Fatalf("unexpected credentials %+v", cfg)
	}
	if !cfg.PlainHTTP {
		t.Fatalf("expected PlainHTTP to be true, got %+v", cfg)
	}
}

func TestManifestRegistryConfigFromConfigRejectsPartialConfig(t *testing.T) {
	_, enabled, err := ManifestRegistryConfigFromConfig(
		&model.ManifestRegistryRuntimeConfig{Registry: "registry.example.com"},
		nil,
	)
	if err == nil {
		t.Fatal("expected error")
	}
	if enabled {
		t.Fatal("expected publishing to stay disabled on partial config")
	}
}
