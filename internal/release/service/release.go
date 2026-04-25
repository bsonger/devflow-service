package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	argoutil "github.com/argoproj/argo-cd/v3/util/argo"
	imageservice "github.com/bsonger/devflow-service/internal/image/service"
	intentservice "github.com/bsonger/devflow-service/internal/intent/service"
	manifestservice "github.com/bsonger/devflow-service/internal/manifest/service"
	"github.com/bsonger/devflow-service/internal/platform/logger"
	"github.com/bsonger/devflow-service/internal/release/transport/argo"
	imagedomain "github.com/bsonger/devflow-service/internal/image/domain"
	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	store "github.com/bsonger/devflow-service/internal/platform/db"
	"github.com/bsonger/devflow-service/internal/release/runtime"
	runtimeclient "github.com/bsonger/devflow-service/internal/release/transport/runtime"
	"github.com/google/uuid"
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

var ReleaseService = &releaseService{}

var (
	ErrImageMissingRuntimeSpecRevision                      = errors.New("image runtime_spec_revision_id is required")
	ErrRuntimeSpecBindingMismatch                           = errors.New("image runtime_spec_revision_id does not match release application and env")
	ErrReleaseManifestNotReady                              = errors.New("manifest is not ready")
	runtimeLookupClient                runtimeclient.Lookup = runtimeclient.New("")
)

type releaseManifestReader interface {
	Get(context.Context, uuid.UUID) (*manifestdomain.Manifest, error)
}

var releaseManifestSource releaseManifestReader = manifestservice.ManifestService

type releaseService struct{}

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
	log = log.With(zap.String("release.type", release.Type), zap.String("manifest.id", release.ManifestID.String()))

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
	if err := s.insert(ctx, release); err != nil {
		return uuid.Nil, err
	}

	log = log.With(zap.String("release.id", release.ID.String()), zap.String("application.id", release.ApplicationID.String()))

	if runtime.IsIntentMode() {
		intentID, err := intentservice.IntentService.CreateReleaseIntent(ctx, release)
		if err != nil {
			return release.ID, err
		}
		log.Info("release accepted in intent mode", zap.String("intent_id", intentID.String()))
		return release.ID, nil
	}

	if err := s.DispatchRelease(ctx, release.ID); err != nil {
		s.handleSyncArgoError(ctx, release, err)
		return release.ID, err
	}
	return release.ID, nil
}

func (s *releaseService) insert(ctx context.Context, release *model.Release) error {
	stepsJSON, err := marshalJSON(release.Steps, "[]")
	if err != nil {
		return err
	}
	_, err = store.DB().ExecContext(ctx, `
		insert into releases (
			id, execution_intent_id, application_id, manifest_id, image_id, env, type, steps, status, external_ref, created_at, updated_at, deleted_at
		) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
	`, release.ID, nullableUUIDPtr(release.ExecutionIntentID), release.ApplicationID, release.ManifestID, release.ImageID, release.Env, release.Type, stepsJSON, release.Status, release.ExternalRef, release.CreatedAt, release.UpdatedAt, release.DeletedAt)
	return err
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
	log := logger.LoggerWithContext(ctx).With(zap.String("release.id", release.ID.String()), zap.String("release.type", release.Type))
	log.Error("sync argo failed", zap.Error(err))
	_ = s.updateStatus(ctx, release.ID, model.ReleaseSyncFailed)
}

func (s *releaseService) Get(ctx context.Context, id uuid.UUID) (*model.Release, error) {
	return scanRelease(store.DB().QueryRowContext(ctx, `
		select id, execution_intent_id, application_id, manifest_id, image_id, env, type, steps, status, external_ref, created_at, updated_at, deleted_at
		from releases
		where id = $1 and deleted_at is null
	`, id))
}

func (s *releaseService) Update(ctx context.Context, release *model.Release) error {
	current, err := s.Get(ctx, release.ID)
	if err != nil {
		return err
	}
	release.CreatedAt = current.CreatedAt
	release.DeletedAt = current.DeletedAt
	release.WithUpdateDefault()
	return s.updateRow(ctx, release)
}

