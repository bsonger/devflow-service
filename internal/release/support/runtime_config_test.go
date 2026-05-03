package support

import (
	"testing"

	model "github.com/bsonger/devflow-service/internal/release/domain"
)

func TestManifestBuildTektonConfigFromModelDefaults(t *testing.T) {
	cfg := ManifestBuildTektonConfigFromModel(nil)

	if cfg.Namespace != "tekton-pipelines" {
		t.Fatalf("namespace = %q", cfg.Namespace)
	}
	if cfg.BuildPipeline != "devflow-tekton-image-build-push-only" {
		t.Fatalf("build pipeline = %q", cfg.BuildPipeline)
	}
	if cfg.PVCGenerateName != "devflow-tekton-image-build-push-only" {
		t.Fatalf("pvc generate name = %q", cfg.PVCGenerateName)
	}
}

func TestManifestBuildTektonConfigFromModelUsesConfiguredValues(t *testing.T) {
	cfg := ManifestBuildTektonConfigFromModel(&model.TektonConfig{
		Namespace:       "tekton-custom",
		BuildPipeline:   "build-custom",
		PVCGenerateName: "pvc-custom",
	})

	if cfg.Namespace != "tekton-custom" {
		t.Fatalf("namespace = %q", cfg.Namespace)
	}
	if cfg.BuildPipeline != "build-custom" {
		t.Fatalf("build pipeline = %q", cfg.BuildPipeline)
	}
	if cfg.PVCGenerateName != "pvc-custom" {
		t.Fatalf("pvc generate name = %q", cfg.PVCGenerateName)
	}
}
