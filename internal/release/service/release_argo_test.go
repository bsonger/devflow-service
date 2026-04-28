package service

import (
	"context"
	"errors"
	"testing"
	"time"

	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	store "github.com/bsonger/devflow-service/internal/platform/db"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	releasesupport "github.com/bsonger/devflow-service/internal/release/support"
	"github.com/google/uuid"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	oras "oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

func TestBuildArgoApplicationUsesOCIArtifactSource(t *testing.T) {
	release := &model.Release{
		BaseModel:          model.BaseModel{ID: uuid.New()},
		ApplicationID:      uuid.New(),
		ManifestID:         uuid.New(),
		EnvironmentID:      "production",
		ArtifactRepository: "zot.zot.svc.cluster.local:5000/devflow/releases/demo-api/production",
		ArtifactDigest:     "sha256:abc",
		ArtifactRef:        "oci://zot.zot.svc.cluster.local:5000/devflow/releases/demo-api/production@sha256:abc",
	}
	manifest := &manifestdomain.Manifest{BaseModel: model.BaseModel{ID: release.ManifestID}}
	target := releasesupport.DeployTarget{Namespace: "checkout", DestinationServer: "https://cluster-prod.example.com"}

	app := buildArgoApplication(release, manifest, &releasesupport.ApplicationProjection{
		Name:        "demo-api",
		ProjectName: "checkout",
	}, target)

	if app.Spec.Source == nil {
		t.Fatal("expected oci application source")
	}
	if app.Spec.Source.Plugin != nil {
		t.Fatalf("unexpected plugin source: %+v", app.Spec.Source.Plugin)
	}
	if app.Spec.Source.RepoURL != "oci://zot.zot.svc.cluster.local:5000/devflow/releases/demo-api/production" {
		t.Fatalf("RepoURL = %q", app.Spec.Source.RepoURL)
	}
	if app.Spec.Source.TargetRevision != "sha256:abc" {
		t.Fatalf("TargetRevision = %q", app.Spec.Source.TargetRevision)
	}
	if app.Spec.Source.Path != "." {
		t.Fatalf("Path = %q", app.Spec.Source.Path)
	}
	if app.Spec.Destination.Namespace != target.Namespace {
		t.Fatalf("namespace = %q", app.Spec.Destination.Namespace)
	}
	if app.Spec.Destination.Server != target.DestinationServer {
		t.Fatalf("server = %q", app.Spec.Destination.Server)
	}
}

func TestBuildArgoApplicationDerivesOCIArtifactFromRef(t *testing.T) {
	release := &model.Release{
		BaseModel:     model.BaseModel{ID: uuid.New()},
		ApplicationID: uuid.New(),
		ManifestID:    uuid.New(),
		EnvironmentID: "production",
		ArtifactRef:   "oci://registry.example.com/devflow/releases/demo-api@sha256:def",
	}
	manifest := &manifestdomain.Manifest{BaseModel: model.BaseModel{ID: release.ManifestID}}
	target := releasesupport.DeployTarget{Namespace: "checkout", DestinationServer: "https://cluster-prod.example.com"}

	app := buildArgoApplication(release, manifest, &releasesupport.ApplicationProjection{
		Name:        "demo-api",
		ProjectName: "checkout",
	}, target)

	if app.Spec.Source == nil {
		t.Fatal("expected oci source")
	}
	if app.Spec.Source.RepoURL != "oci://registry.example.com/devflow/releases/demo-api" {
		t.Fatalf("RepoURL = %q", app.Spec.Source.RepoURL)
	}
	if app.Spec.Source.TargetRevision != "sha256:def" {
		t.Fatalf("TargetRevision = %q", app.Spec.Source.TargetRevision)
	}
	if app.Spec.Source.Path != "." {
		t.Fatalf("Path = %q", app.Spec.Source.Path)
	}
}

func TestDeriveReleaseArtifactMetadata(t *testing.T) {
	release := &model.Release{
		BaseModel:     model.BaseModel{ID: uuid.MustParse("11111111-1111-1111-1111-111111111111")},
		ApplicationID: uuid.New(),
		EnvironmentID: "production",
	}
	app := &releasesupport.ApplicationProjection{Name: "demo-api"}
	cfg := manifestdomain.ManifestRegistryConfig{
		Registry:   "registry.example.com",
		Namespace:  "devflow",
		Repository: "releases",
	}

	repository, tag, ref := deriveReleaseArtifactMetadata(release, app, cfg)
	if repository != "registry.example.com/devflow/releases/demo-api" {
		t.Fatalf("repository = %q", repository)
	}
	if tag != release.ID.String() {
		t.Fatalf("tag = %q", tag)
	}
	if ref != "oci://registry.example.com/devflow/releases/demo-api:"+release.ID.String() {
		t.Fatalf("ref = %q", ref)
	}
}

func TestDeriveReleaseArtifactMetadataFromBundlePrefersDigestRef(t *testing.T) {
	release := &model.Release{
		BaseModel:     model.BaseModel{ID: uuid.MustParse("11111111-1111-1111-1111-111111111111")},
		ApplicationID: uuid.New(),
		EnvironmentID: "production",
	}
	app := &releasesupport.ApplicationProjection{Name: "demo-api"}
	cfg := manifestdomain.ManifestRegistryConfig{
		Registry:   "registry.example.com",
		Namespace:  "devflow",
		Repository: "releases",
	}
	bundle := &model.ReleaseBundle{
		Files: []model.ReleaseBundleFile{{Path: "bundle.yaml", Content: "kind: Deployment\n"}},
	}

	repository, tag, digest, ref := deriveReleaseArtifactMetadataFromBundle(release, app, cfg, bundle)
	if repository != "registry.example.com/devflow/releases/demo-api" {
		t.Fatalf("repository = %q", repository)
	}
	if tag != release.ID.String() {
		t.Fatalf("tag = %q", tag)
	}
	if digest == "" {
		t.Fatal("expected digest")
	}
	if ref != "oci://registry.example.com/devflow/releases/demo-api@"+digest {
		t.Fatalf("ref = %q", ref)
	}
}

func TestApplyReleaseApplicationTriggersSyncAfterUpgrade(t *testing.T) {
	app := &appv1.Application{}
	updated := false
	synced := false
	err := applyReleaseApplication(context.Background(), model.ReleaseUpgrade, app,
		func(context.Context, *appv1.Application) error {
			t.Fatal("create should not be called for upgrade")
			return nil
		},
		func(_ context.Context, got *appv1.Application) error {
			updated = got == app
			return nil
		},
		func(_ context.Context, name string) error {
			synced = true
			return nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !updated {
		t.Fatal("expected update to be called")
	}
	if !synced {
		t.Fatal("expected sync to be called")
	}
}

func TestApplyReleaseApplicationTriggersSyncAfterInstall(t *testing.T) {
	app := &appv1.Application{}
	created := false
	synced := false
	err := applyReleaseApplication(context.Background(), model.ReleaseInstall, app,
		func(_ context.Context, got *appv1.Application) error {
			created = got == app
			return nil
		},
		func(context.Context, *appv1.Application) error {
			t.Fatal("update should not be called for install")
			return nil
		},
		func(_ context.Context, name string) error {
			synced = true
			return nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("expected create to be called")
	}
	if !synced {
		t.Fatal("expected sync to be called")
	}
}

func TestPersistArgoApplicationMetadataUpdatesRelease(t *testing.T) {
	setupTestDB(t)
	releaseID := uuid.New()
	appID := uuid.New()
	manifestID := uuid.New()
	stepsJSON, _ := marshalJSON(model.DefaultReleaseSteps(model.Normal, model.ReleaseUpgrade), "[]")

	_, err := store.DB().ExecContext(context.Background(), `
		insert into releases (id, application_id, manifest_id, env, strategy, type, steps, status, created_at, updated_at, deleted_at)
		values ($1,$2,$3,'staging','rolling','Upgrade',$4,'Syncing',$5,$6,null)
	`, releaseID.String(), appID.String(), manifestID.String(), stepsJSON, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	svc := &releaseService{}
	release, err := svc.Get(context.Background(), releaseID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}

	if err := svc.persistArgoApplicationMetadata(context.Background(), release, "demo-api"); err != nil {
		t.Fatalf("persistArgoApplicationMetadata failed: %v", err)
	}

	updated, err := svc.Get(context.Background(), releaseID)
	if err != nil {
		t.Fatalf("get updated failed: %v", err)
	}
	if updated.ArgoCDApplicationName != "demo-api" {
		t.Fatalf("argocd_application_name = %q", updated.ArgoCDApplicationName)
	}
	if updated.ExternalRef != "demo-api" {
		t.Fatalf("external_ref = %q", updated.ExternalRef)
	}
}

func TestPublishDeploymentBundleRecordsArtifactMetadataWhenRegistryEnabled(t *testing.T) {
	setupTestDB(t)
	originalRuntimeConfig := releasesupport.CurrentRuntimeConfig()
	releasesupport.ConfigureRuntimeConfig(releasesupport.RuntimeConfig{
		ManifestRegistryEnabled: true,
		ManifestRegistry: manifestdomain.ManifestRegistryConfig{
			Registry:   "registry.example.com",
			Namespace:  "devflow",
			Repository: "releases",
		},
	})
	t.Cleanup(func() { releasesupport.ConfigureRuntimeConfig(originalRuntimeConfig) })

	releaseID := uuid.New()
	appID := uuid.New()
	manifestID := uuid.New()
	stepsJSON, _ := marshalJSON(model.DefaultReleaseSteps(model.Normal, model.ReleaseUpgrade), "[]")

	_, err := store.DB().ExecContext(context.Background(), `
		insert into releases (id, application_id, manifest_id, env, strategy, type, steps, status, created_at, updated_at, deleted_at)
		values ($1,$2,$3,'staging','rolling','Upgrade',$4,'Syncing',$5,$6,null)
	`, releaseID.String(), appID.String(), manifestID.String(), stepsJSON, time.Now(), time.Now())
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
			{Name: "demo-api", Ports: []manifestdomain.ManifestServicePort{{Name: "http", ServicePort: 80, TargetPort: 8080}}},
		},
		WorkloadConfigSnapshot: manifestdomain.ManifestWorkloadConfig{Replicas: 1},
	}
	if err := svc.publishDeploymentBundle(context.Background(), release, manifest, &releasesupport.ApplicationProjection{Name: "demo-api"}, releasesupport.DeployTarget{Namespace: "checkout"}); err != nil {
		t.Fatalf("publishDeploymentBundle failed: %v", err)
	}

	updated, err := svc.Get(context.Background(), releaseID)
	if err != nil {
		t.Fatalf("get updated failed: %v", err)
	}
	if updated.ArtifactRepository != "registry.example.com/devflow/releases/demo-api" {
		t.Fatalf("artifact_repository = %q", updated.ArtifactRepository)
	}
	if updated.ArtifactTag != releaseID.String() {
		t.Fatalf("artifact_tag = %q", updated.ArtifactTag)
	}
	if updated.ArtifactDigest == "" {
		t.Fatal("expected artifact_digest to be set")
	}
	if updated.ArtifactRef != "oci://registry.example.com/devflow/releases/demo-api@"+updated.ArtifactDigest {
		t.Fatalf("artifact_ref = %q", updated.ArtifactRef)
	}
}

func TestPublishDeploymentBundleMarksStepWhenRegistryDisabled(t *testing.T) {
	setupTestDB(t)
	originalRuntimeConfig := releasesupport.CurrentRuntimeConfig()
	releasesupport.ConfigureRuntimeConfig(releasesupport.RuntimeConfig{ManifestRegistryEnabled: false})
	t.Cleanup(func() { releasesupport.ConfigureRuntimeConfig(originalRuntimeConfig) })

	releaseID := uuid.New()
	appID := uuid.New()
	manifestID := uuid.New()
	stepsJSON, _ := marshalJSON(model.DefaultReleaseSteps(model.Normal, model.ReleaseUpgrade), "[]")

	_, err := store.DB().ExecContext(context.Background(), `
		insert into releases (id, application_id, manifest_id, env, strategy, type, steps, status, created_at, updated_at, deleted_at)
		values ($1,$2,$3,'staging','rolling','Upgrade',$4,'Syncing',$5,$6,null)
	`, releaseID.String(), appID.String(), manifestID.String(), stepsJSON, time.Now(), time.Now())
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
			{Name: "demo-api", Ports: []manifestdomain.ManifestServicePort{{Name: "http", ServicePort: 80, TargetPort: 8080}}},
		},
		WorkloadConfigSnapshot: manifestdomain.ManifestWorkloadConfig{Replicas: 1},
	}
	if err := svc.publishDeploymentBundle(context.Background(), release, manifest, &releasesupport.ApplicationProjection{Name: "demo-api"}, releasesupport.DeployTarget{Namespace: "checkout"}); err != nil {
		t.Fatalf("publishDeploymentBundle failed: %v", err)
	}

	updated, err := svc.Get(context.Background(), releaseID)
	if err != nil {
		t.Fatalf("get updated failed: %v", err)
	}
	found := false
	for _, step := range updated.Steps {
		if step.Code == "publish_bundle" {
			found = true
			if step.Status != model.StepSucceeded {
				t.Fatalf("publish_bundle status = %q", step.Status)
			}
		}
	}
	if !found {
		t.Fatal("publish_bundle step not found")
	}
}

func TestPublishDeploymentBundleMarksStepFailedWhenPublisherFails(t *testing.T) {
	setupTestDB(t)
	originalRuntimeConfig := releasesupport.CurrentRuntimeConfig()
	releasesupport.ConfigureRuntimeConfig(releasesupport.RuntimeConfig{
		ManifestRegistryEnabled: true,
		ManifestRegistry: manifestdomain.ManifestRegistryConfig{
			Registry:   "registry.example.com",
			Namespace:  "devflow",
			Repository: "releases",
		},
	})
	originalPublisher := releaseBundlePublisherImpl
	releaseBundlePublisherImpl = releaseBundlePublisherFunc(func(context.Context, ReleaseBundlePublishRequest) (*ReleaseBundlePublishResult, error) {
		return nil, errors.New("publisher failed")
	})
	t.Cleanup(func() {
		releasesupport.ConfigureRuntimeConfig(originalRuntimeConfig)
		releaseBundlePublisherImpl = originalPublisher
	})

	releaseID := uuid.New()
	appID := uuid.New()
	manifestID := uuid.New()
	stepsJSON, _ := marshalJSON(model.DefaultReleaseSteps(model.Normal, model.ReleaseUpgrade), "[]")

	_, err := store.DB().ExecContext(context.Background(), `
		insert into releases (id, application_id, manifest_id, env, strategy, type, steps, status, created_at, updated_at, deleted_at)
		values ($1,$2,$3,'staging','rolling','Upgrade',$4,'Syncing',$5,$6,null)
	`, releaseID.String(), appID.String(), manifestID.String(), stepsJSON, time.Now(), time.Now())
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
			{Name: "demo-api", Ports: []manifestdomain.ManifestServicePort{{Name: "http", ServicePort: 80, TargetPort: 8080}}},
		},
		WorkloadConfigSnapshot: manifestdomain.ManifestWorkloadConfig{Replicas: 1},
	}
	err = svc.publishDeploymentBundle(context.Background(), release, manifest, &releasesupport.ApplicationProjection{Name: "demo-api"}, releasesupport.DeployTarget{Namespace: "checkout"})
	if err == nil || err.Error() != "publisher failed" {
		t.Fatalf("err = %v", err)
	}

	updated, err := svc.Get(context.Background(), releaseID)
	if err != nil {
		t.Fatalf("get updated failed: %v", err)
	}
	found := false
	for _, step := range updated.Steps {
		if step.Code == "publish_bundle" {
			found = true
			if step.Status != model.StepFailed || step.Message != "publisher failed" {
				t.Fatalf("unexpected publish_bundle step: %+v", step)
			}
		}
	}
	if !found {
		t.Fatal("publish_bundle step not found")
	}
}

