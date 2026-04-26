package service

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	appconfigdownstream "github.com/bsonger/devflow-service/internal/appconfig/transport/downstream"
	appservicedownstream "github.com/bsonger/devflow-service/internal/appservice/transport/downstream"
	imagedomain "github.com/bsonger/devflow-service/internal/image/domain"
	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	store "github.com/bsonger/devflow-service/internal/platform/db"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

func setupManifestTestDB(t *testing.T) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	createTable := `
CREATE TABLE manifests (
  id TEXT PRIMARY KEY,
  application_id TEXT NOT NULL,
  environment_id TEXT NOT NULL,
  image_id TEXT NOT NULL,
  image_ref TEXT NOT NULL,
  artifact_repository TEXT NOT NULL DEFAULT '',
  artifact_tag TEXT NOT NULL DEFAULT '',
  artifact_ref TEXT NOT NULL DEFAULT '',
  artifact_digest TEXT NOT NULL DEFAULT '',
  artifact_media_type TEXT NOT NULL DEFAULT '',
  artifact_pushed_at DATETIME NULL,
  services_snapshot TEXT NOT NULL DEFAULT '[]',
  workload_config_snapshot TEXT NOT NULL DEFAULT '{}',
  rendered_objects TEXT NOT NULL DEFAULT '[]',
  rendered_yaml TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL,
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
		db.Close()
		store.InitPostgres(nil)
	})
}

func TestBuildManifestPrefersDigestAndRendersObjects(t *testing.T) {
	req := &manifestdomain.CreateManifestRequest{
		ApplicationID: mustUUID("11111111-1111-1111-1111-111111111111"),
		ImageID:       mustUUID("33333333-3333-3333-3333-333333333333"),
	}
	image := &imagedomain.Image{
		ApplicationID: req.ApplicationID,
		Name:          "demo-api",
		RepoAddress:   "registry.cn-hangzhou.aliyuncs.com/devflow",
		Tag:           "20260411-120000",
		Digest:        "sha256:abc",
	}
	workload := &appconfigdownstream.WorkloadConfig{
		ID:           "wc-1",
		Name:         "demo-workload",
		Replicas:     2,
		WorkloadType: "deployment",
		Strategy:     "rolling-update",
	}
	services := []appservicedownstream.Service{{
		ID:   "svc-1",
		Name: "demo-api",
		Ports: []appservicedownstream.ServicePort{{
			Name:        "http",
			ServicePort: 80,
			TargetPort:  8080,
			Protocol:    "TCP",
		}},
	}}

	got, err := buildManifest(req, image, "demo-api", workload, services, imagedomain.ImageRegistryConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if got.ImageRef != "registry.cn-hangzhou.aliyuncs.com/devflow/demo-api@sha256:abc" {
		t.Fatalf("unexpected image ref %q", got.ImageRef)
	}
	if got.EnvironmentID != "" {
		t.Fatalf("EnvironmentID = %q, want empty", got.EnvironmentID)
	}
	if len(got.RenderedObjects) != 2 {
		t.Fatalf("unexpected rendered object count %d", len(got.RenderedObjects))
	}
	if got.RenderedYAML == "" {
		t.Fatal("expected rendered yaml")
	}
}

func TestBuildManifestFallsBackToConfiguredRegistryForGitRepoAddress(t *testing.T) {
	req := &manifestdomain.CreateManifestRequest{
		ApplicationID: mustUUID("11111111-1111-1111-1111-111111111111"),
		ImageID:       mustUUID("33333333-3333-3333-3333-333333333333"),
	}
	image := &imagedomain.Image{
		ApplicationID: req.ApplicationID,
		Name:          "devflow-runtime-service",
		RepoAddress:   "git@github.com:bsonger/devflow-runtime-service.git",
		Tag:           "20260411-120000",
		Digest:        "sha256:abc",
	}
	workload := &appconfigdownstream.WorkloadConfig{Replicas: 1, WorkloadType: "deployment"}
	got, err := buildManifest(req, image, "devflow-runtime-service", workload, nil, imagedomain.ImageRegistryConfig{
		Registry:  "registry.cn-hangzhou.aliyuncs.com",
		Namespace: "devflow",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.ImageRef != "registry.cn-hangzhou.aliyuncs.com/devflow/devflow-runtime-service@sha256:abc" {
		t.Fatalf("unexpected image ref %q", got.ImageRef)
	}
	hasPullSecret := false
	for _, item := range got.RenderedObjects {
		if item.Kind == "ConfigMap" || item.Kind == "VirtualService" {
			t.Fatalf("unexpected rendered object kind %s", item.Kind)
		}
		if item.Kind == "Deployment" {
			if strings.Contains(item.YAML, "imagePullSecrets:") && strings.Contains(item.YAML, "aliyun-docker-config") {
				hasPullSecret = true
			}
			if strings.Contains(item.YAML, "devflow.environment/id:") {
				t.Fatalf("did not expect environment label in manifest deployment, got:\n%s", item.YAML)
			}
		}
	}
	if !hasPullSecret {
		t.Fatal("expected deployment to include aliyun-docker-config imagePullSecrets")
	}
}

func TestResolveManifestImageRepositoryRejectsUndeployableRepository(t *testing.T) {
	_, err := resolveManifestImageRepository(&imagedomain.Image{
		Name:        "demo-api",
		RepoAddress: "git@github.com:bsonger/devflow-runtime-service.git",
	}, imagedomain.ImageRegistryConfig{})
	if err != ErrManifestImageRepositoryMissing {
		t.Fatalf("err = %v, want %v", err, ErrManifestImageRepositoryMissing)
	}
}

func mustUUID(value string) uuid.UUID {
	id, err := uuid.Parse(value)
	if err != nil {
		panic(err)
	}
	return id
}

func TestManifestDeleteSoftDeletesByID(t *testing.T) {
	setupManifestTestDB(t)
	manifestID := uuid.New()
	appID := uuid.New()
	imageID := uuid.New()
	now := time.Now()

	_, err := store.DB().ExecContext(context.Background(), `
		insert into manifests (id, application_id, environment_id, image_id, image_ref, status, created_at, updated_at, deleted_at)
		values ($1,$2,'',$3,'repo/demo@sha256:abc','Ready',$4,$5,null)
	`, manifestID.String(), appID.String(), imageID.String(), now, now)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	svc := &manifestService{}
	err = svc.Delete(context.Background(), manifestID)
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	_, err = svc.Get(context.Background(), manifestID)
	if err == nil {
		t.Fatal("expected error after soft delete")
	}
	if err != sql.ErrNoRows {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestManifestDeleteReturnsNotFoundForMissingID(t *testing.T) {
	setupManifestTestDB(t)
	svc := &manifestService{}
	err := svc.Delete(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error for missing manifest")
	}
	if err != sql.ErrNoRows {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestManifestResourcesViewStillGroupsLegacyKinds(t *testing.T) {
	manifest := &manifestdomain.Manifest{
		BaseModel: model.BaseModel{ID: uuid.New()},
		RenderedObjects: []manifestdomain.ManifestRenderedObject{
			{Kind: "ConfigMap", Name: "cfg", YAML: "kind: ConfigMap\nmetadata:\n  name: cfg\n"},
			{Kind: "VirtualService", Name: "vs", YAML: "kind: VirtualService\nmetadata:\n  name: vs\n"},
		},
	}
	view, err := buildManifestResourcesView(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if view.Resources.ConfigMap == nil || view.Resources.VirtualService == nil {
		t.Fatalf("expected legacy kinds to still decode, got %+v", view.Resources)
	}
}
