package service

import (
	"context"
	"strings"
	"time"

	appconfigdownstream "github.com/bsonger/devflow-service/internal/appconfig/transport/downstream"
	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	"github.com/bsonger/devflow-service/internal/manifest/repository"
	"github.com/bsonger/devflow-service/internal/platform/logger"
	"github.com/bsonger/devflow-service/internal/platform/oci"
	"github.com/bsonger/devflow-service/internal/platform/runtime/observability"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	releasesupport "github.com/bsonger/devflow-service/internal/release/support"
	localtekton "github.com/bsonger/devflow-service/internal/release/transport/tekton"
	servicedownstream "github.com/bsonger/devflow-service/internal/service/transport/downstream"
	sharederrs "github.com/bsonger/devflow-service/internal/shared/errs"
	"github.com/google/uuid"
	tknv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ErrManifestWorkloadConfigMissing = sharederrs.FailedPrecondition("effective workload config is missing")

const (
	manifestTektonNamespace       = "tekton-pipelines"
	manifestTektonBuildPipeline   = "devflow-tekton-image-build-push-only"
	manifestTektonPVCGenerateName = "devflow-tekton-image-build-push-only"
)

var (
	manifestCreatePVC         = localtekton.CreatePVC
	manifestCreatePipelineRun = localtekton.CreatePipelineRun
	manifestPatchPVCOwner     = localtekton.PatchPVCOwner
	manifestGetPipeline       = localtekton.GetPipeline
)

var ManifestService = NewManifestService()

type manifestNetworkReader interface {
	ListServices(context.Context, string) ([]servicedownstream.Service, error)
}

type manifestConfigReader interface {
	FindWorkloadConfig(context.Context, string) (*appconfigdownstream.WorkloadConfig, error)
}

type manifestService struct {
	apps interface {
		Get(context.Context, uuid.UUID) (*releasesupport.ApplicationProjection, error)
	}
	store repository.Store
}

func (s *manifestService) repoStore() repository.Store {
	if s.store == nil {
		s.store = repository.NewPostgresStore()
	}
	return s.store
}

func NewManifestService() *manifestService {
	return &manifestService{
		apps:  releasesupport.ApplicationService,
		store: repository.NewPostgresStore(),
	}
}

func (s *manifestService) CreateManifest(ctx context.Context, req *manifestdomain.CreateManifestRequest) (*manifestdomain.Manifest, error) {
	req.GitRevision = normalizeGitRevision(req.GitRevision)

	log := logger.LoggerWithContext(ctx).With(
		zap.String("operation", "create_manifest"),
		zap.String("resource", "manifest"),
		zap.String("result", "started"),
		zap.String("application_id", req.ApplicationID.String()),
		zap.String("git_revision", strings.TrimSpace(req.GitRevision)),
	)

	runtimeCfg := releasesupport.CurrentRuntimeConfig()
	networks := servicedownstream.New(strings.TrimSpace(runtimeCfg.Downstream.NetworkServiceBaseURL))
	configs := appconfigdownstream.New(strings.TrimSpace(runtimeCfg.Downstream.ConfigServiceBaseURL))

	application, err := s.apps.Get(ctx, req.ApplicationID)
	if err != nil {
		return nil, err
	}

	workloadConfig, err := configs.FindWorkloadConfig(ctx, req.ApplicationID.String())
	if err != nil {
		return nil, err
	}
	if workloadConfig == nil {
		return nil, ErrManifestWorkloadConfigMissing
	}
	services, err := networks.ListServices(ctx, req.ApplicationID.String())
	if err != nil {
		return nil, err
	}
	imageTarget, err := oci.BuildImageTarget(runtimeCfg.ImageRegistry, application.Name, "main", "", time.Now())
	if err != nil {
		return nil, err
	}
	manifest, err := buildManifest(req, application.Name, application.RepoAddress, imageTarget, "", workloadConfig, services)
	if err != nil {
		return nil, err
	}
	manifest.WithCreateDefault()
	if err := submitManifestBuild(ctx, manifest, runtimeCfg.ImageRegistry.Repository(), imageTarget); err != nil {
		log.Error("submit manifest build failed", zap.String("result", "error"), zap.Error(err))
		return nil, err
	}
	if err := s.repoStore().Insert(ctx, manifest); err != nil {
		log.Error("persist manifest failed", zap.String("result", "error"), zap.Error(err))
		return nil, err
	}
	log.Info("manifest created",
		zap.String("result", "success"),
		zap.String("resource_id", manifest.ID.String()),
		zap.String("pipeline_id", manifest.PipelineID),
		zap.String("image_ref", manifest.ImageRef),
	)
	return manifest, nil
}

func normalizeGitRevision(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "main"
	}
	return trimmed
}