func TestPublishDeploymentBundleUsesOrasPublisherMode(t *testing.T) {
	setupTestDB(t)
	originalRuntimeConfig := releasesupport.CurrentRuntimeConfig()
	originalNewRemoteRepository := newOrasRemoteRepository
	originalOrasCopy := orasCopy
	var gotRepository string
	var gotPlainHTTP bool
	var gotCredential auth.Credential
	var gotDstRef string
	newOrasRemoteRepository = func(repository string) (*remote.Repository, error) {
		gotRepository = repository
		return &remote.Repository{}, nil
	}
	orasCopy = func(_ context.Context, _ oras.ReadOnlyTarget, _ string, dst oras.Target, dstRef string, _ oras.CopyOptions) (ocispec.Descriptor, error) {
		gotDstRef = dstRef
		repo, ok := dst.(*remote.Repository)
		if !ok {
			t.Fatalf("dst type = %T", dst)
		}
		gotPlainHTTP = repo.PlainHTTP
		client, ok := repo.Client.(*auth.Client)
		if !ok {
			t.Fatalf("repo.Client type = %T", repo.Client)
		}
		cred, err := client.Credential(context.Background(), "registry.example.com")
		if err != nil {
			t.Fatalf("credential err = %v", err)
		}
		gotCredential = cred
		return ocispec.Descriptor{
			MediaType: releaseBundleArtifactType,
			Digest:    "sha256:remote",
			Size:      123,
		}, nil
	}
	releasesupport.ConfigureRuntimeConfig(releasesupport.RuntimeConfig{
		ManifestRegistryEnabled: true,
		ManifestPublisherMode:   "oras",
		ManifestRegistry: manifestdomain.ManifestRegistryConfig{
			Registry:   "registry.example.com",
			Namespace:  "devflow",
			Repository: "releases",
			Username:   "robot",
			Password:   "secret",
			PlainHTTP:  true,
		},
	})
	t.Cleanup(func() {
		releasesupport.ConfigureRuntimeConfig(originalRuntimeConfig)
		newOrasRemoteRepository = originalNewRemoteRepository
		orasCopy = originalOrasCopy
	})

	releaseID := uuid.New()
	appID := uuid.New()
	manifestID := uuid.New()
	stepsJSON, _ := marshalJSON(model.DefaultReleaseSteps(model.Normal, model.ReleaseUpgrade), "[]")

	_, err := store.DB().ExecContext(context.Background(), `
		insert into releases (id, application_id, manifest_id, env, strategy, type, steps, status, created_at, updated_at, deleted_at)
		values ($1,$2,$3,'staging','rolling','Upgrade',$4,'Syncing',$5,$6,null)
	`, releaseID.String(), appID.String(), manifestID.String(), stepsJSON, time.Now(), time.Now())
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
			{Name: "demo-api", Ports: []manifestdomain.ManifestServicePort{{Name: "http", ServicePort: 80, TargetPort: 8080}}},
		},
		WorkloadConfigSnapshot: manifestdomain.ManifestWorkloadConfig{Replicas: 1},
	}
	if err := svc.publishDeploymentBundle(context.Background(), release, manifest, &releasesupport.ApplicationProjection{Name: "demo-api"}, releasesupport.DeployTarget{Namespace: "checkout"}); err != nil {
		t.Fatalf("publishDeploymentBundle failed: %v", err)
	}

	updated, err := svc.Get(context.Background(), releaseID)
	if err != nil {
		t.Fatalf("get updated failed: %v", err)
	}
	if updated.ArtifactDigest == "" {
		t.Fatal("expected artifact_digest to be set")
	}
	if updated.ArtifactRef != "oci://registry.example.com/devflow/releases/demo-api@"+updated.ArtifactDigest {
		t.Fatalf("artifact_ref = %q", updated.ArtifactRef)
	}
	if updated.ArtifactDigest != "sha256:remote" {
		t.Fatalf("artifact_digest = %q", updated.ArtifactDigest)
	}
	if gotRepository != "registry.example.com/devflow/releases/demo-api" {
		t.Fatalf("remote repository = %q", gotRepository)
	}
	if gotDstRef != releaseID.String() {
		t.Fatalf("dstRef = %q", gotDstRef)
	}
	if !gotPlainHTTP {
		t.Fatal("expected PlainHTTP to be true")
	}
	if gotCredential.Username != "robot" || gotCredential.Password != "secret" {
		t.Fatalf("credential = %+v", gotCredential)
	}
	found := false
	for _, step := range updated.Steps {
		if step.Code == "publish_bundle" {
			found = true
			if step.Message != "deployment bundle published via oras publisher: oci://registry.example.com/devflow/releases/demo-api@sha256:remote" {
				t.Fatalf("publish_bundle message = %q", step.Message)
			}
		}
	}
	if !found {
		t.Fatal("publish_bundle step not found")
	}
}

