package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	argoutil "github.com/argoproj/argo-cd/v3/util/argo"
	imagedomain "github.com/bsonger/devflow-service/internal/image/domain"
	imageservice "github.com/bsonger/devflow-service/internal/image/service"
	intentservice "github.com/bsonger/devflow-service/internal/intent/service"
	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	manifestservice "github.com/bsonger/devflow-service/internal/manifest/service"
	"github.com/bsonger/devflow-service/internal/platform/logger"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/bsonger/devflow-service/internal/release/repository"
	"github.com/bsonger/devflow-service/internal/release/runtime"
	releasesupport "github.com/bsonger/devflow-service/internal/release/support"
	"github.com/bsonger/devflow-service/internal/release/transport/argo"
	runtimeclient "github.com/bsonger/devflow-service/internal/release/transport/runtime"
	sharederrs "github.com/bsonger/devflow-service/internal/shared/errs"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ReleaseListFilter struct {
	IncludeDeleted bool
	ApplicationID  *uuid.UUID
	ManifestID     *uuid.UUID
	ImageID        *uuid.UUID
	Status         string
	Type           string
}

var ReleaseService = &releaseService{store: repository.NewPostgresStore()}

var (
	ErrImageMissingRuntimeSpecRevision                      = sharederrs.FailedPrecondition("image runtime_spec_revision_id is required")
	ErrRuntimeSpecBindingMismatch                           = sharederrs.FailedPrecondition("image runtime_spec_revision_id does not match release application and env")
	ErrReleaseManifestNotReady                              = sharederrs.FailedPrecondition("manifest is not ready")
	runtimeLookupClient                runtimeclient.Lookup = runtimeclient.New("")
)

type releaseManifestReader interface {
	Get(context.Context, uuid.UUID) (*manifestdomain.Manifest, error)
}

var releaseManifestSource releaseManifestReader = manifestservice.ManifestService

type releaseService struct {
	store repository.Store
}

func (s *releaseService) repoStore() repository.Store {
	if s.store == nil {
		s.store = repository.NewPostgresStore()
	}
	return s.store
}

func SetRuntimeClient(client runtimeclient.Lookup) {
	runtimeLookupClient = client
}

func populateReleaseDefaults(release *model.Release, image *imagedomain.Image, env string) {
	release.ApplicationID = image.ApplicationID
	if release.Type == "" {
		release.Type = model.ReleaseUpgrade
	}
	if release.Env == "" {
		release.Env = env
	}
	release.Status = model.ReleasePending
	if len(release.Steps) == 0 {
		release.Steps = model.DefaultReleaseSteps(model.Normal, release.Type)
	}
}

func (s *releaseService) resolveReleaseEnvironment(ctx context.Context, release *model.Release, image *imagedomain.Image) (string, error) {
	if image.RuntimeSpecRevisionID == nil || *image.RuntimeSpecRevisionID == uuid.Nil {
		if strings.TrimSpace(release.Env) != "" {
			return strings.TrimSpace(release.Env), nil
		}
		return "", ErrImageMissingRuntimeSpecRevision
	}
	revision, err := runtimeLookupClient.GetRuntimeSpecRevision(ctx, *image.RuntimeSpecRevisionID)
	if err != nil {
		return "", err
	}
	spec, err := runtimeLookupClient.GetRuntimeSpec(ctx, revision.RuntimeSpecID)
	if err != nil {
		return "", err
	}
	if spec.ApplicationID != image.ApplicationID {
		return "", fmt.Errorf("%w: spec application=%s image application=%s", ErrRuntimeSpecBindingMismatch, spec.ApplicationID, image.ApplicationID)
	}
	if release.Env != "" && release.Env != spec.Environment {
		return "", fmt.Errorf("%w: spec env=%s release env=%s", ErrRuntimeSpecBindingMismatch, spec.Environment, release.Env)
	}
	return spec.Environment, nil
}

