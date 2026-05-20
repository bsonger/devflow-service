package service

import (
	"context"
	"fmt"
	"strings"

	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	releasesupport "github.com/bsonger/devflow-service/internal/release/support"
	"github.com/google/uuid"
)

// renderDeploymentBundle materializes the release-owned deployable payload and persists it for preview/publication.
// This is intentionally distinct from manifest resource inspection, which never includes release-only config or routing inputs.
func (s *releaseService) renderDeploymentBundle(ctx context.Context, release *model.Release, manifest *manifestdomain.Manifest, app *releasesupport.ApplicationProjection, target releasesupport.DeployTarget) error {
	if err := s.UpdateStep(ctx, release.ID, "render_deployment_bundle", model.StepRunning, 25, "rendering deployment bundle", nil, nil); err != nil {
		return err
	}
	applicationName := ""
	if app != nil {
		applicationName = app.Name
	}
	bundle, err := buildReleaseBundle(target.Namespace, applicationName, target.EnvironmentName, manifest, release)
	if err != nil {
		_ = s.UpdateStep(ctx, release.ID, "render_deployment_bundle", model.StepFailed, 100, err.Error(), nil, nil)
		return err
	}
	record := newReleaseBundleRecord(bundle)
	if err := s.repoBundleStore().Insert(ctx, record); err != nil {
		_ = s.UpdateStep(ctx, release.ID, "render_deployment_bundle", model.StepFailed, 100, err.Error(), nil, nil)
		return err
	}
	message := fmt.Sprintf("deployment bundle rendered (%d resources, %d files)", len(bundle.RenderedObjects), len(bundle.Files))
	return s.UpdateStep(ctx, release.ID, "render_deployment_bundle", model.StepSucceeded, 100, message, nil, nil)
}

// publishDeploymentBundle publishes the persisted release bundle artifact for downstream deploy consumers.
// It reads the release bundle store rather than reusing manifest inspection resources so deploy diagnostics stay anchored to release-owned output.
func (s *releaseService) publishDeploymentBundle(ctx context.Context, release *model.Release, manifest *manifestdomain.Manifest, app *releasesupport.ApplicationProjection, target releasesupport.DeployTarget) error {
	runtimeCfg := releasesupport.CurrentRuntimeConfig()
	if err := s.UpdateStep(ctx, release.ID, "publish_bundle", model.StepRunning, 25, publishBundleStartMessage(runtimeCfg), nil, nil); err != nil {
		return err
	}
	bundleRecord, err := s.repoBundleStore().GetByReleaseID(ctx, release.ID)
	if err != nil {
		_ = s.UpdateStep(ctx, release.ID, "publish_bundle", model.StepFailed, 100, err.Error(), nil, nil)
		return err
	}
	if !runtimeCfg.ManifestRegistryEnabled {
		return s.UpdateStep(ctx, release.ID, "publish_bundle", model.StepSucceeded, 100, "bundle publication skipped; manifest registry disabled", nil, nil)
	}
	bundle := buildReleaseBundleFromRecord(release, bundleRecord)
	publisher := resolveReleaseBundlePublisher(runtimeCfg)
	result, err := publisher.PublishBundle(ctx, ReleaseBundlePublishRequest{
		Release:        release,
		Application:    app,
		Bundle:         bundle,
		RegistryConfig: runtimeCfg.ManifestRegistry,
	})
	if err != nil {
		_ = s.UpdateStep(ctx, release.ID, "publish_bundle", model.StepFailed, 100, err.Error(), nil, nil)
		return err
	}
	release.ArtifactRepository = strings.TrimSpace(result.Repository)
	release.ArtifactTag = strings.TrimSpace(result.Tag)
	release.ArtifactDigest = strings.TrimSpace(result.Digest)
	release.ArtifactRef = strings.TrimSpace(result.Ref)
	return s.UpdateArtifact(ctx, release.ID, result.Repository, result.Tag, result.Digest, result.Ref, publishBundleResultMessage(runtimeCfg, result), model.StepSucceeded, 100)
}

func deriveReleaseArtifactMetadata(release *model.Release, app *releasesupport.ApplicationProjection, cfg manifestdomain.ManifestRegistryConfig) (repository, tag, ref string) {
	applicationName := ""
	if app != nil {
		applicationName = app.Name
	}
	repository = cfg.RepositoryFor(applicationName, releaseTargetEnvironment(release))
	tag = "latest"
	if release != nil && release.ID != uuid.Nil {
		tag = release.ID.String()
	}
	if release != nil && release.ID == uuid.Nil {
		tag = "latest"
	}
	if repository == "" {
		return "", tag, ""
	}
	return repository, tag, "oci://" + repository + ":" + tag
}

func deriveReleaseArtifactMetadataFromBundle(release *model.Release, app *releasesupport.ApplicationProjection, cfg manifestdomain.ManifestRegistryConfig, bundle *model.ReleaseBundle) (repository, tag, digest, ref string) {
	applicationName := ""
	if app != nil {
		applicationName = app.Name
	}
	repository = cfg.RepositoryFor(applicationName, releaseTargetEnvironment(release))
	tag = "latest"
	if release != nil && release.ID != uuid.Nil {
		tag = release.ID.String()
	}
	if release != nil && release.ID == uuid.Nil {
		tag = "latest"
	}
	digest = releaseBundleDigest(bundle)
	if repository == "" {
		return "", tag, digest, ""
	}
	if digest != "" {
		return repository, tag, digest, "oci://" + repository + "@" + digest
	}
	return repository, tag, "", "oci://" + repository + ":" + tag
}

func publishBundleStartMessage(runtimeCfg releasesupport.RuntimeConfig) string {
	mode := strings.TrimSpace(runtimeCfg.ManifestPublisherMode)
	if mode == "" {
		mode = "metadata"
	}
	return fmt.Sprintf("publishing deployment bundle via %s publisher", mode)
}

func publishBundleResultMessage(runtimeCfg releasesupport.RuntimeConfig, result *ReleaseBundlePublishResult) string {
	mode := strings.TrimSpace(runtimeCfg.ManifestPublisherMode)
	if mode == "" {
		mode = "metadata"
	}
	if result == nil {
		return fmt.Sprintf("deployment bundle published via %s publisher", mode)
	}
	switch {
	case strings.TrimSpace(result.Ref) != "":
		return fmt.Sprintf("deployment bundle published via %s publisher: %s", mode, strings.TrimSpace(result.Ref))
	case strings.TrimSpace(result.Repository) != "" && strings.TrimSpace(result.Tag) != "":
		return fmt.Sprintf("deployment bundle published via %s publisher: oci://%s:%s", mode, strings.TrimSpace(result.Repository), strings.TrimSpace(result.Tag))
	case strings.TrimSpace(result.Message) != "":
		return strings.TrimSpace(result.Message)
	default:
		return fmt.Sprintf("deployment bundle published via %s publisher", mode)
	}
}
