package openapi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-openapi/spec"
	"sigs.k8s.io/yaml"
)

func TestDevflowOpenAPIContract(t *testing.T) {
	specs := []struct {
		file            string
		requiredRoutes  []string
		protectedRoutes []string
	}{
		{
			file: "devflow.yaml",
			requiredRoutes: []string{
				"/api/v1/runtime/workload",
				"/api/v1/runtime/pods",
				"/api/v1/runtime/pods/{pod_name}",
				"/api/v1/runtime/rollouts",
				"/api/v1/meta/projects",
				"/api/v1/config/app-configs",
				"/api/v1/network/services",
				"/api/v1/release/releases",
				"/api/v1/release/manifests/tekton/status",
			},
			protectedRoutes: []string{
				"/api/v1/release/verify/argo/events",
				"/api/v1/release/verify/release/steps",
				"/api/v1/release/verify/release/artifact",
				"/api/v1/release/manifests/tekton/status",
				"/api/v1/release/manifests/tekton/tasks",
				"/api/v1/release/manifests/tekton/result",
			},
		},
		{
			file: "meta-service.yaml",
			requiredRoutes: []string{
				"/api/v1/meta/projects",
				"/api/v1/meta/applications",
				"/api/v1/meta/applications/{id}/environments",
				"/api/v1/meta/clusters",
				"/api/v1/meta/environments",
			},
		},
		{
			file: "network-service.yaml",
			requiredRoutes: []string{
				"/api/v1/network/services",
				"/api/v1/network/routes",
				"/api/v1/network/routes:validate",
			},
		},
		{
			file: "config-service.yaml",
			requiredRoutes: []string{
				"/api/v1/config/app-configs",
				"/api/v1/config/workload-configs",
			},
		},
		{
			file: "release-service.yaml",
			requiredRoutes: []string{
				"/api/v1/release/manifests",
				"/api/v1/release/manifests/tekton/status",
				"/api/v1/release/manifests/tekton/tasks",
				"/api/v1/release/manifests/tekton/result",
				"/api/v1/release/intents",
				"/api/v1/release/releases",
				"/api/v1/release/verify/argo/events",
				"/api/v1/release/verify/release/steps",
				"/api/v1/release/verify/release/artifact",
			},
			protectedRoutes: []string{
				"/api/v1/release/manifests/tekton/status",
				"/api/v1/release/manifests/tekton/tasks",
				"/api/v1/release/manifests/tekton/result",
				"/api/v1/release/verify/argo/events",
				"/api/v1/release/verify/release/steps",
				"/api/v1/release/verify/release/artifact",
			},
		},
		{
			file: "runtime-service.yaml",
			requiredRoutes: []string{
				"/api/v1/runtime/workload",
				"/api/v1/runtime/pods",
				"/api/v1/runtime/pods/{pod_name}",
				"/api/v1/runtime/rollouts",
			},
		},
	}

	for _, tc := range specs {
		t.Run(tc.file, func(t *testing.T) {
			doc := loadSwaggerFile(t, filepath.Join(".", tc.file))
			if doc.Swagger != "2.0" {
				t.Fatalf("swagger version = %q, want 2.0", doc.Swagger)
			}
			if doc.Info.Title == "" {
				t.Fatal("info.title is required")
			}
			if len(doc.Paths.Paths) == 0 {
				t.Fatal("paths must not be empty")
			}
			if _, ok := doc.SecurityDefinitions["ObserverToken"]; !ok {
				t.Fatal("securityDefinitions.ObserverToken missing")
			}
			if _, ok := doc.SecurityDefinitions["VerifyToken"]; !ok {
				t.Fatal("securityDefinitions.VerifyToken missing")
			}

			for _, route := range tc.requiredRoutes {
				if _, ok := doc.Paths.Paths[route]; !ok {
					t.Fatalf("required path %s missing from %s", route, tc.file)
				}
			}

			raw := loadRawMap(t, filepath.Join(".", tc.file))
			for _, route := range tc.protectedRoutes {
				assertProtectedPost(t, raw, route)
			}
		})
	}

}

func loadSwaggerFile(t *testing.T, path string) *spec.Swagger {
	t.Helper()

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	jsonBody, err := yaml.YAMLToJSON(body)
	if err != nil {
		t.Fatalf("yaml->json %s: %v", path, err)
	}

	var out spec.Swagger
	if err := json.Unmarshal(jsonBody, &out); err != nil {
		t.Fatalf("unmarshal swagger %s: %v", path, err)
	}

	return &out
}

func loadRawMap(t *testing.T, path string) map[string]any {
	t.Helper()

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	jsonBody, err := yaml.YAMLToJSON(body)
	if err != nil {
		t.Fatalf("yaml->json %s: %v", path, err)
	}

	var out map[string]any
	if err := json.Unmarshal(jsonBody, &out); err != nil {
		t.Fatalf("unmarshal raw map %s: %v", path, err)
	}
	return out
}

func assertProtectedPost(t *testing.T, raw map[string]any, route string) {
	t.Helper()

	paths := raw["paths"].(map[string]any)
	pathItem, ok := paths[route].(map[string]any)
	if !ok {
		t.Fatalf("protected path %s missing", route)
	}
	post, ok := pathItem["post"].(map[string]any)
	if !ok {
		t.Fatalf("protected path %s missing post operation", route)
	}
	if _, ok := post["security"]; !ok {
		t.Fatalf("protected path %s missing security stanza", route)
	}
	responses := post["responses"].(map[string]any)
	if _, ok := responses["401"]; !ok {
		t.Fatalf("protected path %s missing 401 response", route)
	}
}
