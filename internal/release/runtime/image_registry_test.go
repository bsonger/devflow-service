package runtime

import (
	"testing"

	model "github.com/bsonger/devflow-service/internal/release/domain"
)

func TestImageRegistryConfigFromConfigRequiresRegistryAndNamespace(t *testing.T) {
	_, err := ImageRegistryConfigFromConfig(&model.ImageRegistryRuntimeConfig{})
	if err == nil {
		t.Fatal("expected error when registry config is missing")
	}
}

func TestImageRegistryConfigFromConfigReadsValues(t *testing.T) {
	cfg, err := ImageRegistryConfigFromConfig(&model.ImageRegistryRuntimeConfig{
		Registry:  "registry.cn-hangzhou.aliyuncs.com",
		Namespace: "devflow",
		Username:  "user",
		Password:  "pass",
	})
	if err != nil {
		t.Fatalf("ImageRegistryConfigFromConfig returned error: %v", err)
	}
	if cfg.Repository() != "registry.cn-hangzhou.aliyuncs.com/devflow" {
		t.Fatalf("Repository = %q", cfg.Repository())
	}
	if cfg.Username != "user" || cfg.Password != "pass" {
		t.Fatalf("unexpected credentials: %+v", cfg)
	}
}