func submitManifestBuild(ctx context.Context, manifest *manifestdomain.Manifest, imageRegistry string, target oci.ImageTarget) error {
	if manifest == nil {
		return sharederrs.Required("manifest")
	}

	pvc, err := manifestCreatePVC(ctx, manifestTektonNamespace, manifestTektonPVCGenerateName, "local-path", "1Gi")
	if err != nil {
		return err
	}

	pctx, span := observability.StartServiceSpan(ctx, "Tekton.CreateManifestPipelineRun")
	defer span.End()

	pr := buildManifestPipelineRun(manifest, pvc.Name, imageRegistry, target)
	sc := trace.SpanContextFromContext(pctx)
	if pr.Annotations == nil {
		pr.Annotations = map[string]string{}
	}
	pr.Annotations[oci.TraceIDAnnotation] = sc.TraceID().String()
	pr.Annotations[oci.SpanAnnotation] = sc.SpanID().String()

	pr, err = manifestCreatePipelineRun(pctx, manifestTektonNamespace, pr)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	if err := manifestPatchPVCOwner(ctx, pvc, pr); err != nil {
		if log := logger.LoggerFromContext(ctx); log != nil {
			log.Warn("patch pvc owner failed", zap.Error(err))
		}
	}

	pipeline, err := manifestGetPipeline(ctx, pr.Namespace, pr.Spec.PipelineRef.Name)
	if err != nil {
		return err
	}

	manifest.PipelineID = pr.Name
	manifest.TraceID = sc.TraceID().String()
	manifest.SpanID = sc.SpanID().String()
	manifest.Steps = buildManifestStepsFromPipeline(pipeline)
	manifest.Status = model.ManifestPending
	return nil
}

func buildManifestPipelineRun(manifest *manifestdomain.Manifest, pvcName, imageRegistry string, target oci.ImageTarget) *tknv1.PipelineRun {
	return &tknv1.PipelineRun{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PipelineRun",
			APIVersion: "tekton.dev/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: manifestTektonBuildPipeline + "-run-",
			Labels: map[string]string{
				"devflow.manifest/id": manifest.ID.String(),
			},
			Annotations: map[string]string{
				"devflow.manifest/id": manifest.ID.String(),
			},
		},
		Spec: tknv1.PipelineRunSpec{
			PipelineRef: &tknv1.PipelineRef{Name: manifestTektonBuildPipeline},
			Params: []tknv1.Param{
				{Name: "git-url", Value: tknv1.ParamValue{Type: tknv1.ParamTypeString, StringVal: manifest.RepoAddress}},
				{Name: "git-revision", Value: tknv1.ParamValue{Type: tknv1.ParamTypeString, StringVal: manifest.GitRevision}},
				{Name: "image-registry", Value: tknv1.ParamValue{Type: tknv1.ParamTypeString, StringVal: imageRegistry}},
				{Name: "name", Value: tknv1.ParamValue{Type: tknv1.ParamTypeString, StringVal: target.Name}},
				{Name: "image-tag", Value: tknv1.ParamValue{Type: tknv1.ParamTypeString, StringVal: target.Tag}},
			},
			Workspaces: []tknv1.WorkspaceBinding{
				{
					Name: "source",
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: pvcName,
					},
				},
				{
					Name:   "dockerconfig",
					Secret: &corev1.SecretVolumeSource{SecretName: "aliyun-docker-config"},
				},
				{
					Name:   "ssh",
					Secret: &corev1.SecretVolumeSource{SecretName: "git-ssh-secret"},
				},
			},
		},
	}
}

