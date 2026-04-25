package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	intentservice "github.com/bsonger/devflow-service/internal/intent/service"
	"github.com/bsonger/devflow-service/internal/platform/logger"
	imagedomain "github.com/bsonger/devflow-service/internal/image/domain"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	localtekton "github.com/bsonger/devflow-service/internal/release/transport/tekton"
	store "github.com/bsonger/devflow-service/internal/platform/db"
	"github.com/bsonger/devflow-service/internal/release/runtime"
	"github.com/google/uuid"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type ImageListFilter struct {
	IncludeDeleted bool
	ApplicationID  *uuid.UUID
	PipelineID     string
	Status         string
	Branch         string
	Name           string
}

var ImageService = &imageService{}

const (
	tektonNamespace       = "tekton-pipelines"
	tektonBuildPipeline   = "devflow-tekton-image-build"
	tektonPVCGenerateName = "devflow-tekton-image-build"
)

type imageService struct{}

func (s *imageService) CreateImage(ctx context.Context, m *imagedomain.Image) (uuid.UUID, error) {
	logger := logger.LoggerFromContext(ctx)
	logger.Info("create image start",
		zap.String("application_id", m.ApplicationID.String()),
		zap.String("branch", m.Branch),
	)

	app, err := ApplicationService.Get(ctx, m.ApplicationID)
	if err != nil {
		logger.Error("get application failed", zap.Error(err))
		return uuid.Nil, err
	}

	m.RepoAddress = app.RepoAddress
	if m.RepoAddress == "" {
		m.RepoAddress = app.RepoURL
	}
	if m.Branch == "" {
		m.Branch = "main"
	}
	registryConfig, err := configuredImageRegistry()
	if err != nil {
		logger.Error("image registry config invalid", zap.Error(err))
		return uuid.Nil, err
	}
	imageTarget, err := imagedomain.BuildImageTarget(registryConfig, app.Name, m.Branch, "", time.Now())
	if err != nil {
		logger.Error("build image target failed", zap.Error(err))
		return uuid.Nil, err
	}
	m.Name = imageTarget.Name
	m.Tag = imageTarget.Tag
	m.Status = model.ImagePending
	m.WithCreateDefault()

	if runtime.IsIntentMode() {
		if err := s.insert(ctx, m); err != nil {
			return uuid.Nil, err
		}
		intentID, err := intentservice.IntentService.CreateBuildIntent(ctx, m)
		if err != nil {
			return m.ID, err
		}
		logger.Info("create image success in intent mode", zap.String("image", m.Name), zap.String("intent_id", intentID.String()))
		return m.ID, nil
	}

	if err := s.submitBuild(ctx, m); err != nil {
		return uuid.Nil, err
	}
	if err := s.insert(ctx, m); err != nil {
		return uuid.Nil, err
	}

	logger.Info("create image success", zap.String("image", m.Name), zap.String("pipelineRun", m.PipelineID))
	return m.ID, nil
}

func (s *imageService) insert(ctx context.Context, m *imagedomain.Image) error {
	stepsJSON, err := marshalJSON(m.Steps, "[]")
	if err != nil {
		return err
	}
	_, err = store.DB().ExecContext(ctx, `
		insert into images (
			id, execution_intent_id, application_id, configuration_revision_id, runtime_spec_revision_id, name, tag, branch, repo_address, commit_hash, digest, pipeline_id, steps, status, created_at, updated_at, deleted_at
		) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
	`, m.ID, nullableUUIDPtr(m.ExecutionIntentID), m.ApplicationID, nullableUUIDPtr(m.ConfigurationRevisionID), nullableUUIDPtr(m.RuntimeSpecRevisionID), m.Name, m.Tag, m.Branch, m.RepoAddress, m.CommitHash, m.Digest, m.PipelineID, stepsJSON, m.Status, m.CreatedAt, m.UpdatedAt, m.DeletedAt)
	return err
}

func (s *imageService) DispatchBuild(ctx context.Context, imageID uuid.UUID) error {
	image, err := s.Get(ctx, imageID)
	if err != nil {
		return err
	}
	if err := s.submitBuild(ctx, image); err != nil {
		return err
	}
	return s.updatePipelineAndSteps(ctx, image.ID, image.PipelineID, image.Steps)
}

