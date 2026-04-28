package service

import (
	"context"
	"database/sql"
	"testing"
	"time"

	appconfigdownstream "github.com/bsonger/devflow-service/internal/appconfig/transport/downstream"
	appservicedownstream "github.com/bsonger/devflow-service/internal/appservice/transport/downstream"
	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	store "github.com/bsonger/devflow-service/internal/platform/db"
	"github.com/bsonger/devflow-service/internal/platform/oci"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/google/uuid"
	tknv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
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
  git_revision TEXT NOT NULL DEFAULT '',
  repo_address TEXT NOT NULL DEFAULT '',
  commit_hash TEXT NOT NULL DEFAULT '',
  image_tag TEXT NOT NULL DEFAULT '',
  image_digest TEXT NOT NULL DEFAULT '',
  pipeline_id TEXT NOT NULL DEFAULT '',
  trace_id TEXT NOT NULL DEFAULT '',
  span_id TEXT NOT NULL DEFAULT '',
  steps TEXT NOT NULL DEFAULT '[]',
  image_ref TEXT NOT NULL,
  services_snapshot TEXT NOT NULL DEFAULT '[]',
  workload_config_snapshot TEXT NOT NULL DEFAULT '{}',
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
	}
	workload := &appconfigdownstream.WorkloadConfig{
		ID:                 "wc-1",
		Replicas:           2,
		ServiceAccountName: "runtime-service",
		Labels:             map[string]string{"team": "platform"},
		Annotations:        map[string]string{"sidecar.istio.io/inject": "true"},
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
	target := oci.ImageTarget{
		Name: "demo-api",
		Tag:  "20260411-120000",
		Ref:  "registry.cn-hangzhou.aliyuncs.com/devflow/demo-api:20260411-120000",
	}

	got, err := buildManifest(req, "demo-api", "registry.cn-hangzhou.aliyuncs.com/devflow", target, "sha256:abc", workload, services)
	if err != nil {
		t.Fatal(err)
	}
	if got.ImageRef != "registry.cn-hangzhou.aliyuncs.com/devflow/demo-api@sha256:abc" {
		t.Fatalf("unexpected image ref %q", got.ImageRef)
	}
}

func TestBuildManifestFallsBackToConfiguredRegistryForGitRepoAddress(t *testing.T) {
	req := &manifestdomain.CreateManifestRequest{
		ApplicationID: mustUUID("11111111-1111-1111-1111-111111111111"),
	}
	workload := &appconfigdownstream.WorkloadConfig{Replicas: 1}
	target := oci.ImageTarget{
		Name: "devflow-runtime-service",
		Tag:  "20260411-120000",
		Ref:  "registry.cn-hangzhou.aliyuncs.com/devflow/devflow-runtime-service:20260411-120000",
	}
	got, err := buildManifest(req, "devflow-runtime-service", "git@github.com:bsonger/devflow-runtime-service.git", target, "sha256:abc", workload, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.ImageRef != "registry.cn-hangzhou.aliyuncs.com/devflow/devflow-runtime-service@sha256:abc" {
		t.Fatalf("unexpected image ref %q", got.ImageRef)
	}
}

func TestNormalizeGitRevisionDefaultsToMain(t *testing.T) {
	if got := normalizeGitRevision(""); got != "main" {
		t.Fatalf("normalizeGitRevision(\"\") = %q, want main", got)
	}
	if got := normalizeGitRevision("  feature/demo "); got != "feature/demo" {
		t.Fatalf("normalizeGitRevision(trim) = %q, want feature/demo", got)
	}
}

func TestBuildManifestPipelineRunUsesGitRevisionAndAnnotations(t *testing.T) {
	manifest := &manifestdomain.Manifest{
		BaseModel:     model.BaseModel{ID: uuid.MustParse("11111111-1111-1111-1111-111111111111")},
		ApplicationID: uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		GitRevision:   "feature/demo",
		RepoAddress:   "git@github.com:example/demo.git",
	}
	target := oci.ImageTarget{
		Name: "demo-api",
		Tag:  "20260427-120000",
		Ref:  "registry.example.com/devflow/demo-api:20260427-120000",
	}

	run := buildManifestPipelineRun(manifest, "pvc-1", "registry.example.com/devflow", target)

	if run.Spec.PipelineRef == nil || run.Spec.PipelineRef.Name != manifestTektonBuildPipeline {
		t.Fatalf("pipeline ref = %+v", run.Spec.PipelineRef)
	}
	params := map[string]string{}
	for _, item := range run.Spec.Params {
		params[item.Name] = item.Value.StringVal
	}
	if params["git-url"] != manifest.RepoAddress {
		t.Fatalf("git-url = %q", params["git-url"])
	}
	if params["git-revision"] != manifest.GitRevision {
		t.Fatalf("git-revision = %q", params["git-revision"])
	}
	if params["image-registry"] != "registry.example.com/devflow" {
		t.Fatalf("image-registry = %q", params["image-registry"])
	}
	if run.Annotations["devflow.manifest/id"] != manifest.ID.String() {
		t.Fatalf("annotation manifest id = %q", run.Annotations["devflow.manifest/id"])
	}
}

func TestBuildManifestStepsFromPipelineIncludesTasksAndFinally(t *testing.T) {
	pipeline := &tknv1.Pipeline{
		Spec: tknv1.PipelineSpec{
			Tasks: []tknv1.PipelineTask{
				{Name: "git-clone"},
				{Name: "image-build-and-push"},
			},
			Finally: []tknv1.PipelineTask{
				{Name: "notify"},
			},
		},
	}

	steps := buildManifestStepsFromPipeline(pipeline)

	if len(steps) != 3 {
		t.Fatalf("len(steps) = %d, want 3", len(steps))
	}
	if steps[0].TaskName != "git-clone" || steps[1].TaskName != "image-build-and-push" || steps[2].TaskName != "notify" {
		t.Fatalf("unexpected steps: %+v", steps)
	}
	for _, step := range steps {
		if step.Status != model.StepPending {
			t.Fatalf("step status = %q, want %q", step.Status, model.StepPending)
		}
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
	now := time.Now()

	_, err := store.DB().ExecContext(context.Background(), `
		insert into manifests (id, application_id, image_ref, status, created_at, updated_at, deleted_at)
		values ($1,$2,$3,'Ready',$4,$5,null)
	`, manifestID.String(), appID.String(), "repo/demo@sha256:abc", now, now)
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

func TestManifestResourcesViewStillBuildsLegacyResourcesEndpoint(t *testing.T) {
	manifest := &manifestdomain.Manifest{
		BaseModel:     model.BaseModel{ID: uuid.New()},
		ApplicationID: uuid.New(),
		ImageRef:      "registry.example.com/devflow/demo-api@sha256:abc",
		ServicesSnapshot: []manifestdomain.ManifestService{
			{Name: "cfg", Ports: []manifestdomain.ManifestServicePort{{Name: "http", ServicePort: 80, TargetPort: 8080}}},
		},
		WorkloadConfigSnapshot: manifestdomain.ManifestWorkloadConfig{
			Replicas: 1,
		},
	}
	view, err := buildManifestResourcesView(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if view.Resources.Deployment == nil || len(view.Resources.Services) != 1 {
		t.Fatalf("expected derived resources, got %+v", view.Resources)
	}
}
