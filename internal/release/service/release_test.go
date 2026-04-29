package service

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	store "github.com/bsonger/devflow-service/internal/platform/db"
	"github.com/bsonger/devflow-service/internal/platform/dbsql"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	releasesupport "github.com/bsonger/devflow-service/internal/release/support"
	sharederrs "github.com/bsonger/devflow-service/internal/shared/errs"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

func marshalJSON(value any, empty string) ([]byte, error) {
	return dbsql.MarshalJSON(value, empty)
}

func setupTestDB(t *testing.T) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	createTable := `
CREATE TABLE releases (
  id TEXT PRIMARY KEY,
  execution_intent_id TEXT NULL,
  application_id TEXT NOT NULL,
  manifest_id TEXT NOT NULL,
  env TEXT NOT NULL,
  strategy TEXT NOT NULL DEFAULT 'rolling',
  routes_snapshot TEXT NOT NULL DEFAULT '[]',
  app_config_snapshot TEXT NOT NULL DEFAULT '{}',
  artifact_repository TEXT NOT NULL DEFAULT '',
  artifact_tag TEXT NOT NULL DEFAULT '',
  artifact_digest TEXT NOT NULL DEFAULT '',
  artifact_ref TEXT NOT NULL DEFAULT '',
  type TEXT NOT NULL,
  steps TEXT NOT NULL DEFAULT '[]',
  status TEXT NOT NULL,
  argocd_application_name TEXT NOT NULL DEFAULT '',
  external_ref TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  deleted_at DATETIME NULL
);

CREATE TABLE release_bundles (
  id TEXT PRIMARY KEY,
  release_id TEXT NOT NULL,
  namespace TEXT NOT NULL,
  artifact_name TEXT NOT NULL,
  bundle_digest TEXT NOT NULL,
  rendered_objects TEXT NOT NULL DEFAULT '[]',
  bundle_yaml TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  deleted_at DATETIME NULL
);
	`
	if _, err := db.Exec(createTable); err != nil {
		t.Fatalf("create table: %v", err)
	}
	store.InitPostgres(db)
	t.Cleanup(func() {
		_ = db.Close()
		store.InitPostgres(nil)
	})
}

