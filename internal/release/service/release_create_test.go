package service

import (
	"context"
	"testing"

	appservicedownstream "github.com/bsonger/devflow-service/internal/appservice/transport/downstream"
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
	manifestID := uuid.New()
	appID := uuid.New()
	release := &model.Release{ManifestID: manifestID, EnvironmentID: "staging"}

	populateReleaseDefaults(release, appID, "prod")

	if release.EnvironmentID != "staging" {
		t.Fatalf("got env %s want staging", release.EnvironmentID)
	}
	if release.Strategy != string(model.ReleaseStrategyRolling) {
		t.Fatalf("got strategy %s want %s", release.Strategy, model.ReleaseStrategyRolling)
	}
}

func TestPopulateReleaseDefaultsFallsBackToProd(t *testing.T) {
	manifestID := uuid.New()
	appID := uuid.New()
	release := &model.Release{ManifestID: manifestID}

	populateReleaseDefaults(release, appID, "prod")

	if release.EnvironmentID != "prod" {
		t.Fatalf("got env %s want prod", release.EnvironmentID)
	}
	if release.Type != model.ReleaseUpgrade {
		t.Fatalf("got type %s want %s", release.Type, model.ReleaseUpgrade)
	}
	if release.Strategy != string(model.ReleaseStrategyRolling) {
		t.Fatalf("got strategy %s want %s", release.Strategy, model.ReleaseStrategyRolling)
	}
}

func TestPopulateReleaseDefaultsPreservesManifestID(t *testing.T) {
	manifestID := uuid.New()
	appID := uuid.New()
	release := &model.Release{ManifestID: manifestID}

	populateReleaseDefaults(release, appID, "prod")

	if release.ManifestID != manifestID {
		t.Fatalf("got manifest id %s want %s", release.ManifestID, manifestID)
	}
}

func TestCreateReleaseRejectsManifestThatIsNotReady(t *testing.T) {
	originalManifestSource := releaseManifestSource
	releaseManifestSource = stubReleaseManifestReader{
		getFn: func(_ context.Context, id uuid.UUID) (*manifestdomain.Manifest, error) {
			return &manifestdomain.Manifest{
				BaseModel:     model.BaseModel{ID: id},
				ApplicationID: uuid.New(),
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

func TestReleaseTargetEnvironmentUsesReleaseEnvironmentOnly(t *testing.T) {
	release := &model.Release{EnvironmentID: "  staging "}

	got := releaseTargetEnvironment(release)

	if got != "staging" {
		t.Fatalf("releaseTargetEnvironment() = %q want staging", got)
	}
}

func TestSelectReleaseRoutesIncludesBaseAndTargetEnvironment(t *testing.T) {
	routes := []appservicedownstream.Route{
		{Name: "base", EnvironmentID: "base"},
		{Name: "target", EnvironmentID: "staging"},
		{Name: "other", EnvironmentID: "prod"},
		{Name: "empty", EnvironmentID: ""},
	}

	got := selectReleaseRoutes(routes, "staging")

	if len(got) != 3 {
		t.Fatalf("len(selectReleaseRoutes) = %d want 3", len(got))
	}
	if got[0].Name != "base" || got[1].Name != "target" || got[2].Name != "empty" {
		t.Fatalf("unexpected selected routes: %+v", got)
	}
}