func TestBuildOrasRemoteRepositoryWithoutCredentials(t *testing.T) {
	originalNewRemoteRepository := newOrasRemoteRepository
	newOrasRemoteRepository = func(repository string) (*remote.Repository, error) {
		return &remote.Repository{}, nil
	}
	t.Cleanup(func() { newOrasRemoteRepository = originalNewRemoteRepository })

	repo, err := buildOrasRemoteRepository(manifestdomain.ManifestRegistryConfig{
		Registry:  "registry.example.com",
		PlainHTTP: true,
	}, "registry.example.com/devflow/releases/demo-api")
	if err != nil {
		t.Fatal(err)
	}
	if !repo.PlainHTTP {
		t.Fatal("expected PlainHTTP to be true")
	}
	if repo.Client != nil {
		t.Fatalf("expected nil client without credentials, got %T", repo.Client)
	}
}

func TestPublishBundleMessageHelpers(t *testing.T) {
	start := publishBundleStartMessage(releasesupport.RuntimeConfig{ManifestPublisherMode: "oras"})
	if start != "publishing deployment bundle via oras publisher" {
		t.Fatalf("start = %q", start)
	}
	message := publishBundleResultMessage(releasesupport.RuntimeConfig{ManifestPublisherMode: "oras"}, &ReleaseBundlePublishResult{
		Repository: "registry.example.com/devflow/releases/demo-api",
		Tag:        "release-1",
		Digest:     "sha256:abc",
		Ref:        "oci://registry.example.com/devflow/releases/demo-api@sha256:abc",
	})
	if message != "deployment bundle published via oras publisher: oci://registry.example.com/devflow/releases/demo-api@sha256:abc" {
		t.Fatalf("message = %q", message)
	}
	fallback := publishBundleResultMessage(releasesupport.RuntimeConfig{}, &ReleaseBundlePublishResult{
		Message: "bundle published metadata recorded",
	})
	if fallback != "bundle published metadata recorded" {
		t.Fatalf("fallback = %q", fallback)
	}
}