func (s *imageService) submitBuild(ctx context.Context, m *imagedomain.Image) error {
	logger := logger.LoggerFromContext(ctx)
	registryConfig, err := configuredImageRegistry()
	if err != nil {
		return err
	}
	imageTarget := imagedomain.ImageTarget{
		Name: m.Name,
		Tag:  m.Tag,
		Ref:  registryConfig.Repository() + "/" + m.Name + ":" + m.Tag,
	}

	pvc, err := localtekton.CreatePVC(ctx, tektonNamespace, tektonPVCGenerateName, "local-path", "1Gi")
	if err != nil {
		logger.Error("create pvc failed", zap.Error(err))
		return err
	}

	pctx, span := StartServiceSpan(ctx, "Tekton.CreatePipelineRun")
	defer span.End()

	pr := m.GeneratePipelineRun(tektonBuildPipeline, pvc.Name, registryConfig, imageTarget)
	sc := trace.SpanContextFromContext(pctx)
	pr.Annotations = map[string]string{
		imagedomain.TraceIDAnnotation: sc.TraceID().String(),
		imagedomain.SpanAnnotation:    sc.SpanID().String(),
	}
	pr, err = localtekton.CreatePipelineRun(pctx, tektonNamespace, pr)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	if err := localtekton.PatchPVCOwner(ctx, pvc, pr); err != nil {
		logger.Warn("patch pvc owner failed", zap.Error(err))
	}
	m.PipelineID = pr.Name

	pipeline, err := localtekton.GetPipeline(ctx, pr.Namespace, pr.Spec.PipelineRef.Name)
	if err != nil {
		return err
	}
	m.Steps = BuildStepsFromPipeline(pipeline)
	return nil
}

func configuredImageRegistry() (imagedomain.ImageRegistryConfig, error) {
	cfg := CurrentRuntimeConfig().ImageRegistry
	if cfg.Repository() == "" {
		return imagedomain.ImageRegistryConfig{}, fmt.Errorf("release-service image registry is not configured")
	}
	return cfg, nil
}

func (s *imageService) Update(ctx context.Context, m *imagedomain.Image) error {
	current, err := s.Get(ctx, m.ID)
	if err != nil {
		return err
	}
	m.CreatedAt = current.CreatedAt
	m.DeletedAt = current.DeletedAt
	m.WithUpdateDefault()
	return s.updateRow(ctx, m)
}

