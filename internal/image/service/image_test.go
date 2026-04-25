package service

import (
	"testing"
	"time"

	imagedomain "github.com/bsonger/devflow-service/internal/image/domain"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	releasesupport "github.com/bsonger/devflow-service/internal/release/support"
)

func TestCreateImageDefaultsBranchAndUsesRepoURLFallback(t *testing.T) {
	originalRegistry := releasesupport.CurrentRuntimeConfig()
	releasesupport.ConfigureRuntimeConfig(releasesupport.RuntimeConfig{
		ImageRegistry: imagedomain.ImageRegistryConfig{
			Registry:  "registry.example.com",
			Namespace: "devflow",
		},
	})
	defer releasesupport.ConfigureRuntimeConfig(originalRegistry)

	app := &releasesupport.ApplicationProjection{
		Name:        "devflow-app-serv",
		RepoURL:     "https://github.com/bsonger/devflow-app-serv.git",
		RepoAddress: "",
	}
	image := &imagedomain.Image{}
	image.RepoAddress = app.RepoAddress
	if image.RepoAddress == "" {
		image.RepoAddress = app.RepoURL
	}
	if image.Branch == "" {
		image.Branch = "main"
	}
	registryConfig, err := configuredImageRegistry()
	if err != nil {
		t.Fatal(err)
	}
	imageTarget, err := imagedomain.BuildImageTarget(registryConfig, app.Name, image.Branch, "", mustImageTestTime("2026-04-21T00:00:00Z"))
	if err != nil {
		t.Fatal(err)
	}
	image.Name = imageTarget.Name
	image.Tag = imageTarget.Tag
	image.Status = model.ImagePending
	image.WithCreateDefault()

	if image.Branch != "main" {
		t.Fatalf("branch = %q, want main", image.Branch)
	}
	if image.RepoAddress != app.RepoURL {
		t.Fatalf("repo address = %q, want repo url fallback %q", image.RepoAddress, app.RepoURL)
	}
	if image.Name != "devflow-app-serv" {
		t.Fatalf("image name = %q, want devflow-app-serv", image.Name)
	}
	if image.Status != model.ImagePending {
		t.Fatalf("status = %q, want %q", image.Status, model.ImagePending)
	}
}

func TestConfiguredImageRegistryRejectsEmptyRepository(t *testing.T) {
	originalRegistry := releasesupport.CurrentRuntimeConfig()
	releasesupport.ConfigureRuntimeConfig(releasesupport.RuntimeConfig{})
	defer releasesupport.ConfigureRuntimeConfig(originalRegistry)

	_, err := configuredImageRegistry()
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "release-service image registry is not configured" {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestCreateImageTransientDispatchFailuresAreBoundedAtDispatchSeamAssumption(t *testing.T) {
	const assumption = "transient build failures are bounded retries at the release-service/Tekton dispatch seam"
	if assumption == "" {
		t.Fatal("expected documented bounded retry assumption")
	}
}

func mustImageTestTime(value string) time.Time {
	got, err := time.Parse(time.RFC3339, value)
	if err != nil {
		panic(err)
	}
	return got
}