func (s *releaseService) Create(ctx context.Context, release *model.Release) (uuid.UUID, error) {
	log := logger.LoggerWithContext(ctx)
	if log == nil {
		log = zap.NewNop()
	}
	log = log.With(
		zap.String("operation", "create_release"),
		zap.String("resource", "release"),
		zap.String("result", "started"),
		zap.String("release_type", release.Type),
		zap.String("manifest_id", release.ManifestID.String()),
	)

	manifest, err := releaseManifestSource.Get(ctx, release.ManifestID)
	if err != nil {
		return uuid.Nil, err
	}
	if manifest.Status != model.ManifestReady {
		return uuid.Nil, ErrReleaseManifestNotReady
	}
	release.ApplicationID = manifest.ApplicationID
	release.ImageID = manifest.ImageID
	if strings.TrimSpace(release.Env) == "" {
		release.Env = strings.TrimSpace(manifest.EnvironmentID)
	}

	image, err := imageservice.ImageService.Get(ctx, release.ImageID)
	if err != nil {
		return uuid.Nil, err
	}
	env, err := s.resolveReleaseEnvironment(ctx, release, image)
	if err != nil {
		return uuid.Nil, err
	}

	populateReleaseDefaults(release, image, env)
	release.WithCreateDefault()
	annotateReleaseSpan(ctx, release)
	if err := s.repoStore().Insert(ctx, release); err != nil {
		return uuid.Nil, err
	}
	observeReleaseCreated(ctx, release)

	log = log.With(
		zap.String("release_id", release.ID.String()),
		zap.String("application_id", release.ApplicationID.String()),
		zap.String("manifest_id", release.ManifestID.String()),
	)

	if runtime.IsIntentMode() {
		intentID, err := intentservice.IntentService.CreateReleaseIntent(ctx, release)
		if err != nil {
			return release.ID, err
		}
		log.Info("release accepted in intent mode",
			zap.String("resource_id", release.ID.String()),
			zap.String("result", "success"),
			zap.String("intent_id", intentID.String()),
		)
		return release.ID, nil
	}

	if err := s.DispatchRelease(ctx, release.ID); err != nil {
		s.handleSyncArgoError(ctx, release, err)
		return release.ID, err
	}
	return release.ID, nil
}

func (s *releaseService) DispatchRelease(ctx context.Context, releaseID uuid.UUID) error {
	release, err := s.Get(ctx, releaseID)
	if err != nil {
		return err
	}
	if err := s.updateStatus(ctx, release.ID, model.ReleaseSyncing); err != nil {
		return err
	}
	release.Status = model.ReleaseSyncing
	return s.syncArgo(ctx, release)
}

func (s *releaseService) handleSyncArgoError(ctx context.Context, release *model.Release, err error) {
	log := logger.LoggerWithContext(ctx).With(
		zap.String("operation", "sync_release"),
		zap.String("resource", "release"),
		zap.String("resource_id", release.ID.String()),
		zap.String("release_type", release.Type),
	)
	log.Error("sync argo failed", zap.String("result", "error"), zap.Error(err))
	_ = s.updateStatus(ctx, release.ID, model.ReleaseSyncFailed)
}

func (s *releaseService) Get(ctx context.Context, id uuid.UUID) (*model.Release, error) {
	return s.repoStore().Get(ctx, id)
}

func (s *releaseService) Update(ctx context.Context, release *model.Release) error {
	current, err := s.Get(ctx, release.ID)
	if err != nil {
		return err
	}
	release.CreatedAt = current.CreatedAt
	release.DeletedAt = current.DeletedAt
	release.WithUpdateDefault()
	return s.repoStore().UpdateRow(ctx, release)
}

func (s *releaseService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repoStore().Delete(ctx, id)
}

func (s *releaseService) List(ctx context.Context, filter ReleaseListFilter) ([]*model.Release, error) {
	return s.repoStore().List(ctx, repository.ListFilter(filter))
}

func (s *releaseService) updateStatus(ctx context.Context, releaseID uuid.UUID, status model.ReleaseStatus) error {
	release, err := s.Get(ctx, releaseID)
	if err != nil {
		return err
	}
	switch release.Status {
	case model.ReleaseSucceeded, model.ReleaseFailed, model.ReleaseRolledBack, model.ReleaseSyncFailed:
		return nil
	}
	if release.Status == status {
		return nil
	}
	previousStatus := release.Status
	release.Status = status
	release.UpdatedAt = time.Now()
	if err := s.repoStore().UpdateRow(ctx, release); err != nil {
		return err
	}
	statusLog := logger.LoggerWithContext(ctx)
	if statusLog == nil {
		statusLog = zap.NewNop()
	}
	statusLog.Info("release status updated",
		zap.String("operation", "update_release_status"),
		zap.String("resource", "release"),
		zap.String("resource_id", release.ID.String()),
		zap.String("result", "success"),
		zap.String("previous_status", string(previousStatus)),
		zap.String("status", string(status)),
	)
	observeReleaseTerminal(ctx, release, status)
	return nil
}

