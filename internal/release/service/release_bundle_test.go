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
			Replicas:           2,
			ServiceAccountName: "demo-api",
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
	if !strings.Contains(bundle.Files[len(bundle.Files)-1].Content, "kind: ServiceAccount") {
		t.Fatalf("bundle.yaml missing serviceaccount: %s", bundle.Files[len(bundle.Files)-1].Content)
	}
	containerSpec, ok := bundle.Resources.Deployment.Object["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)["containers"].([]map[string]any)
	if !ok || len(containerSpec) == 0 {
		t.Fatalf("deployment containers missing: %#v", bundle.Resources.Deployment.Object)
	}
	if got := bundle.Resources.Deployment.Object["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)["serviceAccountName"]; got != "demo-api" {
		t.Fatalf("serviceAccountName = %#v", got)
	}
	ports, ok := containerSpec[0]["ports"].([]map[string]any)
	if !ok || len(ports) != 1 {
		t.Fatalf("deployment ports missing: %#v", containerSpec[0])
	}
	if ports[0]["name"] != "http" || ports[0]["containerPort"] != 8080 {
		t.Fatalf("deployment port = %#v", ports[0])
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
	if !strings.Contains(lastFile.Content, "kind: ConfigMap") || !strings.Contains(lastFile.Content, "kind: ServiceAccount") || !strings.Contains(lastFile.Content, "kind: Deployment") || !strings.Contains(lastFile.Content, "kind: VirtualService") {
		t.Fatalf("bundle.yaml missing expected kinds: %s", lastFile.Content)
	}
}

func TestBuildReleaseBundleFallsBackToApplicationIDWithoutServiceName(t *testing.T) {
	manifest := &manifestdomain.Manifest{
		BaseModel:     model.BaseModel{ID: uuid.New()},
		ApplicationID: uuid.New(),
		ImageRef:      "registry.example.com/devflow/demo-api:latest",
		WorkloadConfigSnapshot: manifestdomain.ManifestWorkloadConfig{
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
	if bundle.ArtifactName != manifest.ApplicationID.String() {
		t.Fatalf("artifact_name = %q", bundle.ArtifactName)
	}
	if bundle.Resources.Deployment == nil || bundle.Resources.Deployment.Name != manifest.ApplicationID.String() {
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

func TestBuildReleaseBundleRendersRolloutForBlueGreenStrategy(t *testing.T) {
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
			Replicas: 1,
		},
	}
	release := &model.Release{
		BaseModel:     model.BaseModel{ID: uuid.New()},
		ApplicationID: manifest.ApplicationID,
		EnvironmentID: "production",
		Strategy:      string(model.ReleaseStrategyBlueGreen),
	}

	bundle, err := buildReleaseBundle("checkout", "demo-api", manifest, release)
	if err != nil {
		t.Fatalf("buildReleaseBundle failed: %v", err)
	}
	if bundle.Resources.Rollout == nil {
		t.Fatal("expected rollout")
	}
	if bundle.Resources.Deployment != nil {
		t.Fatalf("expected deployment to be nil, got %#v", bundle.Resources.Deployment)
	}
	if len(bundle.Resources.Services) != 2 {
		t.Fatalf("services = %d want 2", len(bundle.Resources.Services))
	}
	if bundle.Resources.Services[1].Name != "demo-api-preview" {
		t.Fatalf("preview service = %q", bundle.Resources.Services[1].Name)
	}
}
