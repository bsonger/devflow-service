package service

import (
	"context"
	"fmt"
	"time"

	imagedomain "github.com/bsonger/devflow-service/internal/image/domain"
	"github.com/bsonger/devflow-service/internal/image/repository"
	intentservice "github.com/bsonger/devflow-service/internal/intent/service"
	"github.com/bsonger/devflow-service/internal/platform/logger"
	"github.com/bsonger/devflow-service/internal/platform/runtime/observability"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/bsonger/devflow-service/internal/release/runtime"
	releasesupport "github.com/bsonger/devflow-service/internal/release/support"
	localtekton "github.com/bsonger/devflow-service/internal/release/transport/tekton"
	sharederrs "github.com/bsonger/devflow-service/internal/shared/errs"
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

var ImageService = &imageService{store: repository.NewPostgresStore()}

const (
	tektonNamespace       = "tekton-pipelines"
	tektonBuildPipeline   = "devflow-tekton-image-build"
	tektonPVCGenerateName = "devflow-tekton-image-build"
)

type imageService struct {
	store repository.Store
}

func (s *imageService) repoStore() repository.Store {
	if s.store == nil {
		s.store = repository.NewPostgresStore()
	}
	return s.store
}

func (s *imageService) CreateImage(ctx context.Context, m *imagedomain.Image) (uuid.UUID, error) {
	logger := logger.LoggerFromContext(ctx)
	if logger == nil {
		logger = zap.NewNop()
	}
	logger.Info("create image start",
		zap.String("operation", "create_image"),
		zap.String("resource", "image"),
		zap.String("result", "started"),
		zap.String("application_id", m.ApplicationID.String()),
		zap.String("branch", m.Branch),
	)

	app, err := releasesupport.ApplicationService.Get(ctx, m.ApplicationID)
	if err != nil {
		logger.Error("get application failed",
			zap.String("operation", "create_image"),
			zap.String("resource", "image"),
			zap.String("result", "error"),
			zap.Error(err),
		)
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
		logger.Error("image registry config invalid",
			zap.String("operation", "create_image"),
			zap.String("resource", "image"),
			zap.String("result", "error"),
			zap.Error(err),
		)
		return uuid.Nil, err
	}
	imageTarget, err := imagedomain.BuildImageTarget(registryConfig, app.Name, m.Branch, "", time.Now())
	if err != nil {
		logger.Error("build image target failed",
			zap.String("operation", "create_image"),
			zap.String("resource", "image"),
			zap.String("result", "error"),
			zap.Error(err),
		)
		return uuid.Nil, err
	}
	m.Name = imageTarget.Name
	m.Tag = imageTarget.Tag
	m.Status = model.ImagePending
	m.WithCreateDefault()

	if runtime.IsIntentMode() {
		if err := s.repoStore().Insert(ctx, m); err != nil {
			return uuid.Nil, err
		}
		intentID, err := intentservice.IntentService.CreateBuildIntent(ctx, m)
		if err != nil {
			return m.ID, err
		}
		logger.Info("create image success in intent mode",
			zap.String("operation", "create_image"),
			zap.String("resource", "image"),
			zap.String("resource_id", m.ID.String()),
			zap.String("result", "success"),
			zap.String("image_name", m.Name),
			zap.String("intent_id", intentID.String()),
		)
		return m.ID, nil
	}

	if err := s.submitBuild(ctx, m); err != nil {
		return uuid.Nil, err
	}
	if err := s.repoStore().Insert(ctx, m); err != nil {
		return uuid.Nil, err
	}

	logger.Info("create image success",
		zap.String("operation", "create_image"),
		zap.String("resource", "image"),
		zap.String("resource_id", m.ID.String()),
		zap.String("result", "success"),
		zap.String("image_name", m.Name),
		zap.String("pipeline_run_id", m.PipelineID),
	)
	return m.ID, nil
}

func (s *imageService) DispatchBuild(ctx context.Context, imageID uuid.UUID) error {
	image, err := s.Get(ctx, imageID)
	if err != nil {
		return err
	}
	if err := s.submitBuild(ctx, image); err != nil {
		return err
	}
	return s.repoStore().UpdatePipelineAndSteps(ctx, image.ID, image.PipelineID, image.Steps)
}

func (s *imageService) submitBuild(ctx context.Context, m *imagedomain.Image) error {
	logger := logger.LoggerFromContext(ctx)
	if logger == nil {
		logger = zap.NewNop()
	}
	log := logger.With(
		zap.String("operation", "submit_image_build"),
		zap.String("resource", "image"),
		zap.String("resource_id", m.ID.String()),
		zap.String("image_name", m.Name),
	)
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
		log.Error("create pvc failed", zap.String("result", "error"), zap.Error(err))
		return err
	}

	pctx, span := observability.StartServiceSpan(ctx, "Tekton.CreatePipelineRun")
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
		log.Error("create pipeline run failed", zap.String("result", "error"), zap.Error(err))
		return err
	}
	if err := localtekton.PatchPVCOwner(ctx, pvc, pr); err != nil {
		log.Warn("patch pvc owner failed", zap.String("result", "error"), zap.Error(err))
	}
	m.PipelineID = pr.Name
	log.Info("pipeline run created",
		zap.String("result", "success"),
		zap.String("pipeline_run_id", pr.Name),
		zap.String("namespace", pr.Namespace),
	)

	pipeline, err := localtekton.GetPipeline(ctx, pr.Namespace, pr.Spec.PipelineRef.Name)
	if err != nil {
		log.Error("get pipeline definition failed", zap.String("result", "error"), zap.Error(err))
		return err
	}
	m.Steps = BuildStepsFromPipeline(pipeline)
	return nil
}