func buildManifestStepsFromPipeline(pipeline *tknv1.Pipeline) []model.ImageTask {
	if pipeline == nil {
		return nil
	}
	steps := make([]model.ImageTask, 0, len(pipeline.Spec.Tasks)+len(pipeline.Spec.Finally))
	for _, task := range pipeline.Spec.Tasks {
		steps = append(steps, model.ImageTask{TaskName: task.Name, Status: model.StepPending})
	}
	for _, task := range pipeline.Spec.Finally {
		steps = append(steps, model.ImageTask{TaskName: task.Name, Status: model.StepPending})
	}
	return steps
}

func (s *manifestService) List(ctx context.Context, filter manifestdomain.ManifestListFilter) ([]manifestdomain.Manifest, error) {
	return s.repoStore().List(ctx, filter)
}

func (s *manifestService) GetByPipelineID(ctx context.Context, pipelineID string) (*manifestdomain.Manifest, error) {
	return s.repoStore().GetByPipelineID(ctx, pipelineID)
}

func (s *manifestService) GetResources(ctx context.Context, id uuid.UUID) (*manifestdomain.ManifestResourcesView, error) {
	item, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return buildManifestResourcesView(item)
}

func (s *manifestService) Get(ctx context.Context, id uuid.UUID) (*manifestdomain.Manifest, error) {
	return s.repoStore().Get(ctx, id)
}

func (s *manifestService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repoStore().Delete(ctx, id)
}

func (s *manifestService) AssignPipelineID(ctx context.Context, manifestID uuid.UUID, pipelineID string) error {
	if manifestID == uuid.Nil {
		return sharederrs.InvalidArgument("manifest id cannot be zero")
	}
	return s.repoStore().AssignPipelineID(ctx, manifestID, strings.TrimSpace(pipelineID))
}

func (s *manifestService) UpdateManifestStatusByID(ctx context.Context, manifestID uuid.UUID, status model.ManifestStatus) error {
	if manifestID == uuid.Nil {
		return sharederrs.InvalidArgument("manifest id cannot be zero")
	}
	current, err := s.Get(ctx, manifestID)
	if err != nil {
		return err
	}
	current.Status = convergeManifestStatus(current.Status, current.Steps, current.CommitHash, current.ImageRef, current.ImageTag, current.ImageDigest, status)
	current.UpdatedAt = time.Now()
	return s.repoStore().UpdateStatusAndSteps(ctx, current.ID, current.Status, current.Steps, current.PipelineID)
}

func (s *manifestService) UpdateStepStatus(ctx context.Context, pipelineID, taskName string, status model.StepStatus, message string, start, end *time.Time) error {
	manifest, err := s.GetByPipelineID(ctx, pipelineID)
	if err != nil {
		return err
	}
	changed := false
	for i := range manifest.Steps {
		if manifest.Steps[i].TaskName != taskName {
			continue
		}
		if manifest.Steps[i].Status == model.StepFailed || manifest.Steps[i].Status == model.StepSucceeded || manifest.Steps[i].Status == status {
			return nil
		}
		manifest.Steps[i].Status = status
		manifest.Steps[i].Message = message
		if start != nil {
			manifest.Steps[i].StartTime = start
		}
		if end != nil {
			manifest.Steps[i].EndTime = end
		}
		changed = true
	}
	if !changed {
		return nil
	}
	manifest.Status = convergeManifestStatus(manifest.Status, manifest.Steps, manifest.CommitHash, manifest.ImageRef, manifest.ImageTag, manifest.ImageDigest, manifest.Status)
	manifest.UpdatedAt = time.Now()
	return s.repoStore().UpdateStatusAndSteps(ctx, manifest.ID, manifest.Status, manifest.Steps, manifest.PipelineID)
}