func TestCreateArgoApplicationMessageHelpers(t *testing.T) {
	release := &model.Release{
		EnvironmentID: "production",
		ArtifactRef:   "oci://registry.example.com/devflow/releases/demo-api@sha256:abc",
	}
	start := createArgoApplicationStartMessage(release, "demo-api", releasesupport.DeployTarget{Namespace: "checkout"})
	if start != "creating argocd application demo-api for environment production in namespace checkout" {
		t.Fatalf("start = %q", start)
	}
	success := createArgoApplicationSuccessMessage(release, "demo-api")
	if success != "argocd application demo-api created for environment production and sync requested from oci://registry.example.com/devflow/releases/demo-api@sha256:abc" {
		t.Fatalf("success = %q", success)
	}
	failure := createArgoApplicationFailureMessage("demo-api", errors.New("sync denied"))
	if failure != "argocd application demo-api failed: sync denied" {
		t.Fatalf("failure = %q", failure)
	}
}

type releaseBundlePublisherFunc func(context.Context, ReleaseBundlePublishRequest) (*ReleaseBundlePublishResult, error)

func (f releaseBundlePublisherFunc) PublishBundle(ctx context.Context, req ReleaseBundlePublishRequest) (*ReleaseBundlePublishResult, error) {
	return f(ctx, req)
}

func TestReleaseDeploymentStartStep(t *testing.T) {
	tests := []struct {
		name        string
		release     *model.Release
		wantCode    string
		wantMessage string
	}{
		{
			name:        "rolling default",
			release:     &model.Release{Strategy: "rolling"},
			wantCode:    "start_deployment",
			wantMessage: "deployment sync started",
		},
		{
			name:        "blue green",
			release:     &model.Release{Strategy: "blueGreen"},
			wantCode:    "deploy_preview",
			wantMessage: "preview deployment started",
		},
		{
			name:        "canary",
			release:     &model.Release{Strategy: "canary"},
			wantCode:    "deploy_canary",
			wantMessage: "canary deployment started",
		},
		{
			name:        "nil release",
			release:     nil,
			wantCode:    "",
			wantMessage: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCode, gotMessage := releaseDeploymentStartStep(tt.release)
			if gotCode != tt.wantCode {
				t.Fatalf("code = %q want %q", gotCode, tt.wantCode)
			}
			if gotMessage != tt.wantMessage {
				t.Fatalf("message = %q want %q", gotMessage, tt.wantMessage)
			}
		})
	}
}

var _ *appv1.Application