func configuredImageRegistry() (imagedomain.ImageRegistryConfig, error) {
	cfg := releasesupport.CurrentRuntimeConfig().ImageRegistry
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
	return s.repoStore().UpdateRow(ctx, m)
}

func (s *imageService) List(ctx context.Context, filter ImageListFilter) ([]imagedomain.Image, error) {
	return s.repoStore().List(ctx, repository.ListFilter(filter))
}

func (s *imageService) Get(ctx context.Context, id uuid.UUID) (*imagedomain.Image, error) {
	return s.repoStore().Get(ctx, id)
}

func (s *imageService) AssignPipelineID(ctx context.Context, imageID uuid.UUID, pipelineID string) error {
	if imageID == uuid.Nil {
		return sharederrs.InvalidArgument("image id cannot be zero")
	}
	return s.repoStore().AssignPipelineID(ctx, imageID, pipelineID)
}

func (s *imageService) UpdateImageStatusByID(ctx context.Context, imageID uuid.UUID, status model.ImageStatus) error {
	if imageID == uuid.Nil {
		return sharederrs.InvalidArgument("image id cannot be zero")
	}
	current, err := s.Get(ctx, imageID)
	if err != nil {
		return err
	}
	current.Status = status
	current.UpdatedAt = time.Now()
	return s.repoStore().UpdateStatusAndSteps(ctx, current.ID, current.Status, current.Steps, current.PipelineID)
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
	return s.repoStore().UpdateStatusAndSteps(ctx, image.ID, image.Status, image.Steps, image.PipelineID)
}

func (s *imageService) UpdateImageStatus(ctx context.Context, pipelineID string, status model.ImageStatus) error {
	image, err := s.GetImageByPipelineID(ctx, pipelineID)
	if err != nil {
		return err
	}
	image.Status = status
	image.UpdatedAt = time.Now()
	return s.repoStore().UpdateStatusAndSteps(ctx, image.ID, image.Status, image.Steps, image.PipelineID)
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
	return s.repoStore().UpdateStatusAndSteps(ctx, image.ID, image.Status, image.Steps, image.PipelineID)
}

func (s *imageService) GetImageByPipelineID(ctx context.Context, pipelineID string) (*imagedomain.Image, error) {
	return s.repoStore().GetByPipelineID(ctx, pipelineID)
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
	return s.repoStore().UpdateRow(ctx, image)
}

func (s *imageService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repoStore().Delete(ctx, id)
}