func (s *manifestService) BindTaskRun(ctx context.Context, pipelineID, taskName, taskRun string) error {
	manifest, err := s.GetByPipelineID(ctx, pipelineID)
	if err != nil {
		return err
	}
	changed := false
	for i := range manifest.Steps {
		if manifest.Steps[i].TaskName == taskName {
			if manifest.Steps[i].TaskRun == taskRun {
				return nil
			}
			manifest.Steps[i].TaskRun = taskRun
			changed = true
		}
	}
	if !changed {
		return nil
	}
	manifest.UpdatedAt = time.Now()
	return s.repoStore().UpdateStatusAndSteps(ctx, manifest.ID, manifest.Status, manifest.Steps, manifest.PipelineID)
}

func (s *manifestService) UpdateManifestStatus(ctx context.Context, pipelineID string, status model.ManifestStatus) error {
	manifest, err := s.GetByPipelineID(ctx, pipelineID)
	if err != nil {
		return err
	}
	manifest.Status = convergeManifestStatus(manifest.Status, manifest.Steps, manifest.CommitHash, manifest.ImageRef, manifest.ImageTag, manifest.ImageDigest, status)
	manifest.UpdatedAt = time.Now()
	return s.repoStore().UpdateStatusAndSteps(ctx, manifest.ID, manifest.Status, manifest.Steps, manifest.PipelineID)
}

func (s *manifestService) UpdateBuildResult(ctx context.Context, pipelineID, commitHash, imageRef, imageTag, imageDigest string) error {
	manifest, err := s.GetByPipelineID(ctx, pipelineID)
	if err != nil {
		return err
	}
	manifest.CommitHash = strings.TrimSpace(commitHash)
	manifest.ImageRef = strings.TrimSpace(imageRef)
	manifest.ImageTag = strings.TrimSpace(imageTag)
	manifest.ImageDigest = strings.TrimSpace(imageDigest)
	status := convergeManifestStatus(manifest.Status, manifest.Steps, manifest.CommitHash, manifest.ImageRef, manifest.ImageTag, manifest.ImageDigest, manifest.Status)
	return s.repoStore().UpdateBuildResult(ctx, manifest.ID, commitHash, imageRef, imageTag, imageDigest, status)
}

func convergeManifestStatus(current model.ManifestStatus, steps []model.ImageTask, commitHash, imageRef, imageTag, imageDigest string, requested model.ManifestStatus) model.ManifestStatus {
	switch current {
	case model.ManifestFailed:
		return current
	}
	if requested == model.ManifestFailed {
		return model.ManifestFailed
	}

	anyRunning := false
	anyStarted := false
	anyFailed := false
	allSucceeded := len(steps) > 0

	for _, step := range steps {
		switch step.Status {
		case model.StepFailed:
			anyFailed = true
			allSucceeded = false
		case model.StepSucceeded:
			anyStarted = true
		case model.StepRunning:
			anyRunning = true
			anyStarted = true
			allSucceeded = false
		default:
			allSucceeded = false
		}
	}

	if anyFailed {
		return model.ManifestFailed
	}
	if anyRunning {
		return model.ManifestRunning
	}
	if allSucceeded {
		if hasManifestBuildResult(commitHash, imageRef, imageTag, imageDigest) {
			return model.ManifestSucceeded
		}
		return model.ManifestReady
	}
	if anyStarted {
		return model.ManifestRunning
	}

	switch requested {
	case model.ManifestRunning:
		return model.ManifestRunning
	case model.ManifestReady:
		if hasManifestBuildResult(commitHash, imageRef, imageTag, imageDigest) {
			return model.ManifestSucceeded
		}
		return model.ManifestReady
	case model.ManifestPending:
		return model.ManifestPending
	}

	if current == "" {
		return model.ManifestPending
	}
	return current
}

