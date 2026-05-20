package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	appconfigdownstream "github.com/bsonger/devflow-service/internal/appconfig/transport/downstream"
	intentservice "github.com/bsonger/devflow-service/internal/intent/service"
	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	manifestservice "github.com/bsonger/devflow-service/internal/manifest/service"
	platformobs "github.com/bsonger/devflow-service/internal/platform/runtime/observability"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/bsonger/devflow-service/internal/release/repository"
	"github.com/bsonger/devflow-service/internal/release/runtime"
	releasesupport "github.com/bsonger/devflow-service/internal/release/support"
	"github.com/bsonger/devflow-service/internal/release/transport/argo"
	releasedownstream "github.com/bsonger/devflow-service/internal/release/transport/downstream"
	servicedownstream "github.com/bsonger/devflow-service/internal/service/transport/downstream"
	"github.com/bsonger/devflow-service/internal/shared/downstreamhttp"
	sharederrs "github.com/bsonger/devflow-service/internal/shared/errs"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

type ReleaseListFilter struct {
	IncludeDeleted bool
	ApplicationID  *uuid.UUID
	EnvironmentID  string
	ManifestID     *uuid.UUID
	Status         string
	Type           string
}

var ReleaseService = &releaseService{store: repository.NewPostgresStore(), bundleStore: repository.NewBundlePostgresStore()}

var (
	releaseGetArgoApplication    = argoclient.GetApplication
	releaseUpdateArgoApplication = argoclient.UpdateApplication
	releaseNewKubeClient         = func() (kubernetes.Interface, error) {
		if model.KubeConfig == nil {
			return nil, fmt.Errorf("missing kubernetes config")
		}
		return kubernetes.NewForConfig(model.KubeConfig)
	}
	releaseNewDynamicClient = func() (dynamic.Interface, error) {
		if model.KubeConfig == nil {
			return nil, fmt.Errorf("missing kubernetes config")
		}
		return dynamic.NewForConfig(model.KubeConfig)
	}
)

var argoApplicationNameSanitizer = regexp.MustCompile(`[^a-z0-9-]+`)

