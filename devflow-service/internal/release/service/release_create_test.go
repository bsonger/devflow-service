package service

import (
	"context"
	"testing"

	imagedomain "github.com/bsonger/devflow-service/internal/image/domain"
	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/google/uuid"
)

type stubReleaseManifestReader struct {
	getFn func(context.Context, uuid.UUID) (*manifestdomain.Manifest, error)
}

func (s stubReleaseManifestReader) Get(ctx context.Context, id uuid.UUID) (*manifestdomain.Manifest, error) {
	return s.getFn(ctx, id)
}

func TestPopulateReleaseDefaultsPreservesProvidedEnv(t *testing.T) {
	imageID := uuid.New()
	manifestID := uuid.New()
	appID := uuid.New()
	release := &model.Release{ManifestID: manifestID, ImageID: imageID, Env: "staging"}
	image := &imagedomain.Image{BaseModel: model.BaseModel{ID: imageID}, Name: "demo-main", ApplicationID: appID}

	populateReleaseDefaults(release, image, "prod")

	if release.Env != "staging" {
		t.Fatalf("got env %s want staging", release.Env)
	}
}

func TestPopulateReleaseDefaultsFallsBackToProd(t *testing.T) {
	imageID := uuid.New()
	manifestID := uuid.New()
	appID := uuid.New()
	release := &model.Release{ManifestID: manifestID, ImageID: imageID}
	image := &imagedomain.Image{BaseModel: model.BaseModel{ID: imageID}, Name: "demo-main", ApplicationID: appID}

	populateReleaseDefaults(release, image, "prod")

	if release.Env != "prod" {
		t.Fatalf("got env %s want prod", release.Env)
	}
	if release.Type != model.ReleaseUpgrade {
		t.Fatalf("got type %s want %s", release.Type, model.ReleaseUpgrade)
	}
}

func TestPopulateReleaseDefaultsPreservesManifestID(t *testing.T) {
	imageID := uuid.New()
	manifestID := uuid.New()
	appID := uuid.New()
	release := &model.Release{ManifestID: manifestID, ImageID: imageID}
	image := &imagedomain.Image{BaseModel: model.BaseModel{ID: imageID}, Name: "demo-main", ApplicationID: appID}

	populateReleaseDefaults(release, image, "prod")

	if release.ManifestID != manifestID {
		t.Fatalf("got manifest id %s want %s", release.ManifestID, manifestID)
	}
}

func TestResolveReleaseEnvironmentRequiresImageRuntimeSpecRevisionID(t *testing.T) {
	svc := &releaseService{}
	image := &imagedomain.Image{ApplicationID: uuid.New()}
	release := &model.Release{
		ManifestID: uuid.New(),
		ImageID:    uuid.New(),
	}

	_, err := svc.resolveReleaseEnvironment(context.Background(), release, image)
	if err == nil {
		t.Fatalf("expected error when image runtime_spec_revision_id is missing")
	}
}

func TestResolveReleaseEnvironmentFallsBackToReleaseEnvWhenRuntimeSpecRevisionIDMissing(t *testing.T) {
	svc := &releaseService{}
	image := &imagedomain.Image{ApplicationID: uuid.New()}
	release := &model.Release{
		ManifestID: uuid.New(),
		ImageID:    uuid.New(),
		Env:        "staging",
	}

	env, err := svc.resolveReleaseEnvironment(context.Background(), release, image)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env != "staging" {
		t.Fatalf("env = %q, want staging", env)
	}
}

func TestCreateReleaseRejectsManifestThatIsNotReady(t *testing.T) {
	originalManifestSource := releaseManifestSource
	releaseManifestSource = stubReleaseManifestReader{
		getFn: func(_ context.Context, id uuid.UUID) (*manifestdomain.Manifest, error) {
			return &manifestdomain.Manifest{
				BaseModel:     model.BaseModel{ID: id},
				ApplicationID: uuid.New(),
				EnvironmentID: "staging",
				ImageID:       uuid.New(),
				Status:        model.ManifestPending,
			}, nil
		},
	}
	defer func() { releaseManifestSource = originalManifestSource }()

	svc := &releaseService{}
	_, err := svc.Create(context.Background(), &model.Release{
		ManifestID: uuid.New(),
		Type:       model.ReleaseUpgrade,
	})
	if err == nil {
		t.Fatalf("expected manifest not ready error")
	}
	if err != ErrReleaseManifestNotReady {
		t.Fatalf("got err %v want %v", err, ErrReleaseManifestNotReady)
	}
}
