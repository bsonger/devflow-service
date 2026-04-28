package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	argoutil "github.com/argoproj/argo-cd/v3/util/argo"
	appconfigdownstream "github.com/bsonger/devflow-service/internal/appconfig/transport/downstream"
	intentservice "github.com/bsonger/devflow-service/internal/intent/service"
	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	manifestservice "github.com/bsonger/devflow-service/internal/manifest/service"
	"github.com/bsonger/devflow-service/internal/platform/logger"
	"github.com/bsonger/devflow-service/internal/platform/oci"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/bsonger/devflow-service/internal/release/repository"
	"github.com/bsonger/devflow-service/internal/release/runtime"
	releasesupport "github.com/bsonger/devflow-service/internal/release/support"
	"github.com/bsonger/devflow-service/internal/release/transport/argo"
	servicedownstream "github.com/bsonger/devflow-service/internal/service/transport/downstream"
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
	EnvironmentID  string
	ManifestID     *uuid.UUID
	Status         string
	Type           string
}

var ReleaseService = &releaseService{store: repository.NewPostgresStore()}

var (
	ErrReleaseManifestNotReady = sharederrs.FailedPrecondition("manifest is not ready")
	ErrReleaseAppConfigMissing = sharederrs.FailedPrecondition("effective app config is missing")
)

type releaseManifestReader interface {
	Get(context.Context, uuid.UUID) (*manifestdomain.Manifest, error)
}

type releaseNetworkReader interface {
	ListRoutes(context.Context, string, string) ([]servicedownstream.Route, error)
}