var (
	ErrReleaseManifestNotFound     = sharederrs.NotFound("manifest not found")
	ErrReleaseManifestNotAvailable = sharederrs.FailedPrecondition("manifest is not available")
	ErrReleaseBundleNotReady       = sharederrs.FailedPrecondition("bundle not ready")
	ErrReleaseUnknownStep          = sharederrs.InvalidArgument("unknown release step")
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

var releaseManifestSource releaseManifestReader = releaseManifestReaderWithFallback{local: manifestservice.ManifestService}
var releaseConfigReaderFactory = func() releaseConfigReader { return newReleaseConfigReader() }
var releaseNetworkReaderFactory = func() releaseNetworkReader { return newReleaseNetworkReader() }

type releaseManifestReaderWithFallback struct {
	local releaseManifestReader
}

func (r releaseManifestReaderWithFallback) Get(ctx context.Context, id uuid.UUID) (*manifestdomain.Manifest, error) {
	if r.local == nil {
		return nil, sql.ErrNoRows
	}
	item, err := r.local.Get(ctx, id)
	if err == nil || !errors.Is(err, sql.ErrNoRows) {
		return item, err
	}
	baseURL := strings.TrimSpace(releasesupport.CurrentRuntimeConfig().Downstream.ManifestSourceBaseURL)
	if baseURL == "" {
		return nil, err
	}
	remote, remoteErr := releasedownstream.NewManifestClient(baseURL).GetManifest(ctx, id.String())
	if remoteErr != nil {
		if downstreamhttp.IsStatus(remoteErr, 404) {
			return nil, err
		}
		return nil, remoteErr
	}
	return remote, nil
}

type releaseService struct {
	store       repository.Store
	bundleStore repository.BundleStore
}

func (s *releaseService) repoStore() repository.Store {
	if s.store == nil {
		s.store = repository.NewPostgresStore()
	}
	return s.store
}

func (s *releaseService) repoBundleStore() repository.BundleStore {
	if s.bundleStore == nil {
		s.bundleStore = repository.NewBundlePostgresStore()
	}
	return s.bundleStore
}

func populateReleaseDefaults(release *model.Release, applicationId uuid.UUID, environmentId string) {
	release.ApplicationID = applicationId
	release.Strategy = model.NormalizeReleaseStrategy(release.Strategy)
	release.Type = model.NormalizeReleaseAction(release.Type)
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

// freezeReleaseLiveInputs snapshots deploy-time config and route inputs onto the release row.
// Unlike manifest snapshots, these inputs are environment-aware and exist only to produce the deployable release bundle.
func freezeReleaseLiveInputs(ctx context.Context, release *model.Release) error {
	if release == nil {
		return nil
	}
	configReader := releaseConfigReaderFactory()
	appConfig, err := configReader.FindAppConfig(ctx, release.ApplicationID.String(), releaseTargetEnvironment(release))
	if err != nil {
		return err
	}
	if appConfig != nil {
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
	}

	networkReader := releaseNetworkReaderFactory()
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

// Create validates that the build-side manifest is deployable, freezes release-only live inputs, and then starts deploy execution.
// This is the explicit handoff point from manifest build observation into release-owned bundle render/publication and Argo delivery.
func (s *releaseService) Create(ctx context.Context, release *model.Release) (uuid.UUID, error) {
	log := platformobs.OperationLogger(ctx, "release_service", "create_release", "release",
		zap.String("result", "started"),
		zap.String("release_type", release.Type),
		zap.String("manifest_id", release.ManifestID.String()),
	)

	manifest, err := releaseManifestSource.Get(ctx, release.ManifestID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return uuid.Nil, ErrReleaseManifestNotFound
		}
		return uuid.Nil, err
	}
	if !isReleaseDeployableManifestStatus(manifest.Status) {
		return uuid.Nil, ErrReleaseManifestNotAvailable
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
	case model.ManifestAvailable:
		return true
	default:
		return false
	}
}

func markReleaseStepCompleted(release *model.Release, stepCode, message string) {
	if release == nil {
		return
	}
	stepCode = normalizeReleaseStepKey(stepCode)
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
	log := platformobs.OperationLogger(ctx, "release_service", "sync_release", "release",
		zap.String("resource_id", release.ID.String()),
		zap.String("release_type", release.Type),
	)
	log.Error("sync argo failed", zap.String("result", "error"), zap.Error(err))
	_ = s.updateStatus(ctx, release.ID, model.ReleaseSyncFailed)
}

func (s *releaseService) Get(ctx context.Context, id uuid.UUID) (*model.Release, error) {
	return s.loadRelease(ctx, id)
}

func (s *releaseService) loadRelease(ctx context.Context, id uuid.UUID) (*model.Release, error) {
	release, err := s.repoStore().Get(ctx, id)
	if err != nil {
		return nil, err
	}
	release.Type = model.NormalizeReleaseAction(release.Type)
	if release.Type == "" {
		release.Type = model.ReleaseUpgrade
	}
	release.Strategy = model.NormalizeReleaseStrategy(release.Strategy)
	release.Steps = normalizeReleaseSteps(release)
	s.attachBundleSummary(ctx, release)
	return release, nil
}

func (s *releaseService) GetBundlePreview(ctx context.Context, id uuid.UUID) (*model.ReleaseBundlePreview, error) {
	release, err := s.loadRelease(ctx, id)
	if err != nil {
		return nil, err
	}
	bundle, err := s.repoBundleStore().GetByReleaseID(ctx, release.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			step := findReleaseStep(release.Steps, "render_deployment_bundle")
			if step == nil || step.Status != model.StepSucceeded {
				return nil, ErrReleaseBundleNotReady
			}
		}
		return nil, err
	}
	manifest, err := releaseManifestSource.Get(ctx, release.ManifestID)
	if err != nil {
		return nil, err
	}
	return buildReleaseBundlePreview(release, manifest, bundle), nil
}

func (s *releaseService) Update(ctx context.Context, release *model.Release) error {
	current, err := s.loadRelease(ctx, release.ID)
	if err != nil {
		return err
	}
	if isReleaseTerminalStatus(current.Status) {
		return nil
	}
	release.CreatedAt = current.CreatedAt
	release.DeletedAt = current.DeletedAt
	release.WithUpdateDefault()
	return s.repoStore().UpdateRow(ctx, release)
}

func (s *releaseService) UpdateArtifact(ctx context.Context, releaseID uuid.UUID, repository, tag, digest, ref, message string, status model.StepStatus, progress int32) error {
	release, err := s.loadRelease(ctx, releaseID)
	if err != nil {
		return err
	}
	if isReleaseTerminalStatus(release.Status) {
		return nil
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
	releases, err := s.repoStore().List(ctx, repository.ListFilter(filter))
	if err != nil {
		return nil, err
	}
	for _, release := range releases {
		release.Steps = normalizeReleaseSteps(release)
		s.attachBundleSummary(ctx, release)
	}
	return releases, nil
}

func (s *releaseService) attachBundleSummary(ctx context.Context, release *model.Release) {
	if release == nil {
		return
	}
	bundle, err := s.repoBundleStore().GetByReleaseID(ctx, release.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			release.BundleSummary = nil
			return
		}
		release.BundleSummary = nil
		return
	}
	release.BundleSummary = buildReleaseBundleSummary(release, bundle)
}

func (s *releaseService) updateStatus(ctx context.Context, releaseID uuid.UUID, status model.ReleaseStatus) error {
	return newReleaseStatusManager(s).updateStatus(ctx, releaseID, status)
}

func (s *releaseService) UpdateStatus(ctx context.Context, releaseID uuid.UUID, status model.ReleaseStatus) error {
	return s.updateStatus(ctx, releaseID, status)
}

func (s *releaseService) UpdateStep(ctx context.Context, releaseID uuid.UUID, stepName string, status model.StepStatus, progress int32, message string, start, end *time.Time) error {
	metricStart := time.Now()
	stepName = normalizeReleaseStepKey(stepName)
	if stepName == "" {
		return nil
	}
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	release, err := s.loadRelease(ctx, releaseID)
	if err != nil {
		return err
	}
	if isReleaseTerminalStatus(release.Status) {
		return nil
	}
	nextSteps := cloneReleaseSteps(release.Steps)
	currentStep := findReleaseStep(release.Steps, stepName)
	if currentStep == nil {
		return ErrReleaseUnknownStep
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
	if status == model.StepSucceeded || status == model.StepFailed {
		observeReleaseStage(ctx, release, stepName, status == model.StepSucceeded, time.Since(metricStart))
	}
	return s.updateStatusFromSteps(ctx, releaseID, release.Type, release.Status, nextSteps)
}

func findReleaseStep(steps []model.ReleaseStep, stepName string) *model.ReleaseStep {
	stepName = normalizeReleaseStepKey(stepName)
	for _, step := range steps {
		if step.Code == stepName || normalizeReleaseStepKey(step.Name) == stepName {
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

func isReleaseTerminalStatus(status model.ReleaseStatus) bool {
	switch status {
	case model.ReleaseSucceeded, model.ReleaseFailed, model.ReleaseRolledBack, model.ReleaseSyncFailed:
		return true
	default:
		return false
	}
}

func applyReleaseStepUpdate(steps []model.ReleaseStep, stepName string, status model.StepStatus, progress int32, message string, start, end *time.Time) {
	stepName = normalizeReleaseStepKey(stepName)
	for i := range steps {
		if steps[i].Code != stepName && normalizeReleaseStepKey(steps[i].Name) != stepName {
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

// executeReleasePhases runs the deploy-side pipeline after manifest build handoff has completed.
// From this point on, step messages, bundle preview/publication, and Argo application sync are the authoritative diagnostics surfaces.
func (s *releaseService) executeReleasePhases(ctx context.Context, release *model.Release) error {
	annotateReleaseSpan(ctx, release)
	log := platformobs.OperationLogger(ctx, "release_service", "sync_release", "release",
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
	if err := newReleaseHandoffController(s).run(ctx, release, manifest, app, *target); err != nil {
		return err
	}
	log.Info("release phases completed", zap.String("result", "success"))
	return nil
}

func normalizeReleaseStepKey(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, "-", "_")
	switch value {
	case "":
		return ""
	case "ensure namespace":
		return "ensure_namespace"
	case "ensure pull secret":
		return "ensure_pull_secret"
	case "ensure appproject destination":
		return "ensure_appproject_destination"
	case "canary 10% traffic":
		return "canary_10"
	case "canary 30% traffic":
		return "canary_30"
	case "canary 60% traffic":
		return "canary_60"
	case "canary 100% traffic":
		return "canary_100"
	}
	value = strings.ReplaceAll(value, " ", "_")
	return value
}

func normalizeReleaseSteps(release *model.Release) []model.ReleaseStep {
	if release == nil {
		return nil
	}
	releaseType := inferReleaseType(release)
	canonical := model.DefaultReleaseSteps(releaseType, release.Type)
	if len(canonical) == 0 {
		return release.Steps
	}
	byCode := make(map[string]model.ReleaseStep, len(release.Steps))
	for _, step := range release.Steps {
		key := normalizeReleaseStepKey(step.Code)
		if key == "" {
			key = normalizeReleaseStepKey(step.Name)
		}
		if key == "" {
			continue
		}
		if step.Code == "" {
			step.Code = key
		}
		byCode[key] = step
	}
	out := make([]model.ReleaseStep, 0, len(canonical))
	for _, expected := range canonical {
		if actual, ok := byCode[expected.Code]; ok {
			actual.Code = expected.Code
			actual.Name = expected.Name
			out = append(out, actual)
			continue
		}
		out = append(out, expected)
	}
	return out
}

func inferReleaseType(release *model.Release) model.ReleaseType {
	if release == nil {
		return model.Normal
	}
	releaseType := model.ReleaseStrategyToType(release.Strategy)
	if releaseType != model.Normal {
		return releaseType
	}
	for _, step := range release.Steps {
		switch normalizeReleaseStepKey(step.Code) {
		case "deploy_preview", "observe_preview", "switch_traffic", "verify_active":
			return model.BlueGreen
		case "deploy_canary", "canary_10", "canary_30", "canary_60", "canary_100":
			return model.Canary
		}
	}
	return model.Normal
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
		attribute.String("devflow.release.id", release.ID.String()),
		attribute.String("devflow.release.type", release.Type),
		attribute.String("devflow.application.id", release.ApplicationID.String()),
		attribute.String("devflow.manifest.id", release.ManifestID.String()),
		attribute.String("devflow.environment.id", strings.TrimSpace(release.EnvironmentID)),
	}
	span.SetAttributes(attrs...)
}

func buildReleaseSyncOperation() *appv1.Operation {
	return &appv1.Operation{
		Sync: &appv1.SyncOperation{
			Prune: true,
			SyncOptions: appv1.SyncOptions{
				"Replace=true",
				"Prune=true",
			},
			SyncStrategy: &appv1.SyncStrategy{
				Apply: &appv1.SyncStrategyApply{Force: true},
			},
		},
	}
}

func buildArgoApplication(release *model.Release, manifest *manifestdomain.Manifest, app *releasesupport.ApplicationProjection, target releasesupport.DeployTarget) *appv1.Application {
	name := deriveArgoApplicationName(release, app)
	return &appv1.Application{
		TypeMeta:   metav1.TypeMeta{Kind: "Application", APIVersion: "argoproj.io/v1alpha1"},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: appv1.ApplicationSpec{
			Project:           "app",
			Source:            buildOCIApplicationSource(release),
			Destination:       appv1.ApplicationDestination{Server: target.DestinationServer, Namespace: target.Namespace},
			IgnoreDifferences: releaseApplicationIgnoreDifferences(release),
		},
	}
}

func deriveArgoApplicationName(release *model.Release, app *releasesupport.ApplicationProjection) string {
	base := ""
	if app != nil {
		base = strings.TrimSpace(app.Name)
	}
	if base == "" && release != nil {
		base = release.ApplicationID.String()
	}
	base = sanitizeArgoApplicationName(base)
	if base == "" {
		return ""
	}

	env := ""
	if release != nil {
		env = sanitizeArgoApplicationName(release.EnvironmentID)
	}
	if env == "" {
		return base
	}
	return trimArgoApplicationName(base + "-" + env)
}

func sanitizeArgoApplicationName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	value = strings.ReplaceAll(value, "_", "-")
	value = argoApplicationNameSanitizer.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	return trimArgoApplicationName(value)
}

func trimArgoApplicationName(value string) string {
	const maxLen = 63
	value = strings.Trim(value, "-")
	if len(value) <= maxLen {
		return value
	}
	return strings.Trim(value[:maxLen], "-")
}

func releaseApplicationIgnoreDifferences(release *model.Release) appv1.IgnoreDifferences {
	return appv1.IgnoreDifferences{
		releaseWorkloadRestartedAtIgnoreDifference(release),
	}
}

func releaseWorkloadRestartedAtIgnoreDifference(release *model.Release) appv1.ResourceIgnoreDifferences {
	group, kind := releasePrimaryWorkloadIgnoreTarget(release)
	return appv1.ResourceIgnoreDifferences{
		Group: group,
		Kind:  kind,
		JSONPointers: []string{
			"/metadata/labels/devflow.io~1observe-state",
			"/spec/template/metadata/annotations/kubectl.kubernetes.io~1restartedAt",
			"/spec/template/metadata/labels/devflow.io~1observe-state",
		},
	}
}

func releasePrimaryWorkloadIgnoreTarget(release *model.Release) (string, string) {
	switch model.ReleaseStrategyToType(release.Strategy) {
	case model.BlueGreen, model.Canary:
		return "argoproj.io", "Rollout"
	default:
		return "apps", "Deployment"
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
	repository, targetRevision, artifactRef := releaseExecutionArtifactMetadata(release)
	if targetRevision == "" {
		targetRevision = releaseExecutionArtifactTag(release)
	}
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

func releaseExecutionArtifactMetadata(release *model.Release) (repository, digest, ref string) {
	if release == nil {
		return "", "", ""
	}
	if strings.EqualFold(strings.TrimSpace(release.Type), model.ReleaseRollback) {
		repository = strings.TrimSpace(release.RollbackTargetArtifactRepository)
		digest = strings.TrimSpace(release.RollbackTargetArtifactDigest)
		ref = strings.TrimSpace(release.RollbackTargetArtifactRef)
		if repository != "" || digest != "" || ref != "" {
			return repository, digest, ref
		}
	}
	return strings.TrimSpace(release.ArtifactRepository), strings.TrimSpace(release.ArtifactDigest), strings.TrimSpace(release.ArtifactRef)
}

func releaseExecutionArtifactTag(release *model.Release) string {
	if release == nil {
		return ""
	}
	if strings.EqualFold(strings.TrimSpace(release.Type), model.ReleaseRollback) {
		if tag := strings.TrimSpace(release.RollbackTargetArtifactTag); tag != "" {
			return tag
		}
	}
	return strings.TrimSpace(release.ArtifactTag)
}

func releaseExecutionArtifactRef(release *model.Release) string {
	_, _, ref := releaseExecutionArtifactMetadata(release)
	return ref
}
