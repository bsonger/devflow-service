package service

import (
	"strings"
	"testing"

	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/google/uuid"
)

func TestBuildManifestResourcesViewRemainsInspectionOnly(t *testing.T) {
	applicationID := uuid.New()
	manifestID := uuid.New()
	manifest := &manifestdomain.Manifest{
		BaseModel:     model.BaseModel{ID: manifestID},
		ApplicationID: applicationID,
		ImageRef:      "registry.example.com/devflow/demo-api@sha256:abc",
		ServicesSnapshot: []manifestdomain.ManifestService{{
			Name: "demo-api",
			Ports: []manifestdomain.ManifestServicePort{{
				Name:        "http",
				ServicePort: 80,
				TargetPort:  8080,
				Protocol:    "TCP",
			}},
		}},
		WorkloadConfigSnapshot: manifestdomain.ManifestWorkloadConfig{
			Replicas:           2,
			ServiceAccountName: "demo-api",
			Resources: map[string]any{
				"limits": map[string]any{"cpu": "500m"},
			},
			Env:         []model.EnvVar{{Name: "APP_ENV", Value: "prod"}},
			Annotations: map[string]string{"devflow.io/build-snapshot": "true"},
		},
	}

	view, err := buildManifestResourcesView(manifest)
	if err != nil {
		t.Fatalf("buildManifestResourcesView failed: %v", err)
	}
	if view == nil {
		t.Fatal("expected manifest resources view")
	}
	if view.ManifestID != manifestID {
		t.Fatalf("manifest view id = %s want %s", view.ManifestID, manifestID)
	}
	if view.ApplicationID != applicationID {
		t.Fatalf("manifest view application id = %s want %s", view.ApplicationID, applicationID)
	}
	if view.Resources.ConfigMap != nil {
		t.Fatalf("manifest view should not render release configmap: %#v", view.Resources.ConfigMap)
	}
	if view.Resources.VirtualService != nil {
		t.Fatalf("manifest view should not render release virtualservice: %#v", view.Resources.VirtualService)
	}
	if view.Resources.Rollout != nil {
		t.Fatalf("manifest view should not render release rollout: %#v", view.Resources.Rollout)
	}
	if view.Resources.Deployment == nil {
		t.Fatal("manifest view missing deployment")
	}
	if len(view.Resources.Services) != 1 {
		t.Fatalf("manifest services = %d want 1", len(view.Resources.Services))
	}
	if !strings.Contains(view.Resources.Deployment.YAML, "kind: Deployment") {
		t.Fatalf("manifest deployment yaml missing deployment kind: %s", view.Resources.Deployment.YAML)
	}
	podSpec, ok := view.Resources.Deployment.Object["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)
	if !ok {
		t.Fatalf("manifest deployment pod spec missing: %#v", view.Resources.Deployment.Object)
	}
	if _, ok := podSpec["volumes"]; ok {
		t.Fatalf("manifest view should not include release config volumes: %#v", podSpec["volumes"])
	}
	if got := podSpec["serviceAccountName"]; got != "demo-api" {
		t.Fatalf("manifest serviceAccountName = %#v", got)
	}
}