type releaseConfigReader interface {
	FindAppConfig(context.Context, string, string) (*appconfigdownstream.AppConfig, error)
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

func populateReleaseDefaults(release *model.Release, applicationId uuid.UUID, environmentId string) {
	release.ApplicationID = applicationId
	release.Strategy = model.NormalizeReleaseStrategy(release.Strategy)
	if release.Type == "" {
		release.Type = model.ReleaseUpgrade
	}
	if release.Strategy == "" {
		release.Strategy = string(model.ReleaseStrategyRolling)
	}
	if release.EnvironmentID == "" {
		release.EnvironmentID = environmentId
	}
	release.Status = model.ReleasePending
	if len(release.Steps) == 0 {
		release.Steps = model.DefaultReleaseSteps(model.ReleaseStrategyToType(release.Strategy), release.Type)
	}
}

func releaseTargetEnvironment(release *model.Release) string {
	if release == nil {
		return ""
	}
	return strings.TrimSpace(release.EnvironmentID)
}

func newReleaseNetworkReader() releaseNetworkReader {
	runtimeCfg := releasesupport.CurrentRuntimeConfig()
	return servicedownstream.New(strings.TrimSpace(runtimeCfg.Downstream.NetworkServiceBaseURL))
}

func newReleaseConfigReader() releaseConfigReader {
	runtimeCfg := releasesupport.CurrentRuntimeConfig()
	return appconfigdownstream.New(strings.TrimSpace(runtimeCfg.Downstream.ConfigServiceBaseURL))
}

func selectReleaseRoutes(items []servicedownstream.Route, environmentId string) []servicedownstream.Route {
	environmentId = strings.TrimSpace(environmentId)
	out := make([]servicedownstream.Route, 0, len(items))
	for _, item := range items {
		routeEnv := strings.TrimSpace(item.EnvironmentID)
		switch {
		case routeEnv == "":
			out = append(out, item)
		case routeEnv == "base":
			out = append(out, item)
		case environmentId != "" && routeEnv == environmentId:
			out = append(out, item)
		}
	}
	return out
}

func freezeReleaseLiveInputs(ctx context.Context, release *model.Release) error {
	if release == nil {
		return nil
	}
	configReader := newReleaseConfigReader()
	appConfig, err := configReader.FindAppConfig(ctx, release.ApplicationID.String(), releaseTargetEnvironment(release))
	if err != nil {
		return err
	}
	if appConfig == nil || len(appConfig.Files) == 0 {
		return ErrReleaseAppConfigMissing
	}
	files := make([]model.ReleaseFile, 0, len(appConfig.Files))
	for _, item := range appConfig.Files {
		files = append(files, model.ReleaseFile{Name: item.Name, Content: item.Content})
	}
	data := make(map[string]string, len(appConfig.Files))
	for _, item := range appConfig.Files {
		data[item.Name] = item.Content
	}
	release.AppConfigSnapshot = model.ReleaseAppConfig{
		ID:              appConfig.ID,
		MountPath:       appConfig.MountPath,
		Files:           files,
		Data:            data,
		SourceDirectory: appConfig.SourceDirectory,
		SourceCommit:    appConfig.SourceCommit,
	}

	networkReader := newReleaseNetworkReader()
	routes, err := networkReader.ListRoutes(ctx, release.ApplicationID.String(), release.EnvironmentID)
	if err != nil {
		return err
	}
	release.RoutesSnapshot = make([]model.ReleaseRoute, 0, len(routes))
	for _, item := range routes {
		release.RoutesSnapshot = append(release.RoutesSnapshot, model.ReleaseRoute{
			ID:          item.ID,
			Name:        item.Name,
			Host:        item.Host,
			Path:        item.Path,
			ServiceName: item.ServiceName,
			ServicePort: item.ServicePort,
		})
	}
	return nil
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
	if !isReleaseDeployableManifestStatus(manifest.Status) {
		return uuid.Nil, ErrReleaseManifestNotReady
	}
	release.ApplicationID = manifest.ApplicationID
	populateReleaseDefaults(release, manifest.ApplicationID, strings.TrimSpace(release.EnvironmentID))
	if err := freezeReleaseLiveInputs(ctx, release); err != nil {
		return uuid.Nil, err
	}
	markReleaseStepCompleted(release, "freeze_inputs", "release inputs frozen successfully")
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

func isReleaseDeployableManifestStatus(status model.ManifestStatus) bool {
	switch status {
	case model.ManifestReady, model.ManifestSucceeded:
		return true
	default:
		return false
	}
}

func markReleaseStepCompleted(release *model.Release, stepCode, message string) {
	if release == nil {
		return
	}
	now := time.Now()
	for i := range release.Steps {
		if release.Steps[i].Code != stepCode && release.Steps[i].Name != stepCode {
			continue
		}
		release.Steps[i].Status = model.StepSucceeded
		release.Steps[i].Progress = 100
		release.Steps[i].Message = message
		release.Steps[i].StartTime = &now
		release.Steps[i].EndTime = &now
		return
	}
	release.Steps = append(release.Steps, model.ReleaseStep{
		Code:      stepCode,
		Name:      stepCode,
		Status:    model.StepSucceeded,
		Progress:  100,
		Message:   message,
		StartTime: &now,
		EndTime:   &now,
	})
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
	return s.executeReleasePhases(ctx, release)
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

func (s *releaseService) GetBundlePreview(ctx context.Context, id uuid.UUID) (*model.ReleaseBundle, error) {
	release, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	app, err := releasesupport.ApplicationService.Get(ctx, release.ApplicationID)
	if err != nil {
		return nil, err
	}
	manifest, err := releaseManifestSource.Get(ctx, release.ManifestID)
	if err != nil {
		return nil, err
	}
	target, err := releasesupport.ResolveDeployTarget(ctx, release.ApplicationID.String(), releaseTargetEnvironment(release))
	if err != nil {
		return nil, err
	}
	applicationName := ""
	if app != nil {
		applicationName = app.Name
	}
	return buildReleaseBundle(target.Namespace, applicationName, manifest, release)
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

func (s *releaseService) UpdateArtifact(ctx context.Context, releaseID uuid.UUID, repository, tag, digest, ref, message string, status model.StepStatus, progress int32) error {
	release, err := s.Get(ctx, releaseID)
	if err != nil {
		return err
	}
	repository = strings.TrimSpace(repository)
	tag = strings.TrimSpace(tag)
	digest = strings.TrimSpace(digest)
	ref = strings.TrimSpace(ref)
	if repository != "" {
		release.ArtifactRepository = repository
	}
	if tag != "" {
		release.ArtifactTag = tag
	}
	if digest != "" {
		release.ArtifactDigest = digest
	}
	if ref != "" {
		release.ArtifactRef = ref
	}
	release.UpdatedAt = time.Now()
	if err := s.repoStore().UpdateRow(ctx, release); err != nil {
		return err
	}
	if status != "" {
		if progress < 0 {
			progress = 0
		}
		if progress > 100 {
			progress = 100
		}
		return s.UpdateStep(ctx, releaseID, "publish_bundle", status, progress, message, nil, nil)
	}
	return nil
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
		nextSteps = append(nextSteps, model.ReleaseStep{Code: stepName, Name: stepName, Progress: progress, Status: status, Message: message, StartTime: start, EndTime: end})
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
		if step.Code == stepName || step.Name == stepName {
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
		if steps[i].Code != stepName && steps[i].Name != stepName {
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

func (s *releaseService) executeReleasePhases(ctx context.Context, release *model.Release) error {
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
	target, err := releasesupport.ResolveDeployTarget(ctx, release.ApplicationID.String(), releaseTargetEnvironment(release))
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

	if err := s.renderDeploymentBundle(ctx, release, manifest, app, *target); err != nil {
		return err
	}
	if err := s.publishDeploymentBundle(ctx, release, manifest, app, *target); err != nil {
		return err
	}
	if err := s.createArgoApplication(ctx, release, manifest, app, *target); err != nil {
		return err
	}
	log.Info("release phases completed", zap.String("result", "success"))
	return nil
}

func (s *releaseService) renderDeploymentBundle(ctx context.Context, release *model.Release, manifest *manifestdomain.Manifest, app *releasesupport.ApplicationProjection, target releasesupport.DeployTarget) error {
	if err := s.UpdateStep(ctx, release.ID, "render_deployment_bundle", model.StepRunning, 25, "rendering deployment bundle", nil, nil); err != nil {
		return err
	}
	applicationName := ""
	if app != nil {
		applicationName = app.Name
	}
	bundle, err := buildReleaseBundle(target.Namespace, applicationName, manifest, release)
	if err != nil {
		_ = s.UpdateStep(ctx, release.ID, "render_deployment_bundle", model.StepFailed, 100, err.Error(), nil, nil)
		return err
	}
	message := fmt.Sprintf("deployment bundle rendered (%d resources, %d files)", len(bundle.RenderedObjects), len(bundle.Files))
	return s.UpdateStep(ctx, release.ID, "render_deployment_bundle", model.StepSucceeded, 100, message, nil, nil)
}

func (s *releaseService) publishDeploymentBundle(ctx context.Context, release *model.Release, manifest *manifestdomain.Manifest, app *releasesupport.ApplicationProjection, target releasesupport.DeployTarget) error {
	runtimeCfg := releasesupport.CurrentRuntimeConfig()
	if err := s.UpdateStep(ctx, release.ID, "publish_bundle", model.StepRunning, 25, publishBundleStartMessage(runtimeCfg), nil, nil); err != nil {
		return err
	}
	if !runtimeCfg.ManifestRegistryEnabled {
		return s.UpdateStep(ctx, release.ID, "publish_bundle", model.StepSucceeded, 100, "bundle publication skipped; manifest registry disabled", nil, nil)
	}
	applicationName := ""
	if app != nil {
		applicationName = app.Name
	}
	bundle, err := buildReleaseBundle(target.Namespace, applicationName, manifest, release)
	if err != nil {
		_ = s.UpdateStep(ctx, release.ID, "publish_bundle", model.StepFailed, 100, err.Error(), nil, nil)
		return err
	}
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

func (s *releaseService) createArgoApplication(ctx context.Context, release *model.Release, manifest *manifestdomain.Manifest, app *releasesupport.ApplicationProjection, target releasesupport.DeployTarget) error {
	log := logger.LoggerWithContext(ctx)
	if log == nil {
		log = zap.NewNop()
	}
	application := buildArgoApplication(release, manifest, app, target)
	if err := s.UpdateStep(ctx, release.ID, "create_argocd_application", model.StepRunning, 25, createArgoApplicationStartMessage(release, application.Name, target), nil, nil); err != nil {
		return err
	}
	if err := s.persistArgoApplicationMetadata(ctx, release, application.Name); err != nil {
		return err
	}
	sc := trace.SpanContextFromContext(ctx)
	application.Annotations = map[string]string{
		oci.TraceIDAnnotation: sc.TraceID().String(),
		oci.SpanAnnotation:    sc.SpanID().String(),
	}
	application.Labels = map[string]string{"status": string(model.ReleaseRunning), model.ReleaseIDLabel: release.ID.String()}

	err := applyReleaseApplication(ctx, release.Type, application, argoclient.CreateApplication, argoclient.UpdateApplication, s.syncArgoApplication)
	if err != nil {
		_ = s.UpdateStep(ctx, release.ID, "create_argocd_application", model.StepFailed, 100, createArgoApplicationFailureMessage(application.Name, err), nil, nil)
		log.Error("argo sync failed", zap.String("result", "error"), zap.Error(err))
		return err
	}
	_ = s.UpdateStep(ctx, release.ID, "create_argocd_application", model.StepSucceeded, 100, createArgoApplicationSuccessMessage(release, application.Name), nil, nil)
	if code, message := releaseDeploymentStartStep(release); code != "" {
		_ = s.UpdateStep(ctx, release.ID, code, model.StepRunning, 10, message, nil, nil)
	}
	return nil
}

func deriveReleaseArtifactMetadata(release *model.Release, app *releasesupport.ApplicationProjection, cfg manifestdomain.ManifestRegistryConfig) (repository, tag, ref string) {
	applicationName := ""
	if app != nil {
		applicationName = app.Name
	}
	repository = cfg.RepositoryFor(applicationName, releaseTargetEnvironment(release))
	tag = release.ID.String()
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
	tag = release.ID.String()
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

func createArgoApplicationStartMessage(release *model.Release, appName string, target releasesupport.DeployTarget) string {
	appName = strings.TrimSpace(appName)
	environmentId := releaseTargetEnvironment(release)
	namespace := strings.TrimSpace(target.Namespace)
	switch {
	case appName != "" && environmentId != "" && namespace != "":
		return fmt.Sprintf("creating argocd application %s for environment %s in namespace %s", appName, environmentId, namespace)
	case appName != "" && environmentId != "":
		return fmt.Sprintf("creating argocd application %s for environment %s", appName, environmentId)
	case appName != "":
		return fmt.Sprintf("creating argocd application %s", appName)
	default:
		return "creating argocd application"
	}
}

func createArgoApplicationSuccessMessage(release *model.Release, appName string) string {
	appName = strings.TrimSpace(appName)
	environmentId := releaseTargetEnvironment(release)
	artifactRef := ""
	if release != nil {
		artifactRef = strings.TrimSpace(release.ArtifactRef)
	}
	switch {
	case appName != "" && environmentId != "" && artifactRef != "":
		return fmt.Sprintf("argocd application %s created for environment %s and sync requested from %s", appName, environmentId, artifactRef)
	case appName != "" && artifactRef != "":
		return fmt.Sprintf("argocd application %s created and sync requested from %s", appName, artifactRef)
	case appName != "" && environmentId != "":
		return fmt.Sprintf("argocd application %s created for environment %s and sync requested", appName, environmentId)
	case appName != "":
		return fmt.Sprintf("argocd application %s created and sync requested", appName)
	default:
		return "argocd application created and sync requested"
	}
}

func createArgoApplicationFailureMessage(appName string, err error) string {
	appName = strings.TrimSpace(appName)
	if err == nil {
		if appName != "" {
			return fmt.Sprintf("argocd application %s failed", appName)
		}
		return "argocd application failed"
	}
	if appName != "" {
		return fmt.Sprintf("argocd application %s failed: %s", appName, err.Error())
	}
	return err.Error()
}

func (s *releaseService) persistArgoApplicationMetadata(ctx context.Context, release *model.Release, appName string) error {
	appName = strings.TrimSpace(appName)
	if release == nil || appName == "" {
		return nil
	}
	if release.ArgoCDApplicationName == appName && release.ExternalRef == appName {
		return nil
	}
	release.ArgoCDApplicationName = appName
	release.ExternalRef = appName
	release.UpdatedAt = time.Now()
	return s.repoStore().UpdateRow(ctx, release)
}

func releaseDeploymentStartStep(release *model.Release) (string, string) {
	if release == nil {
		return "", ""
	}
	switch model.ReleaseStrategyToType(release.Strategy) {
	case model.BlueGreen:
		return "deploy_preview", "preview deployment started"
	case model.Canary:
		return "deploy_canary", "canary deployment started"
	default:
		return "start_deployment", "deployment sync started"
	}
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
		attribute.String("deployment.environment", strings.TrimSpace(release.EnvironmentID)),
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
	return &appv1.Application{
		TypeMeta:   metav1.TypeMeta{Kind: "Application", APIVersion: "argoproj.io/v1alpha1"},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: appv1.ApplicationSpec{
			Project:     "app",
			Source:      buildOCIApplicationSource(release),
			Destination: appv1.ApplicationDestination{Server: target.DestinationServer, Namespace: target.Namespace},
		},
	}
}

func buildOCIApplicationSource(release *model.Release) *appv1.ApplicationSource {
	repoURL, targetRevision := deriveOCIApplicationArtifact(release)
	return &appv1.ApplicationSource{
		RepoURL:        repoURL,
		TargetRevision: targetRevision,
		Path:           ".",
	}
}

func deriveOCIApplicationArtifact(release *model.Release) (string, string) {
	if release == nil {
		return "", ""
	}
	repository := strings.TrimSpace(release.ArtifactRepository)
	targetRevision := strings.TrimSpace(release.ArtifactDigest)
	if targetRevision == "" {
		targetRevision = strings.TrimSpace(release.ArtifactTag)
	}
	artifactRef := strings.TrimSpace(release.ArtifactRef)
	if strings.HasPrefix(artifactRef, "oci://") {
		trimmed := strings.TrimPrefix(artifactRef, "oci://")
		if repository == "" {
			if idx := strings.LastIndex(trimmed, "@"); idx > 0 {
				repository = trimmed[:idx]
				if targetRevision == "" {
					targetRevision = trimmed[idx+1:]
				}
			} else if idx := strings.LastIndex(trimmed, ":"); idx > 0 {
				repository = trimmed[:idx]
				if targetRevision == "" {
					targetRevision = trimmed[idx+1:]
				}
			} else {
				repository = trimmed
			}
		}
	}
	if repository == "" {
		return "", targetRevision
	}
	return "oci://" + repository, targetRevision
}

func (s *releaseService) syncArgoApplication(ctx context.Context, appName string) error {
	applications := argoclient.Client.ArgoprojV1alpha1().Applications("argocd")
	_, err := argoutil.SetAppOperation(applications, appName, &appv1.Operation{Sync: &appv1.SyncOperation{}})
	return err
}