func TestUpdateStatusRespectsTerminalGuard(t *testing.T) {
	setupTestDB(t)
	releaseID := uuid.New()
	appID := uuid.New()
	manifestID := uuid.New()

	stepsJSON, _ := marshalJSON(model.DefaultReleaseSteps(model.Normal, model.ReleaseUpgrade), "[]")
	_, err := store.DB().ExecContext(context.Background(), `
		insert into releases (id, application_id, manifest_id, env, type, steps, status, created_at, updated_at, deleted_at)
		values ($1,$2,$3,'staging','Upgrade',$4,'Succeeded',$5,$6,null)
	`, releaseID.String(), appID.String(), manifestID.String(), stepsJSON, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	svc := &releaseService{}
	err = svc.updateStatus(context.Background(), releaseID, model.ReleaseFailed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	release, err := svc.Get(context.Background(), releaseID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if release.Status != model.ReleaseSucceeded {
		t.Fatalf("terminal status was overwritten: got %q want Succeeded", release.Status)
	}
}

func TestReleaseRepositoryPersistsArtifactFields(t *testing.T) {
	setupTestDB(t)
	releaseID := uuid.New()
	appID := uuid.New()
	manifestID := uuid.New()

	stepsJSON, _ := marshalJSON(model.DefaultReleaseSteps(model.Normal, model.ReleaseUpgrade), "[]")
	_, err := store.DB().ExecContext(context.Background(), `
		insert into releases (
			id, application_id, manifest_id, env, strategy, routes_snapshot, app_config_snapshot,
			artifact_repository, artifact_tag, artifact_digest, artifact_ref,
			type, steps, status, created_at, updated_at, deleted_at
		)
		values ($1,$2,$3,'staging','rolling','[]','{}',$4,$5,$6,$7,'Upgrade',$8,'Pending',$9,$10,null)
	`, releaseID.String(), appID.String(), manifestID.String(),
		"registry.example.com/devflow/releases/demo-api",
		"release-20260428",
		"sha256:abc",
		"registry.example.com/devflow/releases/demo-api@sha256:abc",
		stepsJSON, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	svc := &releaseService{}
	release, err := svc.Get(context.Background(), releaseID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if release.ArtifactRepository != "registry.example.com/devflow/releases/demo-api" {
		t.Fatalf("artifact_repository = %q", release.ArtifactRepository)
	}
	if release.ArtifactTag != "release-20260428" {
		t.Fatalf("artifact_tag = %q", release.ArtifactTag)
	}
	if release.ArtifactDigest != "sha256:abc" {
		t.Fatalf("artifact_digest = %q", release.ArtifactDigest)
	}
	if release.ArtifactRef != "registry.example.com/devflow/releases/demo-api@sha256:abc" {
		t.Fatalf("artifact_ref = %q", release.ArtifactRef)
	}
}

func TestReleaseRepositoryPersistsArgoCDApplicationName(t *testing.T) {
	setupTestDB(t)
	releaseID := uuid.New()
	appID := uuid.New()
	manifestID := uuid.New()

	stepsJSON, _ := marshalJSON(model.DefaultReleaseSteps(model.Normal, model.ReleaseUpgrade), "[]")
	_, err := store.DB().ExecContext(context.Background(), `
		insert into releases (
			id, application_id, manifest_id, env, strategy, routes_snapshot, app_config_snapshot,
			artifact_repository, artifact_tag, artifact_digest, artifact_ref,
			type, steps, status, argocd_application_name, external_ref, created_at, updated_at, deleted_at
		)
		values ($1,$2,$3,'staging','rolling','[]','{}','','','','','Upgrade',$4,'Pending',$5,$6,$7,$8,null)
	`, releaseID.String(), appID.String(), manifestID.String(),
		stepsJSON,
		"demo-api",
		"demo-api",
		time.Now(),
		time.Now())
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	svc := &releaseService{}
	release, err := svc.Get(context.Background(), releaseID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if release.ArgoCDApplicationName != "demo-api" {
		t.Fatalf("argocd_application_name = %q", release.ArgoCDApplicationName)
	}
	if release.ExternalRef != "demo-api" {
		t.Fatalf("external_ref = %q", release.ExternalRef)
	}
}

func TestUpdateStatusAllowsNonTerminalTransition(t *testing.T) {
	setupTestDB(t)
	releaseID := uuid.New()
	appID := uuid.New()
	manifestID := uuid.New()

	stepsJSON, _ := marshalJSON(model.DefaultReleaseSteps(model.Normal, model.ReleaseUpgrade), "[]")
	_, err := store.DB().ExecContext(context.Background(), `
		insert into releases (id, application_id, manifest_id, env, type, steps, status, created_at, updated_at, deleted_at)
		values ($1,$2,$3,'staging','Upgrade',$4,'Syncing',$5,$6,null)
	`, releaseID.String(), appID.String(), manifestID.String(), stepsJSON, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	svc := &releaseService{}
	err = svc.updateStatus(context.Background(), releaseID, model.ReleaseRunning)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	release, err := svc.Get(context.Background(), releaseID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if release.Status != model.ReleaseRunning {
		t.Fatalf("status not updated: got %q want Running", release.Status)
	}
}

func TestUpdateArtifactPersistsFieldsAndMarksPublishBundle(t *testing.T) {
	setupTestDB(t)
	releaseID := uuid.New()
	appID := uuid.New()
	manifestID := uuid.New()

	steps := model.DefaultReleaseSteps(model.Normal, model.ReleaseUpgrade)
	stepsJSON, _ := marshalJSON(steps, "[]")
	_, err := store.DB().ExecContext(context.Background(), `
		insert into releases (id, application_id, manifest_id, env, strategy, type, steps, status, created_at, updated_at, deleted_at)
		values ($1,$2,$3,'staging','rolling','Upgrade',$4,'Running',$5,$6,null)
	`, releaseID.String(), appID.String(), manifestID.String(), stepsJSON, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	svc := &releaseService{}
	err = svc.UpdateArtifact(context.Background(), releaseID,
		"registry.example.com/devflow/releases/demo-api",
		"release-20260428",
		"sha256:abc",
		"oci://registry.example.com/devflow/releases/demo-api:release-20260428",
		"deployment bundle published via oras publisher: oci://registry.example.com/devflow/releases/demo-api:release-20260428",
		model.StepSucceeded,
		100,
	)
	if err != nil {
		t.Fatalf("UpdateArtifact failed: %v", err)
	}

	release, err := svc.Get(context.Background(), releaseID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if release.ArtifactRepository != "registry.example.com/devflow/releases/demo-api" {
		t.Fatalf("artifact_repository = %q", release.ArtifactRepository)
	}
	if release.ArtifactTag != "release-20260428" {
		t.Fatalf("artifact_tag = %q", release.ArtifactTag)
	}
	if release.ArtifactDigest != "sha256:abc" {
		t.Fatalf("artifact_digest = %q", release.ArtifactDigest)
	}
	if release.ArtifactRef != "oci://registry.example.com/devflow/releases/demo-api:release-20260428" {
		t.Fatalf("artifact_ref = %q", release.ArtifactRef)
	}
	found := false
	for _, step := range release.Steps {
		if step.Code == "publish_bundle" {
			found = true
			if step.Status != model.StepSucceeded || step.Progress != 100 || step.Message != "deployment bundle published via oras publisher: oci://registry.example.com/devflow/releases/demo-api:release-20260428" {
				t.Fatalf("unexpected publish_bundle step: %+v", step)
			}
		}
	}
	if !found {
		t.Fatal("publish_bundle step not found")
	}
}

func TestRenderDeploymentBundlePersistsBundleRecord(t *testing.T) {
	setupTestDB(t)
	releaseID := uuid.New()
	appID := uuid.New()
	manifestID := uuid.New()
	steps := model.DefaultReleaseSteps(model.Normal, model.ReleaseUpgrade)
	appConfigJSON, _ := marshalJSON(model.ReleaseAppConfig{
		MountPath: "/etc/config",
		Data: map[string]string{
			"LOG_LEVEL": "info",
		},
	}, "{}")
	stepsJSON, _ := marshalJSON(steps, "[]")
	_, err := store.DB().ExecContext(context.Background(), `
		insert into releases (id, application_id, manifest_id, env, strategy, app_config_snapshot, type, steps, status, created_at, updated_at, deleted_at)
		values ($1,$2,$3,'staging','rolling',$4,'Upgrade',$5,'Running',$6,$7,null)
	`, releaseID.String(), appID.String(), manifestID.String(), appConfigJSON, stepsJSON, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	svc := &releaseService{}
	release, err := svc.Get(context.Background(), releaseID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	manifest := &manifestdomain.Manifest{
		BaseModel:     model.BaseModel{ID: manifestID},
		ApplicationID: appID,
		ImageRef:      "registry.example.com/devflow/demo-api@sha256:abc",
		ServicesSnapshot: []manifestdomain.ManifestService{
			{Name: "demo-api", Ports: []manifestdomain.ManifestServicePort{{Name: "http", ServicePort: 80, TargetPort: 8080, Protocol: "TCP"}}},
		},
		WorkloadConfigSnapshot: manifestdomain.ManifestWorkloadConfig{Replicas: 1},
	}
	if err := svc.renderDeploymentBundle(context.Background(), release, manifest, &releasesupport.ApplicationProjection{Name: "demo-api"}, releasesupport.DeployTarget{Namespace: "checkout"}); err != nil {
		t.Fatalf("renderDeploymentBundle failed: %v", err)
	}

	bundle, err := svc.repoBundleStore().GetByReleaseID(context.Background(), releaseID)
	if err != nil {
		t.Fatalf("get bundle failed: %v", err)
	}
	if bundle.Namespace != "checkout" {
		t.Fatalf("bundle namespace = %q", bundle.Namespace)
	}
	if bundle.ArtifactName != "demo-api" {
		t.Fatalf("bundle artifact_name = %q", bundle.ArtifactName)
	}
	if bundle.BundleDigest == "" {
		t.Fatal("expected bundle digest to be set")
	}
	if !strings.Contains(bundle.BundleYAML, "kind: Deployment") {
		t.Fatalf("bundle_yaml = %q", bundle.BundleYAML)
	}

	updated, err := svc.Get(context.Background(), releaseID)
	if err != nil {
		t.Fatalf("get updated failed: %v", err)
	}
	found := false
	for _, step := range updated.Steps {
		if step.Code == "render_deployment_bundle" {
			found = true
			if step.Status != model.StepSucceeded {
				t.Fatalf("render_deployment_bundle status = %q", step.Status)
			}
		}
	}
	if !found {
		t.Fatal("render_deployment_bundle step not found")
	}
}

func TestGetBundlePreviewReadsPersistedBundleRecord(t *testing.T) {
	setupTestDB(t)
	releaseID := uuid.New()
	appID := uuid.New()
	manifestID := uuid.New()
	now := time.Now()
	steps := model.DefaultReleaseSteps(model.Canary, model.ReleaseUpgrade)
	for i := range steps {
		switch steps[i].Code {
		case "render_deployment_bundle":
			steps[i].Status = model.StepSucceeded
			steps[i].EndTime = &now
		case "publish_bundle":
			steps[i].Status = model.StepSucceeded
			steps[i].EndTime = &now
		}
	}
	stepsJSON, _ := marshalJSON(steps, "[]")
	appConfigJSON, _ := marshalJSON(model.ReleaseAppConfig{
		MountPath: "/etc/config",
		Data: map[string]string{
			"LOG_LEVEL": "info",
		},
		Files: []model.ReleaseFile{{Name: "app.yaml", Content: "log_level: info"}},
	}, "{}")
	routesJSON, _ := marshalJSON([]model.ReleaseRoute{{
		Name:        "api",
		Host:        "demo.example.com",
		Path:        "/",
		ServiceName: "demo-api",
		ServicePort: 80,
	}}, "[]")
	_, err := store.DB().ExecContext(context.Background(), `
		insert into releases (
			id, application_id, manifest_id, env, strategy, routes_snapshot, app_config_snapshot,
			artifact_repository, artifact_tag, artifact_digest, artifact_ref,
			type, steps, status, created_at, updated_at, deleted_at
		)
		values ($1,$2,$3,'staging','canary',$4,$5,$6,$7,$8,$9,'Upgrade',$10,'Running',$11,$12,null)
	`, releaseID.String(), appID.String(), manifestID.String(), routesJSON, appConfigJSON,
		"registry.example.com/devflow/releases/demo-api",
		releaseID.String(),
		"sha256:artifact",
		"oci://registry.example.com/devflow/releases/demo-api@sha256:artifact",
		stepsJSON, now, now)
	if err != nil {
		t.Fatalf("insert release failed: %v", err)
	}

	svc := &releaseService{}
	bundle := newReleaseBundleRecord(&model.ReleaseBundle{
		ReleaseID:     releaseID,
		ApplicationID: appID,
		EnvironmentID: "staging",
		Namespace:     "persisted-checkout",
		ArtifactName:  "demo-api",
		RenderedObjects: []model.ReleaseRenderedResource{
			{
				Kind:      "Service",
				Name:      "demo-api",
				Namespace: "persisted-checkout",
				YAML:      "apiVersion: v1\nkind: Service\nmetadata:\n  name: demo-api\n",
				Object: map[string]any{
					"spec": map[string]any{
						"selector": map[string]any{"app.kubernetes.io/name": "demo-api"},
						"ports": []map[string]any{{
							"name":       "http",
							"port":       80,
							"targetPort": 8080,
							"protocol":   "TCP",
						}},
					},
				},
			},
			{
				Kind:      "Rollout",
				Name:      "demo-api",
				Namespace: "persisted-checkout",
				YAML:      "apiVersion: argoproj.io/v1alpha1\nkind: Rollout\nmetadata:\n  name: demo-api\n",
				Object: map[string]any{
					"spec": map[string]any{
						"replicas": 2,
						"strategy": map[string]any{
							"canary": map[string]any{
								"stableService": "demo-api",
								"canaryService": "demo-api-canary",
								"steps": []map[string]any{
									{"setWeight": 10},
									{"setWeight": 30},
								},
							},
						},
						"template": map[string]any{
							"spec": map[string]any{
								"serviceAccountName": "demo-api",
								"containers": []map[string]any{{
									"image": "registry.example.com/demo-api:abc",
									"env": []map[string]any{
										{"name": "LOG_LEVEL", "value": "info"},
									},
									"resources":      map[string]any{"limits": map[string]any{"cpu": "500m"}},
									"readinessProbe": map[string]any{"httpGet": map[string]any{"path": "/healthz"}},
									"volumeMounts": []map[string]any{{
										"name":      "app-config",
										"mountPath": "/etc/config",
									}},
								}},
							},
						},
					},
				},
			},
		},
	})
	if err := svc.repoBundleStore().Insert(context.Background(), bundle); err != nil {
		t.Fatalf("insert bundle failed: %v", err)
	}

	originalManifestSource := releaseManifestSource
	releaseManifestSource = stubReleaseManifestReader{
		getFn: func(_ context.Context, id uuid.UUID) (*manifestdomain.Manifest, error) {
			if id != manifestID {
				t.Fatalf("manifest id = %s want %s", id, manifestID)
			}
			return &manifestdomain.Manifest{
				BaseModel:     model.BaseModel{ID: manifestID},
				ApplicationID: appID,
				CommitHash:    "abc123",
				ImageRef:      "registry.example.com/demo-api:abc123",
				ImageDigest:   "sha256:image",
				ServicesSnapshot: []manifestdomain.ManifestService{
					{Name: "demo-api", Ports: []manifestdomain.ManifestServicePort{{Name: "http", ServicePort: 80, TargetPort: 8080, Protocol: "TCP"}}},
				},
				WorkloadConfigSnapshot: manifestdomain.ManifestWorkloadConfig{
					Replicas: 2,
					Env:      []model.EnvVar{{Name: "LOG_LEVEL", Value: "info"}},
				},
			}, nil
		},
	}
	defer func() { releaseManifestSource = originalManifestSource }()

	preview, err := svc.GetBundlePreview(context.Background(), releaseID)
	if err != nil {
		t.Fatalf("GetBundlePreview failed: %v", err)
	}
	if preview.Namespace != "persisted-checkout" {
		t.Fatalf("preview namespace = %q", preview.Namespace)
	}
	if preview.BundleDigest != bundle.BundleDigest {
		t.Fatalf("preview bundle_digest = %q want %q", preview.BundleDigest, bundle.BundleDigest)
	}
	if preview.FrozenInputs.AppConfig.MountPath != "/etc/config" {
		t.Fatalf("app_config mount_path = %q", preview.FrozenInputs.AppConfig.MountPath)
	}
	if len(preview.FrozenInputs.Services) != 1 {
		t.Fatalf("services = %#v", preview.FrozenInputs.Services)
	}
	if preview.Artifact == nil || preview.Artifact.Ref == "" {
		t.Fatalf("artifact = %#v", preview.Artifact)
	}
	if preview.PublishedAt == nil {
		t.Fatal("expected published_at to be derived from steps")
	}
	if len(preview.RenderedBundle.ResourceGroups) != 2 {
		t.Fatalf("resource_groups = %#v", preview.RenderedBundle.ResourceGroups)
	}
	if len(preview.RenderedBundle.RenderedResources) != 2 {
		t.Fatalf("rendered_resources = %#v", preview.RenderedBundle.RenderedResources)
	}
	if len(preview.RenderedBundle.Files) != 3 {
		t.Fatalf("files = %#v", preview.RenderedBundle.Files)
	}
}

func TestGetReleaseAttachesBundleSummary(t *testing.T) {
	setupTestDB(t)
	releaseID := uuid.New()
	appID := uuid.New()
	manifestID := uuid.New()
	now := time.Now()
	steps := model.DefaultReleaseSteps(model.Canary, model.ReleaseUpgrade)
	for i := range steps {
		switch steps[i].Code {
		case "render_deployment_bundle":
			steps[i].Status = model.StepSucceeded
			steps[i].EndTime = &now
		case "publish_bundle":
			steps[i].Status = model.StepSucceeded
			steps[i].EndTime = &now
		}
	}
	stepsJSON, _ := marshalJSON(steps, "[]")
	_, err := store.DB().ExecContext(context.Background(), `
		insert into releases (
			id, application_id, manifest_id, env, strategy,
			artifact_repository, artifact_tag, artifact_digest, artifact_ref,
			type, steps, status, created_at, updated_at, deleted_at
		)
		values ($1,$2,$3,'staging','canary',$4,$5,$6,$7,'Upgrade',$8,'Running',$9,$10,null)
	`, releaseID.String(), appID.String(), manifestID.String(),
		"registry.example.com/devflow/releases/demo-api",
		releaseID.String(),
		"sha256:artifact",
		"oci://registry.example.com/devflow/releases/demo-api@sha256:artifact",
		stepsJSON, now, now)
	if err != nil {
		t.Fatalf("insert release failed: %v", err)
	}

	bundle := newReleaseBundleRecord(&model.ReleaseBundle{
		ReleaseID:     releaseID,
		ApplicationID: appID,
		EnvironmentID: "staging",
		Namespace:     "checkout",
		ArtifactName:  "demo-api",
		RenderedObjects: []model.ReleaseRenderedResource{
			{Kind: "ConfigMap", Name: "demo-api", Namespace: "checkout", YAML: "kind: ConfigMap\n"},
			{Kind: "Service", Name: "demo-api", Namespace: "checkout", YAML: "kind: Service\n"},
			{Kind: "Service", Name: "demo-api-canary", Namespace: "checkout", YAML: "kind: Service\n"},
			{Kind: "Rollout", Name: "demo-api", Namespace: "checkout", YAML: "kind: Rollout\n"},
			{Kind: "VirtualService", Name: "demo-api", Namespace: "checkout", YAML: "kind: VirtualService\n"},
		},
	})

	svc := &releaseService{}
	if err := svc.repoBundleStore().Insert(context.Background(), bundle); err != nil {
		t.Fatalf("insert bundle failed: %v", err)
	}

	release, err := svc.Get(context.Background(), releaseID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if release.BundleSummary == nil {
		t.Fatal("expected bundle_summary")
	}
	if !release.BundleSummary.Available {
		t.Fatal("expected bundle_summary.available to be true")
	}
	if release.BundleSummary.Namespace != "checkout" {
		t.Fatalf("bundle_summary.namespace = %q", release.BundleSummary.Namespace)
	}
	if release.BundleSummary.BundleDigest != bundle.BundleDigest {
		t.Fatalf("bundle_summary.bundle_digest = %q want %q", release.BundleSummary.BundleDigest, bundle.BundleDigest)
	}
	if release.BundleSummary.PrimaryWorkloadKind != "Rollout" {
		t.Fatalf("bundle_summary.primary_workload_kind = %q", release.BundleSummary.PrimaryWorkloadKind)
	}
	if release.BundleSummary.ResourceCounts.Total != 5 {
		t.Fatalf("resource_counts.total = %d", release.BundleSummary.ResourceCounts.Total)
	}
	if release.BundleSummary.ResourceCounts.Services != 2 {
		t.Fatalf("resource_counts.services = %d", release.BundleSummary.ResourceCounts.Services)
	}
	if release.BundleSummary.Artifact == nil || release.BundleSummary.Artifact.Ref == "" {
		t.Fatalf("bundle_summary.artifact = %#v", release.BundleSummary.Artifact)
	}
	if release.BundleSummary.RenderedAt == nil {
		t.Fatal("expected bundle_summary.rendered_at")
	}
	if release.BundleSummary.PublishedAt == nil {
		t.Fatal("expected bundle_summary.published_at")
	}
}

func TestUpdateStepAppendsOrphanStep(t *testing.T) {
	setupTestDB(t)
	releaseID := uuid.New()
	appID := uuid.New()
	manifestID := uuid.New()

	steps := model.DefaultReleaseSteps(model.Normal, model.ReleaseUpgrade)
	stepsJSON, _ := marshalJSON(steps, "[]")
	_, err := store.DB().ExecContext(context.Background(), `
		insert into releases (id, application_id, manifest_id, env, type, steps, status, created_at, updated_at, deleted_at)
		values ($1,$2,$3,'staging','Upgrade',$4,'Syncing',$5,$6,null)
	`, releaseID.String(), appID.String(), manifestID.String(), stepsJSON, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	svc := &releaseService{}
	err = svc.UpdateStep(context.Background(), releaseID, "orphan step", model.StepRunning, 25, "working", nil, nil)
	if err == nil {
		t.Fatal("expected unknown release step error")
	}
	if !sharederrs.HasCode(err, sharederrs.CodeInvalidArgument) {
		t.Fatalf("unexpected error code: %v", err)
	}
}

func TestUpdateStepUpdatesExistingStep(t *testing.T) {
	setupTestDB(t)
	releaseID := uuid.New()
	appID := uuid.New()
	manifestID := uuid.New()

	steps := model.DefaultReleaseSteps(model.Normal, model.ReleaseUpgrade)
	stepsJSON, _ := marshalJSON(steps, "[]")
	_, err := store.DB().ExecContext(context.Background(), `
		insert into releases (id, application_id, manifest_id, env, type, steps, status, created_at, updated_at, deleted_at)
		values ($1,$2,$3,'staging','Upgrade',$4,'Syncing',$5,$6,null)
	`, releaseID.String(), appID.String(), manifestID.String(), stepsJSON, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	svc := &releaseService{}
	err = svc.UpdateStep(context.Background(), releaseID, "render_deployment_bundle", model.StepSucceeded, 100, "done", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	release, err := svc.Get(context.Background(), releaseID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	found := false
	for _, s := range release.Steps {
		if s.Code == "render_deployment_bundle" {
			found = true
			if s.Status != model.StepSucceeded || s.Progress != 100 {
				t.Fatalf("unexpected step state: %+v", s)
			}
		}
	}
	if !found {
		t.Fatal("existing step not updated")
	}
}

func TestUpdateStepReturnsNotFoundForMissingRelease(t *testing.T) {
	setupTestDB(t)
	svc := &releaseService{}
	err := svc.UpdateStep(context.Background(), uuid.New(), "deploy", model.StepRunning, 0, "", nil, nil)
	if err == nil {
		t.Fatal("expected error for missing release")
	}
	if err != sql.ErrNoRows {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestUpdateStatusReturnsNotFoundForMissingRelease(t *testing.T) {
	setupTestDB(t)
	svc := &releaseService{}
	err := svc.UpdateStatus(context.Background(), uuid.New(), model.ReleaseRunning)
	if err == nil {
		t.Fatal("expected error for missing release")
	}
	if err != sql.ErrNoRows {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestReleaseStatusConvergenceFromSyncingToRunning(t *testing.T) {
	setupTestDB(t)
	releaseID := uuid.New()
	appID := uuid.New()
	manifestID := uuid.New()

	steps := model.DefaultReleaseSteps(model.Normal, model.ReleaseUpgrade)
	stepsJSON, _ := marshalJSON(steps, "[]")
	_, err := store.DB().ExecContext(context.Background(), `
		insert into releases (id, application_id, manifest_id, env, type, steps, status, created_at, updated_at, deleted_at)
		values ($1,$2,$3,'staging','Upgrade',$4,'Syncing',$5,$6,null)
	`, releaseID.String(), appID.String(), manifestID.String(), stepsJSON, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	svc := &releaseService{}
	err = svc.UpdateStep(context.Background(), releaseID, "ensure_namespace", model.StepSucceeded, 100, "done", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	release, err := svc.Get(context.Background(), releaseID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if release.Status != model.ReleaseRunning {
		t.Fatalf("expected Running after first step, got %q", release.Status)
	}
}

func TestReleaseStatusConvergenceToSucceeded(t *testing.T) {
	setupTestDB(t)
	releaseID := uuid.New()
	appID := uuid.New()
	manifestID := uuid.New()

	steps := model.DefaultReleaseSteps(model.Normal, model.ReleaseUpgrade)
	stepsJSON, _ := marshalJSON(steps, "[]")
	_, err := store.DB().ExecContext(context.Background(), `
		insert into releases (id, application_id, manifest_id, env, type, steps, status, created_at, updated_at, deleted_at)
		values ($1,$2,$3,'staging','Upgrade',$4,'Syncing',$5,$6,null)
	`, releaseID.String(), appID.String(), manifestID.String(), stepsJSON, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	svc := &releaseService{}
	for _, step := range steps {
		err := svc.UpdateStep(context.Background(), releaseID, step.Name, model.StepSucceeded, 100, "done", nil, nil)
		if err != nil {
			t.Fatalf("update step %q failed: %v", step.Name, err)
		}
	}

	release, err := svc.Get(context.Background(), releaseID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if release.Status != model.ReleaseSucceeded {
		t.Fatalf("expected Succeeded after all steps, got %q", release.Status)
	}
}

func TestReleaseStatusConvergenceToFailed(t *testing.T) {
	setupTestDB(t)
	releaseID := uuid.New()
	appID := uuid.New()
	manifestID := uuid.New()

	steps := model.DefaultReleaseSteps(model.Normal, model.ReleaseUpgrade)
	stepsJSON, _ := marshalJSON(steps, "[]")
	_, err := store.DB().ExecContext(context.Background(), `
		insert into releases (id, application_id, manifest_id, env, type, steps, status, created_at, updated_at, deleted_at)
		values ($1,$2,$3,'staging','Upgrade',$4,'Syncing',$5,$6,null)
	`, releaseID.String(), appID.String(), manifestID.String(), stepsJSON, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	svc := &releaseService{}
	err = svc.UpdateStep(context.Background(), releaseID, steps[0].Name, model.StepSucceeded, 100, "done", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = svc.UpdateStep(context.Background(), releaseID, steps[1].Name, model.StepFailed, 0, "auth error", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	release, err := svc.Get(context.Background(), releaseID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if release.Status != model.ReleaseFailed {
		t.Fatalf("expected Failed after step failure, got %q", release.Status)
	}
}

func TestReleaseStatusConvergenceTerminalProtectionAfterArgoFailed(t *testing.T) {
	setupTestDB(t)
	releaseID := uuid.New()
	appID := uuid.New()
	manifestID := uuid.New()

	steps := model.DefaultReleaseSteps(model.Normal, model.ReleaseUpgrade)
	stepsJSON, _ := marshalJSON(steps, "[]")
	_, err := store.DB().ExecContext(context.Background(), `
		insert into releases (id, application_id, manifest_id, env, type, steps, status, created_at, updated_at, deleted_at)
		values ($1,$2,$3,'staging','Upgrade',$4,'Failed',$5,$6,null)
	`, releaseID.String(), appID.String(), manifestID.String(), stepsJSON, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	svc := &releaseService{}
	err = svc.UpdateStep(context.Background(), releaseID, steps[0].Name, model.StepSucceeded, 100, "done", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	release, err := svc.Get(context.Background(), releaseID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if release.Status != model.ReleaseFailed {
		t.Fatalf("terminal Failed was overwritten: got %q", release.Status)
	}
}

func TestReleaseStatusConvergenceBootstrapApplyRolloutSuccess(t *testing.T) {
	setupTestDB(t)
	releaseID := uuid.New()
	appID := uuid.New()
	manifestID := uuid.New()

	steps := model.DefaultReleaseSteps(model.Normal, model.ReleaseUpgrade)
	stepsJSON, _ := marshalJSON(steps, "[]")
	_, err := store.DB().ExecContext(context.Background(), `
		insert into releases (id, application_id, manifest_id, env, type, steps, status, created_at, updated_at, deleted_at)
		values ($1,$2,$3,'staging','Upgrade',$4,'Syncing',$5,$6,null)
	`, releaseID.String(), appID.String(), manifestID.String(), stepsJSON, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	svc := &releaseService{}
	_ = svc.UpdateStep(context.Background(), releaseID, "freeze_inputs", model.StepSucceeded, 100, "inputs frozen", nil, nil)
	_ = svc.UpdateStep(context.Background(), releaseID, "ensure_namespace", model.StepSucceeded, 100, "namespace ready", nil, nil)
	_ = svc.UpdateStep(context.Background(), releaseID, "ensure_pull_secret", model.StepSucceeded, 100, "pull secret ready", nil, nil)
	_ = svc.UpdateStep(context.Background(), releaseID, "ensure_appproject_destination", model.StepSucceeded, 100, "appproject destination ready", nil, nil)
	_ = svc.UpdateStep(context.Background(), releaseID, "render_deployment_bundle", model.StepSucceeded, 100, "bundle rendered", nil, nil)
	_ = svc.UpdateStep(context.Background(), releaseID, "publish_bundle", model.StepSucceeded, 100, "bundle published", nil, nil)
	_ = svc.UpdateStep(context.Background(), releaseID, "create_argocd_application", model.StepSucceeded, 100, "application created", nil, nil)
	_ = svc.UpdateStep(context.Background(), releaseID, "start_deployment", model.StepSucceeded, 100, "deployment started", nil, nil)
	_ = svc.UpdateStep(context.Background(), releaseID, "observe_rollout", model.StepSucceeded, 100, "deployment healthy", nil, nil)
	_ = svc.UpdateStep(context.Background(), releaseID, "finalize_release", model.StepSucceeded, 100, "release finalized", nil, nil)

	release, err := svc.Get(context.Background(), releaseID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if release.Status != model.ReleaseSucceeded {
		t.Fatalf("expected Succeeded after bootstrap+apply+rollout success, got %q", release.Status)
	}
	// Verify step messages are preserved
	foundDeploy := false
	for _, s := range release.Steps {
		if s.Code == "observe_rollout" {
			foundDeploy = true
			if s.Message != "deployment healthy" {
				t.Fatalf("expected step message 'deployment healthy', got %q", s.Message)
			}
		}
	}
	if !foundDeploy {
		t.Fatal("observe_rollout step not found")
	}
}

func TestReleaseStatusConvergenceSyncFailure(t *testing.T) {
	setupTestDB(t)
	releaseID := uuid.New()
	appID := uuid.New()
	manifestID := uuid.New()

	steps := model.DefaultReleaseSteps(model.Normal, model.ReleaseUpgrade)
	stepsJSON, _ := marshalJSON(steps, "[]")
	_, err := store.DB().ExecContext(context.Background(), `
		insert into releases (id, application_id, manifest_id, env, type, steps, status, created_at, updated_at, deleted_at)
		values ($1,$2,$3,'staging','Upgrade',$4,'Syncing',$5,$6,null)
	`, releaseID.String(), appID.String(), manifestID.String(), stepsJSON, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	svc := &releaseService{}
	// Sync fails during bootstrap
	_ = svc.UpdateStep(context.Background(), releaseID, "ensure_namespace", model.StepSucceeded, 100, "namespace ready", nil, nil)
	_ = svc.UpdateStep(context.Background(), releaseID, "ensure_pull_secret", model.StepFailed, 0, "secret creation denied: rbac forbidden", nil, nil)

	release, err := svc.Get(context.Background(), releaseID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if release.Status != model.ReleaseFailed {
		t.Fatalf("expected Failed after sync/bootstrap failure, got %q", release.Status)
	}
	// Verify failure message is preserved
	foundSecret := false
	for _, s := range release.Steps {
		if s.Code == "ensure_pull_secret" {
			foundSecret = true
			if s.Message != "secret creation denied: rbac forbidden" {
				t.Fatalf("expected step message 'secret creation denied: rbac forbidden', got %q", s.Message)
			}
		}
	}
	if !foundSecret {
		t.Fatal("ensure_pull_secret step not found")
	}
}

func TestReleaseStatusConvergenceRolloutFailure(t *testing.T) {
	setupTestDB(t)
	releaseID := uuid.New()
	appID := uuid.New()
	manifestID := uuid.New()

	steps := model.DefaultReleaseSteps(model.Canary, model.ReleaseUpgrade)
	stepsJSON, _ := marshalJSON(steps, "[]")
	_, err := store.DB().ExecContext(context.Background(), `
		insert into releases (id, application_id, manifest_id, env, type, steps, status, created_at, updated_at, deleted_at)
		values ($1,$2,$3,'staging','Upgrade',$4,'Running',$5,$6,null)
	`, releaseID.String(), appID.String(), manifestID.String(), stepsJSON, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	svc := &releaseService{}
	// Bootstrap and apply succeed
	_ = svc.UpdateStep(context.Background(), releaseID, "ensure_namespace", model.StepSucceeded, 100, "done", nil, nil)
	_ = svc.UpdateStep(context.Background(), releaseID, "ensure_pull_secret", model.StepSucceeded, 100, "done", nil, nil)
	_ = svc.UpdateStep(context.Background(), releaseID, "ensure_appproject_destination", model.StepSucceeded, 100, "done", nil, nil)
	_ = svc.UpdateStep(context.Background(), releaseID, "render_deployment_bundle", model.StepSucceeded, 100, "done", nil, nil)
	_ = svc.UpdateStep(context.Background(), releaseID, "publish_bundle", model.StepSucceeded, 100, "done", nil, nil)
	_ = svc.UpdateStep(context.Background(), releaseID, "create_argocd_application", model.StepSucceeded, 100, "done", nil, nil)
	// Canary rollout fails at 30%
	_ = svc.UpdateStep(context.Background(), releaseID, "deploy_canary", model.StepSucceeded, 100, "done", nil, nil)
	_ = svc.UpdateStep(context.Background(), releaseID, "canary_10", model.StepSucceeded, 100, "done", nil, nil)
	_ = svc.UpdateStep(context.Background(), releaseID, "canary_30", model.StepFailed, 0, "analysis failed: error rate 15% > threshold 5%", nil, nil)

	release, err := svc.Get(context.Background(), releaseID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if release.Status != model.ReleaseFailed {
		t.Fatalf("expected Failed after rollout failure, got %q", release.Status)
	}
	// Verify rollout failure message is preserved
	foundCanary := false
	for _, s := range release.Steps {
		if s.Code == "canary_30" {
			foundCanary = true
			if s.Message != "analysis failed: error rate 15% > threshold 5%" {
				t.Fatalf("expected step message 'analysis failed: error rate 15%% > threshold 5%%', got %q", s.Message)
			}
		}
	}
	if !foundCanary {
		t.Fatal("canary_30 step not found")
	}
}

func TestReleaseStatusConvergenceDuplicateLateEventsAfterTerminal(t *testing.T) {
	setupTestDB(t)
	releaseID := uuid.New()
	appID := uuid.New()
	manifestID := uuid.New()

	steps := model.DefaultReleaseSteps(model.Normal, model.ReleaseUpgrade)
	stepsJSON, _ := marshalJSON(steps, "[]")
	_, err := store.DB().ExecContext(context.Background(), `
		insert into releases (id, application_id, manifest_id, env, type, steps, status, created_at, updated_at, deleted_at)
		values ($1,$2,$3,'staging','Upgrade',$4,'Succeeded',$5,$6,null)
	`, releaseID.String(), appID.String(), manifestID.String(), stepsJSON, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	svc := &releaseService{}
	// Late duplicate/orphan step events should not alter terminal status
	_ = svc.UpdateStep(context.Background(), releaseID, "deploy ready", model.StepFailed, 0, "late failure", nil, nil)
	_ = svc.UpdateStep(context.Background(), releaseID, "orphan late step", model.StepRunning, 50, "late running", nil, nil)

	release, err := svc.Get(context.Background(), releaseID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if release.Status != model.ReleaseSucceeded {
		t.Fatalf("terminal Succeeded was overwritten by late events: got %q", release.Status)
	}
}
