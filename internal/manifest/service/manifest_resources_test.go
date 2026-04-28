package service

import (
	"testing"

	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/google/uuid"
)

func TestBuildManifestResourcesViewDerivesResourcesFromSnapshots(t *testing.T) {
	manifestID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	appID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	manifest := &manifestdomain.Manifest{
		BaseModel:     model.BaseModel{ID: manifestID},
		ApplicationID: appID,
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
			Replicas: 2,
			Env:      []model.EnvVar{{Name: "APP_ENV", Value: "prod"}},
		},
	}

	got, err := buildManifestResourcesView(manifest)
	if err != nil {
		t.Fatalf("buildManifestResourcesView() error = %v", err)
	}
	if got.ManifestID != manifestID {
		t.Fatalf("ManifestID = %s want %s", got.ManifestID, manifestID)
	}
	if got.Resources.ConfigMap != nil || got.Resources.VirtualService != nil || got.Resources.Rollout != nil {
		t.Fatalf("expected only derived service/deployment resources, got %+v", got.Resources)
	}
	if got.Resources.Deployment == nil || got.Resources.Deployment.Kind != "Deployment" {
		t.Fatalf("expected deployment resource, got %#v", got.Resources.Deployment)
	}
	if len(got.Resources.Services) != 1 || got.Resources.Services[0].Name != "demo-api" {
		t.Fatalf("expected one service resource, got %#v", got.Resources.Services)
	}
	if got.Resources.Deployment.Object["kind"] != "Deployment" {
		t.Fatalf("expected decoded deployment object, got %#v", got.Resources.Deployment.Object)
	}
}