func (s *releaseService) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := store.DB().ExecContext(ctx, `
		update releases
		set deleted_at = $2, updated_at = $2
		where id = $1 and deleted_at is null
	`, id, time.Now())
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (s *releaseService) List(ctx context.Context, filter ReleaseListFilter) ([]*model.Release, error) {
	query := `
		select id, execution_intent_id, application_id, manifest_id, image_id, env, type, steps, status, external_ref, created_at, updated_at, deleted_at
		from releases
	`
	clauses := make([]string, 0, 5)
	args := make([]any, 0, 5)
	if !filter.IncludeDeleted {
		clauses = append(clauses, "deleted_at is null")
	}
	if filter.ApplicationID != nil {
		args = append(args, *filter.ApplicationID)
		clauses = append(clauses, placeholderClause("application_id", len(args)))
	}
	if filter.ManifestID != nil {
		args = append(args, *filter.ManifestID)
		clauses = append(clauses, placeholderClause("manifest_id", len(args)))
	}
	if filter.ImageID != nil {
		args = append(args, *filter.ImageID)
		clauses = append(clauses, placeholderClause("image_id", len(args)))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		clauses = append(clauses, placeholderClause("status", len(args)))
	}
	if filter.Type != "" {
		args = append(args, filter.Type)
		clauses = append(clauses, placeholderClause("type", len(args)))
	}
	if len(clauses) > 0 {
		query += " where " + strings.Join(clauses, " and ")
	}
	query += " order by created_at desc"

	rows, err := store.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*model.Release, 0)
	for rows.Next() {
		item, err := scanRelease(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
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
	release.Status = status
	release.UpdatedAt = time.Now()
	return s.updateRow(ctx, release)
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
		if err := s.updateSteps(ctx, release); err != nil {
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
	if err := s.updateSteps(ctx, release); err != nil {
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
	app, err := ApplicationService.Get(ctx, release.ApplicationID)
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
	target, err := resolveDeployTarget(ctx, release.ApplicationID.String(), environmentID)
	if err != nil {
		return err
	}

	// Run ordered bootstrap gates before Argo Application apply
	bootstrap, err := newBootstrapExecutor()
	if err != nil {
		log.Error("bootstrap executor creation failed", zap.String("release_id", release.ID.String()), zap.Error(err))
		return err
	}

	results, err := bootstrap.runBootstrapGates(ctx, *target, app.ProjectName)
	for _, res := range results {
		_ = s.UpdateStep(ctx, release.ID, res.StepName, res.Status, 100, res.Message, res.Start, res.End)
	}
	if err != nil {
		log.Error("bootstrap gates failed", zap.String("release_id", release.ID.String()), zap.Error(err))
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
		log.Error("argo sync failed", zap.String("release_id", release.ID.String()), zap.String("type", release.Type), zap.Error(err))
		return err
	}
	return nil
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
		return errors.New("unknown release type")
	}
	return syncFn(ctx, application.Name)
}

func buildArgoApplication(release *model.Release, manifest *manifestdomain.Manifest, app *applicationProjection, target deployTarget) *appv1.Application {
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

func (s *releaseService) updateRow(ctx context.Context, release *model.Release) error {
	stepsJSON, err := marshalJSON(release.Steps, "[]")
	if err != nil {
		return err
	}
	result, err := store.DB().ExecContext(ctx, `
		update releases
		set execution_intent_id=$2, application_id=$3, manifest_id=$4, image_id=$5, env=$6, type=$7, steps=$8, status=$9, external_ref=$10, updated_at=$11, deleted_at=$12
		where id = $1
	`, release.ID, nullableUUIDPtr(release.ExecutionIntentID), release.ApplicationID, release.ManifestID, release.ImageID, release.Env, release.Type, stepsJSON, release.Status, release.ExternalRef, release.UpdatedAt, release.DeletedAt)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (s *releaseService) updateSteps(ctx context.Context, release *model.Release) error {
	stepsJSON, err := marshalJSON(release.Steps, "[]")
	if err != nil {
		return err
	}
	result, err := store.DB().ExecContext(ctx, `
		update releases
		set steps = $2, updated_at = $3
		where id = $1 and deleted_at is null
	`, release.ID, stepsJSON, release.UpdatedAt)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}