func (s *imageService) List(ctx context.Context, filter ImageListFilter) ([]imagedomain.Image, error) {
	query := `
		select id, execution_intent_id, application_id, configuration_revision_id, runtime_spec_revision_id, name, tag, branch, repo_address, commit_hash, digest, pipeline_id, steps, status, created_at, updated_at, deleted_at
		from images
	`
	clauses := make([]string, 0, 6)
	args := make([]any, 0, 6)
	if !filter.IncludeDeleted {
		clauses = append(clauses, "deleted_at is null")
	}
	if filter.ApplicationID != nil {
		args = append(args, *filter.ApplicationID)
		clauses = append(clauses, placeholderClause("application_id", len(args)))
	}
	if filter.PipelineID != "" {
		args = append(args, filter.PipelineID)
		clauses = append(clauses, placeholderClause("pipeline_id", len(args)))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		clauses = append(clauses, placeholderClause("status", len(args)))
	}
	if filter.Branch != "" {
		args = append(args, filter.Branch)
		clauses = append(clauses, placeholderClause("branch", len(args)))
	}
	if filter.Name != "" {
		args = append(args, filter.Name)
		clauses = append(clauses, placeholderClause("name", len(args)))
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
	out := make([]imagedomain.Image, 0)
	for rows.Next() {
		item, err := scanImage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *imageService) Get(ctx context.Context, id uuid.UUID) (*imagedomain.Image, error) {
	return scanImage(store.DB().QueryRowContext(ctx, `
		select id, execution_intent_id, application_id, configuration_revision_id, runtime_spec_revision_id, name, tag, branch, repo_address, commit_hash, digest, pipeline_id, steps, status, created_at, updated_at, deleted_at
		from images
		where id = $1 and deleted_at is null
	`, id))
}

func (s *imageService) AssignPipelineID(ctx context.Context, imageID uuid.UUID, pipelineID string) error {
	if imageID == uuid.Nil {
		return errors.New("image id cannot be zero")
	}
	result, err := store.DB().ExecContext(ctx, `
		update images
		set pipeline_id = $2, updated_at = $3
		where id = $1 and deleted_at is null
	`, imageID, pipelineID, time.Now())
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (s *imageService) UpdateImageStatusByID(ctx context.Context, imageID uuid.UUID, status model.ImageStatus) error {
	if imageID == uuid.Nil {
		return errors.New("image id cannot be zero")
	}
	current, err := s.Get(ctx, imageID)
	if err != nil {
		return err
	}
	current.Status = status
	current.UpdatedAt = time.Now()
	return s.updateStatusAndSteps(ctx, current.ID, current.Status, current.Steps, current.PipelineID)
}

func (s *imageService) UpdateStepStatus(ctx context.Context, pipelineID, taskName string, status model.StepStatus, message string, start, end *time.Time) error {
	image, err := s.GetImageByPipelineID(ctx, pipelineID)
	if err != nil {
		return err
	}
	changed := false
	for i := range image.Steps {
		if image.Steps[i].TaskName != taskName {
			continue
		}
		if image.Steps[i].Status == model.StepFailed || image.Steps[i].Status == model.StepSucceeded || image.Steps[i].Status == status {
			return nil
		}
		image.Steps[i].Status = status
		image.Steps[i].Message = message
		if start != nil {
			image.Steps[i].StartTime = start
		}
		if end != nil {
			image.Steps[i].EndTime = end
		}
		changed = true
	}
	if !changed {
		return nil
	}
	image.UpdatedAt = time.Now()
	return s.updateStatusAndSteps(ctx, image.ID, image.Status, image.Steps, image.PipelineID)
}

func (s *imageService) UpdateImageStatus(ctx context.Context, pipelineID string, status model.ImageStatus) error {
	image, err := s.GetImageByPipelineID(ctx, pipelineID)
	if err != nil {
		return err
	}
	image.Status = status
	image.UpdatedAt = time.Now()
	return s.updateStatusAndSteps(ctx, image.ID, image.Status, image.Steps, image.PipelineID)
}

func BuildStepsFromPipeline(pipeline *v1.Pipeline) []model.ImageTask {
	steps := make([]model.ImageTask, 0, len(pipeline.Spec.Tasks)+len(pipeline.Spec.Finally))
	for _, task := range pipeline.Spec.Tasks {
		steps = append(steps, model.ImageTask{TaskName: task.Name, Status: model.StepPending})
	}
	for _, task := range pipeline.Spec.Finally {
		steps = append(steps, model.ImageTask{TaskName: task.Name, Status: model.StepPending})
	}
	return steps
}

func (s *imageService) BindTaskRun(ctx context.Context, pipelineID, taskName, taskRun string) error {
	image, err := s.GetImageByPipelineID(ctx, pipelineID)
	if err != nil {
		return err
	}
	changed := false
	for i := range image.Steps {
		if image.Steps[i].TaskName == taskName {
			if image.Steps[i].TaskRun == taskRun {
				return nil
			}
			image.Steps[i].TaskRun = taskRun
			changed = true
		}
	}
	if !changed {
		return nil
	}
	image.UpdatedAt = time.Now()
	return s.updateStatusAndSteps(ctx, image.ID, image.Status, image.Steps, image.PipelineID)
}

func (s *imageService) GetImageByPipelineID(ctx context.Context, pipelineID string) (*imagedomain.Image, error) {
	return scanImage(store.DB().QueryRowContext(ctx, `
		select id, execution_intent_id, application_id, configuration_revision_id, runtime_spec_revision_id, name, tag, branch, repo_address, commit_hash, digest, pipeline_id, steps, status, created_at, updated_at, deleted_at
		from images
		where pipeline_id = $1 and deleted_at is null
	`, pipelineID))
}

func (s *imageService) Patch(ctx context.Context, id uuid.UUID, patch *imagedomain.PatchImageRequest) error {
	if patch == nil || patch.IsEmpty() {
		return nil
	}
	image, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if patch.Digest != "" {
		image.Digest = patch.Digest
	}
	if patch.CommitHash != "" {
		image.CommitHash = patch.CommitHash
	}
	if patch.Tag != "" {
		image.Tag = patch.Tag
	}
	image.UpdatedAt = time.Now()
	return s.updateRow(ctx, image)
}

func (s *imageService) updatePipelineAndSteps(ctx context.Context, id uuid.UUID, pipelineID string, steps []model.ImageTask) error {
	stepsJSON, err := marshalJSON(steps, "[]")
	if err != nil {
		return err
	}
	result, err := store.DB().ExecContext(ctx, `
		update images
		set pipeline_id = $2, steps = $3, updated_at = $4
		where id = $1 and deleted_at is null
	`, id, pipelineID, stepsJSON, time.Now())
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (s *imageService) updateStatusAndSteps(ctx context.Context, id uuid.UUID, status model.ImageStatus, steps []model.ImageTask, pipelineID string) error {
	stepsJSON, err := marshalJSON(steps, "[]")
	if err != nil {
		return err
	}
	result, err := store.DB().ExecContext(ctx, `
		update images
		set status = $2, steps = $3, pipeline_id = $4, updated_at = $5
		where id = $1 and deleted_at is null
	`, id, status, stepsJSON, pipelineID, time.Now())
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (s *imageService) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := store.DB().ExecContext(ctx, `
		update images
		set deleted_at = $2, updated_at = $2
		where id = $1 and deleted_at is null
	`, id, time.Now())
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (s *imageService) updateRow(ctx context.Context, m *imagedomain.Image) error {
	stepsJSON, err := marshalJSON(m.Steps, "[]")
	if err != nil {
		return err
	}
	result, err := store.DB().ExecContext(ctx, `
		update images
		set execution_intent_id=$2, application_id=$3, configuration_revision_id=$4, runtime_spec_revision_id=$5, name=$6, tag=$7, branch=$8, repo_address=$9, commit_hash=$10, digest=$11, pipeline_id=$12, steps=$13, status=$14, updated_at=$15, deleted_at=$16
		where id=$1 and deleted_at is null
	`, m.ID, nullableUUIDPtr(m.ExecutionIntentID), m.ApplicationID, nullableUUIDPtr(m.ConfigurationRevisionID), nullableUUIDPtr(m.RuntimeSpecRevisionID), m.Name, m.Tag, m.Branch, m.RepoAddress, m.CommitHash, m.Digest, m.PipelineID, stepsJSON, m.Status, m.UpdatedAt, m.DeletedAt)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}