func (s *releaseService) UpdateStatus(ctx context.Context, releaseID uuid.UUID, status model.ReleaseStatus) error {
	return s.updateStatus(ctx, releaseID, status)
}

func (s *releaseService) UpdateStep(ctx context.Context, releaseID uuid.UUID, stepName string, status model.StepStatus, progress int32, message string, start, end *time.Time) error {
	if stepName == "" {
		return nil
	}
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	release, err := s.Get(ctx, releaseID)
	if err != nil {
		return err
	}
	nextSteps := cloneReleaseSteps(release.Steps)
	currentStep := findReleaseStep(release.Steps, stepName)
	if currentStep == nil {
		nextSteps = append(nextSteps, model.ReleaseStep{Name: stepName, Progress: progress, Status: status, Message: message, StartTime: start, EndTime: end})
		release.Steps = nextSteps
		release.UpdatedAt = time.Now()
		if err := s.repoStore().UpdateSteps(ctx, release); err != nil {
			return err
		}
		return s.updateStatusFromSteps(ctx, releaseID, release.Type, release.Status, nextSteps)
	}
	if currentStep.Status == model.StepFailed || currentStep.Status == model.StepSucceeded {
		return nil
	}
	applyReleaseStepUpdate(nextSteps, stepName, status, progress, message, start, end)
	release.Steps = nextSteps
	release.UpdatedAt = time.Now()
	if err := s.repoStore().UpdateSteps(ctx, release); err != nil {
		return err
	}
	return s.updateStatusFromSteps(ctx, releaseID, release.Type, release.Status, nextSteps)
}

func findReleaseStep(steps []model.ReleaseStep, stepName string) *model.ReleaseStep {
	for _, step := range steps {
		if step.Name == stepName {
			current := step
			return &current
		}
	}
	return nil
}

func cloneReleaseSteps(steps []model.ReleaseStep) []model.ReleaseStep {
	if len(steps) == 0 {
		return nil
	}
	cloned := make([]model.ReleaseStep, len(steps))
	copy(cloned, steps)
	return cloned
}

func applyReleaseStepUpdate(steps []model.ReleaseStep, stepName string, status model.StepStatus, progress int32, message string, start, end *time.Time) {
	for i := range steps {
		if steps[i].Name != stepName {
			continue
		}
		steps[i].Status = status
		steps[i].Progress = progress
		steps[i].Message = message
		if start != nil {
			steps[i].StartTime = start
		}
		if end != nil {
			steps[i].EndTime = end
		}
		return
	}
}

func (s *releaseService) updateStatusFromSteps(ctx context.Context, releaseID uuid.UUID, releaseAction string, currentStatus model.ReleaseStatus, steps []model.ReleaseStep) error {
	nextStatus := model.DeriveReleaseStatusFromSteps(releaseAction, currentStatus, steps)
	if nextStatus == currentStatus {
		return nil
	}
	return s.updateStatus(ctx, releaseID, nextStatus)
}

func (s *releaseService) syncArgo(ctx context.Context, release *model.Release) error {
	log := logger.LoggerWithContext(ctx)
	annotateReleaseSpan(ctx, release)
	log = log.With(
		zap.String("operation", "sync_release"),
		zap.String("resource", "release"),
		zap.String("resource_id", release.ID.String()),
		zap.String("release_type", release.Type),
	)
	app, err := releasesupport.ApplicationService.Get(ctx, release.ApplicationID)
	if err != nil {
		return err
	}
	manifest, err := releaseManifestSource.Get(ctx, release.ManifestID)
	if err != nil {
		return err
	}
	environmentID := strings.TrimSpace(release.Env)
	if manifest != nil && strings.TrimSpace(manifest.EnvironmentID) != "" {
		environmentID = strings.TrimSpace(manifest.EnvironmentID)
	}
	target, err := releasesupport.ResolveDeployTarget(ctx, release.ApplicationID.String(), environmentID)
	if err != nil {
		return err
	}

	// Run ordered bootstrap gates before Argo Application apply
	bootstrap, err := newBootstrapExecutor()
	if err != nil {
		log.Error("bootstrap executor creation failed", zap.String("result", "error"), zap.Error(err))
		return err
	}

	results, err := bootstrap.runBootstrapGates(ctx, *target, app.ProjectName)
	for _, res := range results {
		_ = s.UpdateStep(ctx, release.ID, res.StepName, res.Status, 100, res.Message, res.Start, res.End)
	}
	if err != nil {
		log.Error("bootstrap gates failed", zap.String("result", "error"), zap.Error(err))
		return err
	}

	application := buildArgoApplication(release, manifest, app, *target)
	sc := trace.SpanContextFromContext(ctx)
	application.Annotations = map[string]string{
		imagedomain.TraceIDAnnotation: sc.TraceID().String(),
		imagedomain.SpanAnnotation:    sc.SpanID().String(),
	}
	application.Labels = map[string]string{"status": string(model.ReleaseRunning), model.ReleaseIDLabel: release.ID.String()}

	err = applyReleaseApplication(ctx, release.Type, application, argoclient.CreateApplication, argoclient.UpdateApplication, s.syncArgoApplication)
	if err != nil {
		log.Error("argo sync failed", zap.String("result", "error"), zap.Error(err))
		return err
	}
	log.Info("argo sync completed", zap.String("result", "success"))
	return nil
}

