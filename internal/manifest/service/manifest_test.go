package service

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	appconfigdownstream "github.com/bsonger/devflow-service/internal/appconfig/transport/downstream"
	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	store "github.com/bsonger/devflow-service/internal/platform/db"
	platformlogger "github.com/bsonger/devflow-service/internal/platform/logger"
	"github.com/bsonger/devflow-service/internal/platform/oci"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	releasesupport "github.com/bsonger/devflow-service/internal/release/support"
	servicedownstream "github.com/bsonger/devflow-service/internal/service/transport/downstream"
	"github.com/google/uuid"
	tknv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
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
		_ = db.Close()
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
	services := []servicedownstream.Service{{
		ID:   "svc-1",
		Name: "demo-api",
		Ports: []servicedownstream.ServicePort{{
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
	if got.Status != model.ManifestPending {
		t.Fatalf("status = %q, want %q", got.Status, model.ManifestPending)
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

type stubManifestApplicationReader struct {
	getFn func(context.Context, uuid.UUID) (*releasesupport.ApplicationProjection, error)
}

func (s stubManifestApplicationReader) Get(ctx context.Context, id uuid.UUID) (*releasesupport.ApplicationProjection, error) {
	return s.getFn(ctx, id)
}

func TestBuildManifestFreezesCanonicalizedWorkloadSnapshot(t *testing.T) {
	req := &manifestdomain.CreateManifestRequest{
		ApplicationID: mustUUID("11111111-1111-1111-1111-111111111111"),
	}
	workload := &appconfigdownstream.WorkloadConfig{
		ID:                 "wc-legacy-migrated",
		Replicas:           3,
		ServiceAccountName: "demo-api",
		Resources: appconfigdownstream.WorkloadResourceRequirements{
			SizeClass: "medium",
			Requests:  appconfigdownstream.WorkloadResourceList{CPU: "250m", Memory: "256Mi"},
			Limits:    appconfigdownstream.WorkloadResourceList{CPU: "1", Memory: "1Gi"},
		},
		Probes: appconfigdownstream.WorkloadProbes{
			Readiness: &appconfigdownstream.WorkloadProbe{Path: "/readyz", Port: "http", PeriodSeconds: 5},
		},
		Env:         []appconfigdownstream.EnvVar{{Name: "LOG_LEVEL", Value: "debug"}},
		Labels:      map[string]string{"team": "platform"},
		Annotations: map[string]string{"example.com/revision": "migrated"},
	}
	target := oci.ImageTarget{
		Name: "demo-api",
		Tag:  "20260411-120000",
		Ref:  "registry.example.com/devflow/demo-api:20260411-120000",
	}
	got, err := buildManifest(req, "demo-api", "git@github.com:example/demo-api.git", target, "sha256:abc", workload, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.WorkloadConfigSnapshot.Resources != workload.Resources {
		t.Fatalf("resources snapshot = %+v, want %+v", got.WorkloadConfigSnapshot.Resources, workload.Resources)
	}
	if got.WorkloadConfigSnapshot.Resources.SizeClass != workload.Resources.SizeClass {
		t.Fatalf("size class = %q, want %q", got.WorkloadConfigSnapshot.Resources.SizeClass, workload.Resources.SizeClass)
	}
	if got.WorkloadConfigSnapshot.Probes.Readiness == nil || got.WorkloadConfigSnapshot.Probes.Readiness.Path != "/readyz" {
		t.Fatalf("unexpected probes snapshot %+v", got.WorkloadConfigSnapshot.Probes)
	}
	if len(got.WorkloadConfigSnapshot.Env) != 1 || got.WorkloadConfigSnapshot.Env[0].Name != "LOG_LEVEL" {
		t.Fatalf("unexpected env snapshot %+v", got.WorkloadConfigSnapshot.Env)
	}
}

func TestCreateManifestReturnsMissingConfigWhenCleanupDeletedLegacyRow(t *testing.T) {
	originalCreatePVC := manifestCreatePVC
	originalCreatePipelineRun := manifestCreatePipelineRun
	originalPatchPVCOwner := manifestPatchPVCOwner
	originalGetPipeline := manifestGetPipeline
	originalLogger := platformlogger.Logger
	t.Cleanup(func() {
		manifestCreatePVC = originalCreatePVC
		manifestCreatePipelineRun = originalCreatePipelineRun
		manifestPatchPVCOwner = originalPatchPVCOwner
		manifestGetPipeline = originalGetPipeline
		platformlogger.Logger = originalLogger
	})
	platformlogger.Logger = zap.NewNop()

	manifestCreatePVC = func(context.Context, string, string, string, string) (*corev1.PersistentVolumeClaim, error) {
		t.Fatal("manifest build should not be submitted when workload config is missing")
		return nil, nil
	}
	manifestCreatePipelineRun = func(context.Context, string, *tknv1.PipelineRun) (*tknv1.PipelineRun, error) {
		t.Fatal("manifest build should not be submitted when workload config is missing")
		return nil, nil
	}
	manifestPatchPVCOwner = func(context.Context, *corev1.PersistentVolumeClaim, *tknv1.PipelineRun) error {
		t.Fatal("manifest build should not be submitted when workload config is missing")
		return nil
	}
	manifestGetPipeline = func(context.Context, string, string) (*tknv1.Pipeline, error) {
		t.Fatal("manifest build should not be submitted when workload config is missing")
		return nil, nil
	}

	configAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workload-configs" || r.URL.RawQuery != "application_id=11111111-1111-1111-1111-111111111111" {
			t.Fatalf("unexpected request path=%s query=%s", r.URL.Path, r.URL.RawQuery)
		}
		_, _ = io.WriteString(w, `{"data":[]}`)
	}))
	defer configAPI.Close()

	runtimeCfg := releasesupport.CurrentRuntimeConfig()
	cfg := runtimeCfg
	cfg.Downstream.ConfigServiceBaseURL = configAPI.URL
	t.Cleanup(func() { releasesupport.ConfigureRuntimeConfig(runtimeCfg) })
	releasesupport.ConfigureRuntimeConfig(cfg)

	svc := &manifestService{apps: stubManifestApplicationReader{
		getFn: func(_ context.Context, id uuid.UUID) (*releasesupport.ApplicationProjection, error) {
			return &releasesupport.ApplicationProjection{ID: id, Name: "demo-api", RepoAddress: "git@github.com:example/demo-api.git"}, nil
		},
	}}
	_, err := svc.CreateManifest(context.Background(), &manifestdomain.CreateManifestRequest{ApplicationID: mustUUID("11111111-1111-1111-1111-111111111111")})
	if !errors.Is(err, ErrManifestWorkloadConfigMissing) {
		t.Fatalf("CreateManifest() error = %v, want %v", err, ErrManifestWorkloadConfigMissing)
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

	run := buildManifestPipelineRun(manifest, "pvc-1", "registry.example.com/devflow", target, releasesupport.ManifestBuildTektonConfig{})

	if run.Spec.PipelineRef == nil || run.Spec.PipelineRef.Name != "devflow-tekton-image-build-push-only" {
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
	if params["SERVICE_NAME"] != target.Name {
		t.Fatalf("SERVICE_NAME = %q", params["SERVICE_NAME"])
	}
	if run.Annotations["devflow.manifest/id"] != manifest.ID.String() {
		t.Fatalf("annotation manifest id = %q", run.Annotations["devflow.manifest/id"])
	}
}

func TestBuildManifestPipelineRunUsesConfiguredTektonPipeline(t *testing.T) {
	manifest := &manifestdomain.Manifest{
		BaseModel:   model.BaseModel{ID: uuid.MustParse("11111111-1111-1111-1111-111111111111")},
		GitRevision: "feature/demo",
		RepoAddress: "git@github.com:example/demo.git",
	}
	target := oci.ImageTarget{
		Name: "demo-api",
		Tag:  "20260427-120000",
		Ref:  "registry.example.com/devflow/demo-api:20260427-120000",
	}

	run := buildManifestPipelineRun(manifest, "pvc-1", "registry.example.com/devflow", target, releasesupport.ManifestBuildTektonConfig{
		BuildPipeline:   "custom-build-pipeline",
		PVCGenerateName: "custom-build-pipeline",
	})

	if run.Spec.PipelineRef == nil || run.Spec.PipelineRef.Name != "custom-build-pipeline" {
		t.Fatalf("pipeline ref = %+v", run.Spec.PipelineRef)
	}
	if run.GenerateName != "custom-build-pipeline-run-" {
		t.Fatalf("generate name = %q", run.GenerateName)
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
		values ($1,$2,$3,'Available',$4,$5,null)
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

func TestUpdateBuildResultDoesNotChangeRuntimeReportedStatus(t *testing.T) {
	setupManifestTestDB(t)
	svc := &manifestService{}
	manifestID := uuid.New()
	appID := uuid.New()
	now := time.Now()
	manifest := &manifestdomain.Manifest{
		BaseModel:     model.BaseModel{ID: manifestID, CreatedAt: now, UpdatedAt: now},
		ApplicationID: appID,
		PipelineID:    "pipe-running",
		Status:        model.ManifestRunning,
		Steps: []model.ImageTask{
			{TaskName: "git-clone", Status: model.StepSucceeded},
			{TaskName: "image-build-and-push", Status: model.StepRunning},
		},
		ImageRef: "registry.example.com/devflow/demo-api:pending",
	}
	if err := svc.repoStore().Insert(context.Background(), manifest); err != nil {
		t.Fatalf("insert manifest: %v", err)
	}

	if err := svc.UpdateBuildResult(context.Background(), manifest.PipelineID, "abcdef123456", "registry.example.com/devflow/demo-api@sha256:abc", "20260428-120000", "sha256:abc"); err != nil {
		t.Fatalf("UpdateBuildResult() error = %v", err)
	}

	got, err := svc.Get(context.Background(), manifestID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Status != model.ManifestRunning {
		t.Fatalf("status = %q, want %q", got.Status, model.ManifestRunning)
	}
	if got.ImageDigest != "sha256:abc" {
		t.Fatalf("image digest = %q", got.ImageDigest)
	}
}

func TestUpdateStepStatusDoesNotPromoteManifestStatus(t *testing.T) {
	setupManifestTestDB(t)
	svc := &manifestService{}
	manifestID := uuid.New()
	appID := uuid.New()
	now := time.Now()
	manifest := &manifestdomain.Manifest{
		BaseModel:     model.BaseModel{ID: manifestID, CreatedAt: now, UpdatedAt: now},
		ApplicationID: appID,
		PipelineID:    "pipe-success",
		Status:        model.ManifestAvailable,
		CommitHash:    "abcdef123456",
		ImageRef:      "registry.example.com/devflow/demo-api@sha256:abc",
		ImageTag:      "20260428-120000",
		ImageDigest:   "sha256:abc",
		Steps: []model.ImageTask{
			{TaskName: "git-clone", Status: model.StepSucceeded},
			{TaskName: "image-build-and-push", Status: model.StepRunning},
		},
	}
	if err := svc.repoStore().Insert(context.Background(), manifest); err != nil {
		t.Fatalf("insert manifest: %v", err)
	}

	if err := svc.UpdateStepStatus(context.Background(), manifest.PipelineID, "image-build-and-push", model.StepSucceeded, "done", nil, nil); err != nil {
		t.Fatalf("UpdateStepStatus() error = %v", err)
	}

	got, err := svc.Get(context.Background(), manifestID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Status != model.ManifestAvailable {
		t.Fatalf("status = %q, want %q", got.Status, model.ManifestAvailable)
	}
	if got.Steps[1].Status != model.StepSucceeded {
		t.Fatalf("step status = %q, want %q", got.Steps[1].Status, model.StepSucceeded)
	}
}

func TestUpdateManifestStatusAcceptsRuntimeReportedAvailable(t *testing.T) {
	setupManifestTestDB(t)
	svc := &manifestService{}
	manifestID := uuid.New()
	appID := uuid.New()
	now := time.Now()
	manifest := &manifestdomain.Manifest{
		BaseModel:     model.BaseModel{ID: manifestID, CreatedAt: now, UpdatedAt: now},
		ApplicationID: appID,
		PipelineID:    "pipe-gated",
		Status:        model.ManifestRunning,
		Steps: []model.ImageTask{
			{TaskName: "git-clone", Status: model.StepSucceeded},
			{TaskName: "image-build-and-push", Status: model.StepRunning},
		},
	}
	if err := svc.repoStore().Insert(context.Background(), manifest); err != nil {
		t.Fatalf("insert manifest: %v", err)
	}

	if err := svc.UpdateManifestStatus(context.Background(), manifest.PipelineID, model.ManifestAvailable); err != nil {
		t.Fatalf("UpdateManifestStatus() error = %v", err)
	}

	got, err := svc.Get(context.Background(), manifestID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Status != model.ManifestAvailable {
		t.Fatalf("status = %q, want %q", got.Status, model.ManifestAvailable)
	}
}

func TestUpdateBuildResultNoopWhenPayloadAlreadyPersisted(t *testing.T) {
	setupManifestTestDB(t)
	svc := &manifestService{}
	manifestID := uuid.New()
	appID := uuid.New()
	now := time.Now()
	manifest := &manifestdomain.Manifest{
		BaseModel:     model.BaseModel{ID: manifestID, CreatedAt: now, UpdatedAt: now},
		ApplicationID: appID,
		PipelineID:    "pipe-noop-result",
		Status:        model.ManifestAvailable,
		CommitHash:    "abcdef123456",
		ImageRef:      "registry.example.com/devflow/demo-api@sha256:abc",
		ImageTag:      "20260428-120000",
		ImageDigest:   "sha256:abc",
		Steps: []model.ImageTask{
			{TaskName: "git-clone", Status: model.StepSucceeded},
			{TaskName: "image-build-and-push", Status: model.StepSucceeded},
		},
	}
	if err := svc.repoStore().Insert(context.Background(), manifest); err != nil {
		t.Fatalf("insert manifest: %v", err)
	}

	before, err := svc.Get(context.Background(), manifestID)
	if err != nil {
		t.Fatalf("Get() before error = %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := svc.UpdateBuildResult(context.Background(), manifest.PipelineID, manifest.CommitHash, manifest.ImageRef, manifest.ImageTag, manifest.ImageDigest); err != nil {
		t.Fatalf("UpdateBuildResult() error = %v", err)
	}
	after, err := svc.Get(context.Background(), manifestID)
	if err != nil {
		t.Fatalf("Get() after error = %v", err)
	}
	if !after.UpdatedAt.Equal(before.UpdatedAt) {
		t.Fatalf("updated_at changed on noop build result: before=%s after=%s", before.UpdatedAt, after.UpdatedAt)
	}
}

func TestUpdateManifestStatusByIDNoopWhenConverged(t *testing.T) {
	setupManifestTestDB(t)
	svc := &manifestService{}
	manifestID := uuid.New()
	appID := uuid.New()
	now := time.Now()
	manifest := &manifestdomain.Manifest{
		BaseModel:     model.BaseModel{ID: manifestID, CreatedAt: now, UpdatedAt: now},
		ApplicationID: appID,
		PipelineID:    "pipe-noop-status",
		Status:        model.ManifestRunning,
		Steps: []model.ImageTask{
			{TaskName: "git-clone", Status: model.StepRunning},
			{TaskName: "image-build-and-push", Status: model.StepPending},
		},
	}
	if err := svc.repoStore().Insert(context.Background(), manifest); err != nil {
		t.Fatalf("insert manifest: %v", err)
	}

	before, err := svc.Get(context.Background(), manifestID)
	if err != nil {
		t.Fatalf("Get() before error = %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := svc.UpdateManifestStatusByID(context.Background(), manifestID, model.ManifestRunning); err != nil {
		t.Fatalf("UpdateManifestStatusByID() error = %v", err)
	}
	after, err := svc.Get(context.Background(), manifestID)
	if err != nil {
		t.Fatalf("Get() after error = %v", err)
	}
	if !after.UpdatedAt.Equal(before.UpdatedAt) {
		t.Fatalf("updated_at changed on noop status write: before=%s after=%s", before.UpdatedAt, after.UpdatedAt)
	}
}
