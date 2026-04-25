package service

import (
	"testing"

	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/google/uuid"
)

func TestBuildManifestResourcesViewGroupsFrozenObjects(t *testing.T) {
	manifestID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	appID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	imageID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	manifest := &manifestdomain.Manifest{
		BaseModel:     model.BaseModel{ID: manifestID},
		ApplicationID: appID,
		EnvironmentID: "staging",
		ImageID:       imageID,
		RenderedObjects: []manifestdomain.ManifestRenderedObject{
			{
				Kind:      "ConfigMap",
				Name:      "demo-api-config",
				Namespace: "staging",
				YAML: `apiVersion: v1
kind: ConfigMap
metadata:
  name: demo-api-config
  namespace: staging
data:
  config.yaml: "foo: bar"
`,
			},
			{
				Kind:      "Service",
				Name:      "demo-api",
				Namespace: "staging",
				YAML: `apiVersion: v1
kind: Service
metadata:
  name: demo-api
  namespace: staging
spec:
  ports:
    - port: 80
`,
			},
			{
				Kind:      "VirtualService",
				Name:      "demo-api",
				Namespace: "staging",
				YAML: `apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: demo-api
  namespace: staging
spec:
  hosts:
    - demo.example.com
`,
			},
			{
				Kind:      "Deployment",
				Name:      "demo-api",
				Namespace: "staging",
				YAML: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo-api
  namespace: staging
spec:
  replicas: 2
`,
			},
		},
	}

	got, err := buildManifestResourcesView(manifest)
	if err != nil {
		t.Fatalf("buildManifestResourcesView() error = %v", err)
	}
	if got.ManifestID != manifestID {
		t.Fatalf("ManifestID = %s want %s", got.ManifestID, manifestID)
	}
	if got.Resources.ConfigMap == nil || got.Resources.ConfigMap.Name != "demo-api-config" {
		t.Fatalf("expected configmap resource, got %#v", got.Resources.ConfigMap)
	}
	if got.Resources.Deployment == nil || got.Resources.Deployment.Kind != "Deployment" {
		t.Fatalf("expected deployment resource, got %#v", got.Resources.Deployment)
	}
	if got.Resources.Rollout != nil {
		t.Fatalf("expected rollout to be nil, got %#v", got.Resources.Rollout)
	}
	if len(got.Resources.Services) != 1 || got.Resources.Services[0].Name != "demo-api" {
		t.Fatalf("expected one service resource, got %#v", got.Resources.Services)
	}
	if got.Resources.VirtualService == nil || got.Resources.VirtualService.Name != "demo-api" {
		t.Fatalf("expected virtualservice resource, got %#v", got.Resources.VirtualService)
	}
	if got.Resources.Deployment.Object["kind"] != "Deployment" {
		t.Fatalf("expected decoded deployment object, got %#v", got.Resources.Deployment.Object)
	}
	metadata, _ := got.Resources.ConfigMap.Object["metadata"].(map[string]any)
	if metadata["name"] != "demo-api-config" {
		t.Fatalf("expected decoded metadata, got %#v", got.Resources.ConfigMap.Object)
	}
	if len(got.RenderedObjects) != 4 {
		t.Fatalf("RenderedObjects len = %d want 4", len(got.RenderedObjects))
	}
}
