package service

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	workloadconfigdomain "github.com/bsonger/devflow-service/internal/workloadconfig/domain"
	"github.com/google/uuid"
)

func TestBuildReleaseBundleRendersConfigMapDeploymentServiceAndVirtualService(t *testing.T) {
	releaseID := uuid.New()
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
			Metrics: workloadconfigdomain.WorkloadMetrics{
				Enabled:       true,
				Port:          9090,
				ScrapeProfile: workloadconfigdomain.WorkloadMetricsScrapeProfileFast,
			},
			Labels: map[string]string{
				"team":                        "payments",
				model.ReleaseIDLabel:          "user-overridden-release",
				model.ReleaseApplicationLabel: "user-overridden-app",
			},
			Annotations: map[string]string{
				"example.com/trace": "enabled",
			},
			Resources: workloadconfigdomain.WorkloadResourceRequirements{
				Limits: workloadconfigdomain.WorkloadResourceList{CPU: "500m"},
			},
			Env: []model.EnvVar{{Name: "APP_ENV", Value: "prod"}},
		},
	}
	release := &model.Release{
		BaseModel:     model.BaseModel{ID: releaseID},
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

	bundle, err := buildReleaseBundle("checkout", "demo-api", "production", manifest, release)
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
	deploymentSpec := bundle.Resources.Deployment.Object["spec"].(map[string]any)
	template := deploymentSpec["template"].(map[string]any)
	templateMeta := template["metadata"].(map[string]any)
	templateLabels := templateMeta["labels"].(map[string]any)
	templateAnnotations := templateMeta["annotations"].(map[string]any)
	metadata := bundle.Resources.Deployment.Object["metadata"].(map[string]any)
	workloadLabels := metadata["labels"].(map[string]any)
	containerSpec, ok := template["spec"].(map[string]any)["containers"].([]map[string]any)
	if !ok || len(containerSpec) == 0 {
		t.Fatalf("deployment containers missing: %#v", bundle.Resources.Deployment.Object)
	}
	env := containerSpec[0]["env"].([]map[string]any)
	assertEnvContains(t, env, "APP_ENV", "prod")
	assertEnvContains(t, env, "SERVICE_NAME", "demo-api")
	assertEnvContains(t, env, "OTEL_SERVICE_NAME", "demo-api")
	assertEnvContains(t, env, "OTEL_SERVICE_NAMESPACE", "devflow")
	assertEnvContains(t, env, "DEPLOYMENT_ENVIRONMENT", "production")
	assertEnvContains(t, env, "SERVICE_VERSION", "sha256:abc")
	assertEnvContains(t, env, "METRICS_PORT", "9090")
	for _, labels := range []map[string]any{workloadLabels, templateLabels} {
		assertRequiredIdentityLabels(t, labels, releaseID.String(), manifest.ApplicationID.String(), "production", "demo-api")
		if got := labels["team"]; got != "payments" {
			t.Fatalf("custom team label = %#v", got)
		}
	}
	if got := templateAnnotations["example.com/trace"]; got != "enabled" {
		t.Fatalf("template annotation = %#v", got)
	}
	if got := bundle.Resources.Deployment.Object["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)["serviceAccountName"]; got != "demo-api" {
		t.Fatalf("serviceAccountName = %#v", got)
	}
	if got := bundle.Resources.Deployment.Object["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)["serviceAccount"]; got != "demo-api" {
		t.Fatalf("serviceAccount = %#v", got)
	}
	podSpec := bundle.Resources.Deployment.Object["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)
	if got := podSpec["dnsPolicy"]; got != "ClusterFirst" {
		t.Fatalf("dnsPolicy = %#v", got)
	}
	if got := podSpec["restartPolicy"]; got != "Always" {
		t.Fatalf("restartPolicy = %#v", got)
	}
	if got := podSpec["terminationGracePeriodSeconds"]; got != 30 {
		t.Fatalf("terminationGracePeriodSeconds = %#v", got)
	}
	ports, ok := containerSpec[0]["ports"].([]map[string]any)
	if !ok || len(ports) != 2 {
		t.Fatalf("deployment ports missing: %#v", containerSpec[0])
	}
	if ports[0]["name"] != "http" || ports[0]["containerPort"] != 8080 {
		t.Fatalf("deployment port = %#v", ports[0])
	}
	if ports[1]["name"] != "metrics" || ports[1]["containerPort"] != 9090 {
		t.Fatalf("metrics port = %#v", ports[1])
	}
	if len(bundle.Resources.Services) != 1 {
		t.Fatalf("services len = %d, want 1", len(bundle.Resources.Services))
	}
	serviceMetadata := bundle.Resources.Services[0].Object["metadata"].(map[string]any)
	serviceLabels := serviceMetadata["labels"].(map[string]any)
	if got := serviceLabels[releaseScrapeLabel]; got != "true" {
		t.Fatalf("service scrape label = %#v", got)
	}
	if got := serviceLabels[releaseScrapeProfileLabel]; got != "fast" {
		t.Fatalf("service scrape profile label = %#v", got)
	}
	servicePorts := bundle.Resources.Services[0].Object["spec"].(map[string]any)["ports"].([]map[string]any)
	if len(servicePorts) != 2 {
		t.Fatalf("service ports = %#v", servicePorts)
	}
	if servicePorts[1]["name"] != "metrics" || servicePorts[1]["port"] != 9090 || servicePorts[1]["targetPort"] != 9090 {
		t.Fatalf("service metrics port = %#v", servicePorts[1])
	}
	if len(bundle.Files) < 2 {
		t.Fatalf("expected bundle files, got %d", len(bundle.Files))
	}
	lastFile := bundle.Files[len(bundle.Files)-1]
	if !strings.Contains(lastFile.Content, model.ReleaseIDLabel+": "+releaseID.String()) {
		t.Fatalf("bundle.yaml missing release-id label: %s", lastFile.Content)
	}
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

	bundle, err := buildReleaseBundle("", "", "staging", manifest, release)
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

	bundle, err := buildReleaseBundle("checkout", "demo-api", "production", manifest, release)
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

func TestBuildReleaseBundleStripsDriftProneAnnotations(t *testing.T) {
	releaseID := uuid.New()
	manifest := &manifestdomain.Manifest{
		BaseModel:     model.BaseModel{ID: uuid.New()},
		ApplicationID: uuid.New(),
		ImageRef:      "registry.example.com/devflow/demo-api@sha256:abc",
		ServicesSnapshot: []manifestdomain.ManifestService{{
			Name: "demo-api",
		}},
		WorkloadConfigSnapshot: manifestdomain.ManifestWorkloadConfig{
			Replicas: 1,
			Annotations: map[string]string{
				"kubectl.kubernetes.io/restartedAt": "2026-05-01T02:03:04Z",
				"example.com/trace":                 "enabled",
			},
		},
	}
	release := &model.Release{
		BaseModel:     model.BaseModel{ID: releaseID},
		ApplicationID: manifest.ApplicationID,
		EnvironmentID: "production",
	}

	bundle, err := buildReleaseBundle("checkout", "demo-api", "production", manifest, release)
	if err != nil {
		t.Fatalf("buildReleaseBundle failed: %v", err)
	}
	metadata := bundle.Resources.Deployment.Object["metadata"].(map[string]any)
	if annotations, ok := metadata["annotations"]; ok {
		annotationMap := annotations.(map[string]any)
		if _, exists := annotationMap["kubectl.kubernetes.io/restartedAt"]; exists {
			t.Fatalf("workload metadata should not include drift-prone annotation: %#v", annotationMap)
		}
	}
	templateAnnotations := bundle.Resources.Deployment.Object["spec"].(map[string]any)["template"].(map[string]any)["metadata"].(map[string]any)["annotations"].(map[string]any)
	if _, exists := templateAnnotations["kubectl.kubernetes.io/restartedAt"]; exists {
		t.Fatalf("template annotations should strip drift-prone annotation: %#v", templateAnnotations)
	}
	if got := templateAnnotations["example.com/trace"]; got != "enabled" {
		t.Fatalf("expected supplementary annotation to survive, got %#v", got)
	}
	if strings.Contains(bundle.Files[len(bundle.Files)-1].Content, "kubectl.kubernetes.io/restartedAt") {
		t.Fatalf("bundle.yaml should not contain drift-prone annotation: %s", bundle.Files[len(bundle.Files)-1].Content)
	}
}

func TestBuildReleaseBundleUsesFrozenManifestSnapshotWithoutLiveWorkloadReads(t *testing.T) {
	manifest := &manifestdomain.Manifest{
		BaseModel:     model.BaseModel{ID: uuid.New()},
		ApplicationID: uuid.New(),
		ImageRef:      "registry.example.com/devflow/demo-api@sha256:abc",
		ServicesSnapshot: []manifestdomain.ManifestService{{
			Name:  "demo-api",
			Ports: []manifestdomain.ManifestServicePort{{Name: "http", ServicePort: 80, TargetPort: 8080, Protocol: "TCP"}},
		}},
		WorkloadConfigSnapshot: manifestdomain.ManifestWorkloadConfig{
			Replicas: 4,
			Resources: workloadconfigdomain.WorkloadResourceRequirements{
				SizeClass: workloadconfigdomain.WorkloadSizeClassLarge,
			},
			Metrics: workloadconfigdomain.WorkloadMetrics{
				Enabled: true,
				Port:    9100,
			},
			Env: []model.EnvVar{{Name: "SNAPSHOT_ONLY", Value: "true"}},
		},
	}
	release := &model.Release{
		BaseModel:     model.BaseModel{ID: uuid.New()},
		ApplicationID: manifest.ApplicationID,
		EnvironmentID: "staging",
	}

	bundle, err := buildReleaseBundle("checkout", "demo-api", "staging", manifest, release)
	if err != nil {
		t.Fatalf("buildReleaseBundle failed: %v", err)
	}
	container := bundle.Resources.Deployment.Object["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)["containers"].([]map[string]any)[0]
	resources := container["resources"].(map[string]any)
	limits := resources["limits"].(map[string]any)
	requests := resources["requests"].(map[string]any)
	if got := bundle.Resources.Deployment.Object["spec"].(map[string]any)["replicas"]; got != 4 {
		t.Fatalf("deployment replicas = %#v", got)
	}
	if got := requests["cpu"]; got != "500m" {
		t.Fatalf("requests cpu = %#v", got)
	}
	if got := requests["memory"]; got != "512Mi" {
		t.Fatalf("requests memory = %#v", got)
	}
	if got := limits["cpu"]; got != "2" {
		t.Fatalf("limits cpu = %#v", got)
	}
	if got := limits["memory"]; got != "2Gi" {
		t.Fatalf("limits memory = %#v", got)
	}
	env := container["env"].([]map[string]any)
	assertEnvContains(t, env, "SNAPSHOT_ONLY", "true")
	assertEnvContains(t, env, "SERVICE_NAME", "demo-api")
	assertEnvContains(t, env, "DEPLOYMENT_ENVIRONMENT", "staging")
	assertEnvContains(t, env, "SERVICE_VERSION", "sha256:abc")
	assertEnvContains(t, env, "METRICS_PORT", "9100")
	if !strings.Contains(bundle.Files[len(bundle.Files)-1].Content, "SNAPSHOT_ONLY") {
		t.Fatalf("bundle.yaml missing frozen env entry: %s", bundle.Files[len(bundle.Files)-1].Content)
	}
}

func TestBuildReleaseBundleWorkloadEnvPreservesExplicitOTELOverrides(t *testing.T) {
	manifest := &manifestdomain.Manifest{
		BaseModel:     model.BaseModel{ID: uuid.New()},
		ApplicationID: uuid.New(),
		ImageRef:      "registry.example.com/devflow/demo-api:latest",
		CommitHash:    "deadbeef",
		WorkloadConfigSnapshot: manifestdomain.ManifestWorkloadConfig{
			Replicas: 1,
			Env: []model.EnvVar{
				{Name: "OTEL_SERVICE_NAMESPACE", Value: "custom-ns"},
				{Name: "SERVICE_VERSION", Value: "custom-version"},
				{Name: "METRICS_PORT", Value: "19090"},
			},
			Metrics: workloadconfigdomain.WorkloadMetrics{
				Enabled: true,
				Port:    9090,
			},
		},
	}
	release := &model.Release{
		BaseModel:     model.BaseModel{ID: uuid.New()},
		ApplicationID: manifest.ApplicationID,
		EnvironmentID: "staging",
	}

	bundle, err := buildReleaseBundle("checkout", "demo-api", "staging", manifest, release)
	if err != nil {
		t.Fatalf("buildReleaseBundle failed: %v", err)
	}
	container := bundle.Resources.Deployment.Object["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)["containers"].([]map[string]any)[0]
	env := container["env"].([]map[string]any)

	assertEnvContains(t, env, "OTEL_SERVICE_NAMESPACE", "custom-ns")
	assertEnvContains(t, env, "SERVICE_VERSION", "custom-version")
	assertEnvContains(t, env, "OTEL_SERVICE_NAME", "demo-api")
	assertEnvContains(t, env, "DEPLOYMENT_ENVIRONMENT", "staging")
	assertEnvContains(t, env, "METRICS_PORT", "19090")
}

func TestBuildReleaseBundleUsesEnvironmentNameForDeploymentEnvironment(t *testing.T) {
	manifest := &manifestdomain.Manifest{
		BaseModel:     model.BaseModel{ID: uuid.New()},
		ApplicationID: uuid.New(),
		ImageRef:      "registry.example.com/devflow/demo-api@sha256:abc",
		WorkloadConfigSnapshot: manifestdomain.ManifestWorkloadConfig{
			Replicas: 1,
		},
	}
	release := &model.Release{
		BaseModel:     model.BaseModel{ID: uuid.New()},
		ApplicationID: manifest.ApplicationID,
		EnvironmentID: "b780ca97-a213-4763-bfb9-43f7e3a11ee7",
	}

	bundle, err := buildReleaseBundle("checkout", "demo-api", "pre-production", manifest, release)
	if err != nil {
		t.Fatalf("buildReleaseBundle failed: %v", err)
	}
	container := bundle.Resources.Deployment.Object["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)["containers"].([]map[string]any)[0]
	env := container["env"].([]map[string]any)

	assertEnvContains(t, env, "DEPLOYMENT_ENVIRONMENT", "pre-production")
}

func TestBuildReleaseBundleKeepsRequiredIdentityLabels(t *testing.T) {
	releaseID := uuid.New()
	manifest := &manifestdomain.Manifest{
		BaseModel:     model.BaseModel{ID: uuid.New()},
		ApplicationID: uuid.New(),
		ImageRef:      "registry.example.com/devflow/demo-api@sha256:abc",
		ServicesSnapshot: []manifestdomain.ManifestService{{
			Name: "demo-api",
		}},
		WorkloadConfigSnapshot: manifestdomain.ManifestWorkloadConfig{
			Replicas: 1,
			Labels: map[string]string{
				"app.kubernetes.io/name":      "overridden",
				model.ReleaseIDLabel:          "wrong-release",
				model.ReleaseApplicationLabel: "wrong-app",
				model.ReleaseEnvironmentLabel: "wrong-env",
				"custom.io/owner":             "platform",
			},
		},
	}
	release := &model.Release{
		BaseModel:     model.BaseModel{ID: releaseID},
		ApplicationID: manifest.ApplicationID,
		EnvironmentID: "production",
		Strategy:      string(model.ReleaseStrategyCanary),
	}

	bundle, err := buildReleaseBundle("checkout", "demo-api", "production", manifest, release)
	if err != nil {
		t.Fatalf("buildReleaseBundle failed: %v", err)
	}
	if bundle.Resources.Rollout == nil {
		t.Fatal("expected rollout")
	}
	metadata := bundle.Resources.Rollout.Object["metadata"].(map[string]any)
	workloadLabels := metadata["labels"].(map[string]any)
	templateLabels := bundle.Resources.Rollout.Object["spec"].(map[string]any)["template"].(map[string]any)["metadata"].(map[string]any)["labels"].(map[string]any)
	for _, labels := range []map[string]any{workloadLabels, templateLabels} {
		assertRequiredIdentityLabels(t, labels, releaseID.String(), manifest.ApplicationID.String(), "production", "demo-api")
		if got := labels["custom.io/owner"]; got != "platform" {
			t.Fatalf("expected custom label to survive, got %#v", got)
		}
	}
}

func assertRequiredIdentityLabels(t *testing.T, labels map[string]any, releaseID, applicationID, environmentID, name string) {
	t.Helper()
	if got := labels[model.ReleaseIDLabel]; got != releaseID {
		t.Fatalf("release-id label = %#v", got)
	}
	if got := labels[model.ReleaseApplicationLabel]; got != applicationID {
		t.Fatalf("application label = %#v", got)
	}
	if got := labels[model.ReleaseEnvironmentLabel]; got != environmentID {
		t.Fatalf("environment label = %#v", got)
	}
	if got := labels["app.kubernetes.io/name"]; got != name {
		t.Fatalf("app label = %#v", got)
	}
}

func assertEnvContains(t *testing.T, env []map[string]any, name, want string) {
	t.Helper()
	for _, entry := range env {
		if entry["name"] == name {
			if entry["value"] != want {
				t.Fatalf("%s value = %#v want %q", name, entry["value"], want)
			}
			return
		}
	}
	t.Fatalf("env missing %s: %#v", name, env)
}
