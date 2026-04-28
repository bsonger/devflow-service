package service

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/google/uuid"
)

func TestBuildReleaseBundleRendersConfigMapDeploymentServiceAndVirtualService(t *testing.T) {
	manifest := &manifestdomain.Manifest{
		BaseModel:     model.BaseModel{ID: uuid.New()},
		ApplicationID: uuid.New(),
		ImageRef:      "registry.example.com/devflow/demo-api@sha256:abc",
		ServicesSnapshot: []manifestdomain.ManifestService{
			{
				Name: "demo-api",
				Ports: []manifestdomain.ManifestServicePort{
					{Name: "http", ServicePort: 80, TargetPort: 8080, Protocol: "TCP"},
				},
			},
		},
		WorkloadConfigSnapshot: manifestdomain.ManifestWorkloadConfig{
			Name:     "demo-api",
			Replicas: 2,
			Resources: map[string]any{
				"limits": map[string]any{"cpu": "500m"},
			},
			Env: []model.EnvVar{{Name: "APP_ENV", Value: "prod"}},
		},
	}
	release := &model.Release{
		BaseModel:     model.BaseModel{ID: uuid.New()},
		ApplicationID: manifest.ApplicationID,
		EnvironmentID: "production",
		AppConfigSnapshot: model.ReleaseAppConfig{
			MountPath: "/etc/app-config",
			Data:      map[string]string{"application.yaml": "server:\n  port: 8080\n"},
		},
		RoutesSnapshot: []model.ReleaseRoute{{
			Name:        "demo-api",
			Host:        "api.example.com",
			Path:        "/",
			ServiceName: "demo-api",
			ServicePort: 80,
		}},
	}

	bundle, err := buildReleaseBundle("checkout", "demo-api", manifest, release)
	if err != nil {
		t.Fatalf("buildReleaseBundle failed: %v", err)
	}
	if bundle.Resources.ConfigMap == nil {
		t.Fatal("expected configmap")
	}
	if bundle.Resources.Deployment == nil {
		t.Fatal("expected deployment")
	}
	if bundle.Resources.VirtualService == nil {
		t.Fatal("expected virtualservice")
	}
	if len(bundle.Resources.Services) != 1 {
		t.Fatalf("services = %d", len(bundle.Resources.Services))
	}
	if len(bundle.Files) < 2 {
		t.Fatalf("expected bundle files, got %d", len(bundle.Files))
	}
	lastFile := bundle.Files[len(bundle.Files)-1]
	if lastFile.Path != "bundle.yaml" {
		t.Fatalf("expected bundle.yaml, got %q", lastFile.Path)
	}
	if !strings.Contains(lastFile.Content, "kind: ConfigMap") || !strings.Contains(lastFile.Content, "kind: Deployment") || !strings.Contains(lastFile.Content, "kind: VirtualService") {
		t.Fatalf("bundle.yaml missing expected kinds: %s", lastFile.Content)
	}
}

func TestBuildReleaseBundleUsesWorkloadNameFallback(t *testing.T) {
	manifest := &manifestdomain.Manifest{
		BaseModel:     model.BaseModel{ID: uuid.New()},
		ApplicationID: uuid.New(),
		ImageRef:      "registry.example.com/devflow/demo-api:latest",
		WorkloadConfigSnapshot: manifestdomain.ManifestWorkloadConfig{
			Name:     "workload-name",
			Replicas: 1,
		},
	}
	release := &model.Release{
		BaseModel:     model.BaseModel{ID: uuid.New()},
		ApplicationID: manifest.ApplicationID,
		EnvironmentID: "staging",
	}

	bundle, err := buildReleaseBundle("", "", manifest, release)
	if err != nil {
		t.Fatalf("buildReleaseBundle failed: %v", err)
	}
	if bundle.ArtifactName != "workload-name" {
		t.Fatalf("artifact_name = %q", bundle.ArtifactName)
	}
	if bundle.Resources.Deployment == nil || bundle.Resources.Deployment.Name != "workload-name" {
		t.Fatalf("unexpected deployment = %#v", bundle.Resources.Deployment)
	}
}

func TestReleaseBundleDigestUsesBundleYAML(t *testing.T) {
	bundle := &model.ReleaseBundle{
		Files: []model.ReleaseBundleFile{
			{Path: "01-deployment-demo.yaml", Content: "kind: Deployment\n"},
			{Path: "bundle.yaml", Content: "kind: ConfigMap\n---\nkind: Deployment\n"},
		},
	}
	got := releaseBundleDigest(bundle)
	sum := sha256.Sum256([]byte("kind: ConfigMap\n---\nkind: Deployment\n"))
	want := "sha256:" + hex.EncodeToString(sum[:])
	if got != want {
		t.Fatalf("digest = %q want %q", got, want)
	}
}