func hasManifestBuildResult(commitHash, imageRef, imageTag, imageDigest string) bool {
	return strings.TrimSpace(commitHash) != "" ||
		strings.TrimSpace(imageRef) != "" ||
		strings.TrimSpace(imageTag) != "" ||
		strings.TrimSpace(imageDigest) != ""
}

func buildManifestResourcesView(item *manifestdomain.Manifest) (*manifestdomain.ManifestResourcesView, error) {
	if item == nil {
		return nil, nil
	}
	applicationName := deriveManifestResourceApplicationName(item)
	resources, err := renderManifestResources("", applicationName, item.ApplicationID.String(), item.WorkloadConfigSnapshot, item.ServicesSnapshot, item.ImageRef, map[string]string{})
	if err != nil {
		return nil, err
	}
	view := &manifestdomain.ManifestResourcesView{
		ManifestID:    item.ID,
		ApplicationID: item.ApplicationID,
		Resources:     manifestdomain.ManifestGroupedResources{Services: []manifestdomain.ManifestRenderedResource{}},
	}
	for _, rendered := range resources {
		switch strings.ToLower(strings.TrimSpace(rendered.Kind)) {
		case "configmap":
			view.Resources.ConfigMap = &rendered
		case "deployment":
			view.Resources.Deployment = &rendered
		case "rollout":
			view.Resources.Rollout = &rendered
		case "service":
			view.Resources.Services = append(view.Resources.Services, rendered)
		case "virtualservice":
			view.Resources.VirtualService = &rendered
		}
	}
	return view, nil
}

func deriveManifestResourceApplicationName(item *manifestdomain.Manifest) string {
	if item == nil {
		return ""
	}
	if len(item.ServicesSnapshot) > 0 && strings.TrimSpace(item.ServicesSnapshot[0].Name) != "" {
		return strings.TrimSpace(item.ServicesSnapshot[0].Name)
	}
	return item.ApplicationID.String()
}

func buildManifest(req *manifestdomain.CreateManifestRequest, applicationName, repoAddress string, target oci.ImageTarget, imageDigest string, workload *appconfigdownstream.WorkloadConfig, services []servicedownstream.Service) (*manifestdomain.Manifest, error) {
	servicesSnapshot := make([]manifestdomain.ManifestService, 0, len(services))
	for _, item := range services {
		ports := make([]manifestdomain.ManifestServicePort, 0, len(item.Ports))
		for _, port := range item.Ports {
			ports = append(ports, manifestdomain.ManifestServicePort{Name: port.Name, ServicePort: port.ServicePort, TargetPort: port.TargetPort, Protocol: port.Protocol})
		}
		servicesSnapshot = append(servicesSnapshot, manifestdomain.ManifestService{ID: item.ID, Name: item.Name, Ports: ports})
	}
	imageRef, _, err := resolveWorkloadImageRef(target.Ref[:strings.LastIndex(target.Ref, ":")], target.Tag, imageDigest)
	if err != nil {
		return nil, err
	}

	workloadSnapshot := manifestdomain.ManifestWorkloadConfig{
		ID:                 workload.ID,
		Replicas:           workload.Replicas,
		ServiceAccountName: workload.ServiceAccountName,
		Resources:          workload.Resources,
		Probes:             workload.Probes,
		Env:                toModelEnvVars(workload.Env),
		Labels:             workload.Labels,
		Annotations:        workload.Annotations,
	}

	return &manifestdomain.Manifest{
		ApplicationID:          req.ApplicationID,
		GitRevision:            req.GitRevision,
		RepoAddress:            repoAddress,
		ImageRef:               imageRef,
		ImageTag:               target.Tag,
		ServicesSnapshot:       servicesSnapshot,
		WorkloadConfigSnapshot: workloadSnapshot,
		Status:                 model.ManifestReady,
	}, nil
}

func toModelEnvVars(items []appconfigdownstream.EnvVar) []model.EnvVar {
	out := make([]model.EnvVar, 0, len(items))
	for _, item := range items {
		out = append(out, model.EnvVar{Name: item.Name, Value: item.Value})
	}
	return out
}