func annotateReleaseSpan(ctx context.Context, release *model.Release) {
	if release == nil {
		return
	}
	span := trace.SpanFromContext(ctx)
	if span == nil {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String("release.id", release.ID.String()),
		attribute.String("release.type", release.Type),
		attribute.String("application.id", release.ApplicationID.String()),
		attribute.String("manifest.id", release.ManifestID.String()),
		attribute.String("deployment.environment", strings.TrimSpace(release.Env)),
	}
	if release.ImageID != uuid.Nil {
		attrs = append(attrs, attribute.String("image.id", release.ImageID.String()))
	}
	span.SetAttributes(attrs...)
}

func applyReleaseApplication(ctx context.Context, releaseType string, application *appv1.Application, createFn func(context.Context, *appv1.Application) error, updateFn func(context.Context, *appv1.Application) error, syncFn func(context.Context, string) error) error {
	switch releaseType {
	case model.ReleaseInstall:
		if err := createFn(ctx, application); err != nil {
			return err
		}
	case model.ReleaseUpgrade, model.ReleaseRollback:
		if err := updateFn(ctx, application); err != nil {
			return err
		}
	default:
		return sharederrs.InvalidArgument("unknown release type")
	}
	return syncFn(ctx, application.Name)
}

func buildArgoApplication(release *model.Release, manifest *manifestdomain.Manifest, app *releasesupport.ApplicationProjection, target releasesupport.DeployTarget) *appv1.Application {
	name := app.Name
	if name == "" {
		name = release.ApplicationID.String()
	}
	source := buildOCIApplicationSource(manifest)
	if source == nil {
		source = buildRepoPluginApplicationSource(release)
	}
	return &appv1.Application{
		TypeMeta:   metav1.TypeMeta{Kind: "Application", APIVersion: "argoproj.io/v1alpha1"},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: appv1.ApplicationSpec{
			Project:     "app",
			Source:      source,
			Destination: appv1.ApplicationDestination{Server: target.DestinationServer, Namespace: target.Namespace},
		},
	}
}

func buildOCIApplicationSource(manifest *manifestdomain.Manifest) *appv1.ApplicationSource {
	if manifest == nil || strings.TrimSpace(manifest.ArtifactRepository) == "" {
		return nil
	}
	revision := strings.TrimSpace(manifest.ArtifactDigest)
	if revision == "" {
		revision = strings.TrimSpace(manifest.ArtifactTag)
	}
	if revision == "" {
		return nil
	}
	return &appv1.ApplicationSource{
		RepoURL:        "oci://" + strings.TrimSpace(manifest.ArtifactRepository),
		TargetRevision: revision,
		Path:           ".",
	}
}

func buildRepoPluginApplicationSource(release *model.Release) *appv1.ApplicationSource {
	manifestRepo := model.GetConfigRepo()
	imageID := release.ImageID.String()
	manifestID := release.ManifestID.String()
	releaseID := release.ID.String()
	return &appv1.ApplicationSource{
		RepoURL: manifestRepo.Address,
		Path:    "./",
		Plugin: &appv1.ApplicationSourcePlugin{
			Name: "plugin",
			Parameters: []appv1.ApplicationSourcePluginParameter{
				{Name: "env", String_: &release.Env},
				{Name: "manifest-id", String_: &manifestID},
				{Name: "image-id", String_: &imageID},
				{Name: "release-id", String_: &releaseID},
			},
		},
	}
}

func (s *releaseService) syncArgoApplication(ctx context.Context, appName string) error {
	applications := argoclient.Client.ArgoprojV1alpha1().Applications("argocd")
	_, err := argoutil.SetAppOperation(applications, appName, &appv1.Operation{Sync: &appv1.SyncOperation{}})
	return err
}
