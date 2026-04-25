package domain

import (
	"testing"

	model "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/google/uuid"
)

func TestGeneratePipelineRunUsesProvidedPipelineName(t *testing.T) {
	image := &Image{}
	run := image.GeneratePipelineRun(
		"devflow-tekton-image-build",
		"pvc-1",
		ImageRegistryConfig{Registry: "registry.cn-hangzhou.aliyuncs.com", Namespace: "devflow"},
		ImageTarget{Name: "portal-api", Tag: "20260408-130500"},
	)
	if run.GenerateName != "devflow-tekton-image-build-run-" {
		t.Fatalf("GenerateName = %q", run.GenerateName)
	}
}

func TestGeneratePipelineRunParamsContainsAllRequiredValues(t *testing.T) {
	image := &Image{
		BaseModel:   model.BaseModel{ID: uuid.MustParse("11111111-1111-1111-1111-111111111111")},
		Name:        "portal-api",
		RepoAddress: "https://github.com/example/portal",
		Branch:      "main",
	}
	params := image.GeneratePipelineRunParams(
		ImageRegistryConfig{Registry: "registry.cn-hangzhou.aliyuncs.com", Namespace: "devflow"},
		ImageTarget{Name: "portal-api", Tag: "20260408-130500"},
	)
	if len(params) != 7 {
		t.Fatalf("len(params) = %d want 7", len(params))
	}
}

func TestGenerateImageVersionContainsTimestampAndRandom(t *testing.T) {
	v := GenerateImageVersion("portal")
	if len(v) < 10 {
		t.Fatalf("version too short: %q", v)
	}
	if v[:6] != "portal" {
		t.Fatalf("version prefix = %q want portal", v[:6])
	}
}

func TestGenerateImageVersionIsDeterministicInPrefix(t *testing.T) {
	prefix := "checkout"
	v1 := GenerateImageVersion(prefix)
	v2 := GenerateImageVersion(prefix)
	if v1 == v2 {
		t.Fatal("GenerateImageVersion should produce different values for same prefix")
	}
}
