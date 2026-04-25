package service

import (
	"context"
	"fmt"
	"strings"

	appconfigdownstream "github.com/bsonger/devflow-service/internal/appconfig/transport/downstream"
	appservicedownstream "github.com/bsonger/devflow-service/internal/appservice/transport/downstream"
	imagedomain "github.com/bsonger/devflow-service/internal/image/domain"
	imageservice "github.com/bsonger/devflow-service/internal/image/service"
	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	"github.com/bsonger/devflow-service/internal/manifest/repository"
	"github.com/bsonger/devflow-service/internal/platform/logger"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	releasesupport "github.com/bsonger/devflow-service/internal/release/support"
	sharederrs "github.com/bsonger/devflow-service/internal/shared/errs"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"sigs.k8s.io/yaml"
)

var (
	ErrManifestImageApplicationMismatch = sharederrs.FailedPrecondition("image does not belong to application")
	ErrManifestAppConfigMissing         = sharederrs.FailedPrecondition("effective app config is missing")
	ErrManifestWorkloadConfigMissing    = sharederrs.FailedPrecondition("effective workload config is missing")
	ErrManifestRouteTargetInvalid       = sharederrs.FailedPrecondition("route points to missing service or port")
	ErrManifestImageRepositoryMissing   = sharederrs.FailedPrecondition("image repository is not deployable")
)

var ManifestService = NewManifestService()

type manifestImageReader interface {
	Get(context.Context, uuid.UUID) (*imagedomain.Image, error)
}

type manifestNetworkReader interface {
	ListServices(context.Context, string) ([]appservicedownstream.Service, error)
	ListRoutes(context.Context, string) ([]appservicedownstream.Route, error)
}

type manifestConfigReader interface {
	FindAppConfig(context.Context, string, string) (*appconfigdownstream.AppConfig, error)
	FindWorkloadConfig(context.Context, string, string) (*appconfigdownstream.WorkloadConfig, error)
}

type manifestService struct {
	images manifestImageReader
	apps   interface {
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
		images: imageservice.ImageService,
		apps:   releasesupport.ApplicationService,
		store:  repository.NewPostgresStore(),
	}
}

func (s *manifestService) CreateManifest(ctx context.Context, req *manifestdomain.CreateManifestRequest) (*manifestdomain.Manifest, error) {
	log := logger.LoggerWithContext(ctx).With(
		zap.String("operation", "create_manifest"),
		zap.String("resource", "manifest"),
		zap.String("result", "started"),
		zap.String("application_id", req.ApplicationID.String()),
		zap.String("environment_id", req.EnvironmentID),
		zap.String("image_id", req.ImageID.String()),
	)

	runtimeCfg := releasesupport.CurrentRuntimeConfig()
	networks := appservicedownstream.New(strings.TrimSpace(runtimeCfg.Downstream.NetworkServiceBaseURL))
	configs := appconfigdownstream.New(strings.TrimSpace(runtimeCfg.Downstream.ConfigServiceBaseURL))
	artifacts := newManifestArtifactPublishing(runtimeCfg.ManifestRegistry, runtimeCfg.ManifestRegistryEnabled)

	image, err := s.images.Get(ctx, req.ImageID)
	if err != nil {
		return nil, err
	}
	if image.ApplicationID != req.ApplicationID {
		return nil, ErrManifestImageApplicationMismatch
	}
	target, err := releasesupport.ResolveDeployTarget(ctx, req.ApplicationID.String(), req.EnvironmentID)
	if err != nil {
		return nil, err
	}
	appConfig, err := configs.FindAppConfig(ctx, req.ApplicationID.String(), req.EnvironmentID)
	if err != nil {
		return nil, err
	}
	if appConfig == nil || (len(appConfig.Files) == 0 && len(appConfig.RenderedConfigMap) == 0) {
		return nil, ErrManifestAppConfigMissing
	}
	workloadConfig, err := configs.FindWorkloadConfig(ctx, req.ApplicationID.String(), req.EnvironmentID)
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
	routes, err := networks.ListRoutes(ctx, req.ApplicationID.String())
	if err != nil {
		return nil, err
	}
	application, err := s.apps.Get(ctx, req.ApplicationID)
	if err != nil {
		return nil, err
	}

	manifest, err := buildManifest(req, image, application.Name, appConfig, workloadConfig, services, routes, target.Namespace, runtimeCfg.ImageRegistry)
	if err != nil {
		return nil, err
	}
	manifest.WithCreateDefault()
	if err := publishManifestArtifact(ctx, manifest, application.Name, runtimeCfg.ManifestRegistry, artifacts); err != nil {
		log.Error("publish manifest artifact failed", zap.String("result", "error"), zap.Error(err))
		return nil, err
	}
	if err := s.repoStore().Insert(ctx, manifest); err != nil {
		log.Error("persist manifest failed", zap.String("result", "error"), zap.Error(err))
		return nil, err
	}
	log.Info("manifest created",
		zap.String("result", "success"),
		zap.String("resource_id", manifest.ID.String()),
		zap.String("artifact_repository", manifest.ArtifactRepository),
		zap.String("artifact_digest", manifest.ArtifactDigest),
	)
	return manifest, nil
}

func resolveManifestImageRepository(image *imagedomain.Image, cfg imagedomain.ImageRegistryConfig) (string, error) {
	if image == nil {
		return "", sharederrs.Required("image")
	}
	name := strings.Trim(strings.TrimSpace(image.Name), "/")
	if name == "" {
		return "", sharederrs.Required("image_name")
	}
	repoAddress := strings.TrimSpace(image.RepoAddress)
	if looksLikeContainerRepository(repoAddress) {
		return strings.TrimRight(repoAddress, "/") + "/" + name, nil
	}
	repository := strings.TrimRight(cfg.Repository(), "/")
	if repository == "" {
		return "", ErrManifestImageRepositoryMissing
	}
	return repository + "/" + name, nil
}

func looksLikeContainerRepository(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "git@") || strings.Contains(lower, "://") || strings.HasSuffix(lower, ".git") || strings.Contains(lower, "github.com") || strings.Contains(lower, "gitlab") {
		return false
	}
	return true
}

func (s *manifestService) List(ctx context.Context, filter manifestdomain.ManifestListFilter) ([]manifestdomain.Manifest, error) {
	return s.repoStore().List(ctx, filter)
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

func buildManifestResourcesView(item *manifestdomain.Manifest) (*manifestdomain.ManifestResourcesView, error) {
	if item == nil {
		return nil, nil
	}
	view := &manifestdomain.ManifestResourcesView{
		ManifestID:      item.ID,
		ApplicationID:   item.ApplicationID,
		EnvironmentID:   item.EnvironmentID,
		Resources:       manifestdomain.ManifestGroupedResources{Services: []manifestdomain.ManifestRenderedResource{}},
		RenderedObjects: make([]manifestdomain.ManifestRenderedResource, 0, len(item.RenderedObjects)),
	}
	for _, rendered := range item.RenderedObjects {
		decoded, err := decodeManifestRenderedResource(rendered)
		if err != nil {
			return nil, err
		}
		view.RenderedObjects = append(view.RenderedObjects, decoded)
		switch strings.ToLower(strings.TrimSpace(rendered.Kind)) {
		case "configmap":
			view.Resources.ConfigMap = &decoded
		case "deployment":
			view.Resources.Deployment = &decoded
		case "rollout":
			view.Resources.Rollout = &decoded
		case "service":
			view.Resources.Services = append(view.Resources.Services, decoded)
		case "virtualservice":
			view.Resources.VirtualService = &decoded
		}
	}
	return view, nil
}

func decodeManifestRenderedResource(item manifestdomain.ManifestRenderedObject) (manifestdomain.ManifestRenderedResource, error) {
	decoded := manifestdomain.ManifestRenderedResource{
		Kind:      item.Kind,
		Name:      item.Name,
		Namespace: item.Namespace,
		YAML:      item.YAML,
	}
	if strings.TrimSpace(item.YAML) == "" {
		return decoded, nil
	}
	var object map[string]any
	if err := yaml.Unmarshal([]byte(item.YAML), &object); err != nil {
		return manifestdomain.ManifestRenderedResource{}, fmt.Errorf("decode rendered object %s/%s: %w", item.Kind, item.Name, err)
	}
	decoded.Object = object
	return decoded, nil
}

func buildManifest(req *manifestdomain.CreateManifestRequest, image *imagedomain.Image, applicationName string, appConfig *appconfigdownstream.AppConfig, workload *appconfigdownstream.WorkloadConfig, services []appservicedownstream.Service, routes []appservicedownstream.Route, namespace string, imageRegistry imagedomain.ImageRegistryConfig) (*manifestdomain.Manifest, error) {
	servicesSnapshot := make([]manifestdomain.ManifestService, 0, len(services))
	servicePorts := make(map[string]map[int]struct{}, len(services))
	for _, item := range services {
		ports := make([]manifestdomain.ManifestServicePort, 0, len(item.Ports))
		knownPorts := make(map[int]struct{}, len(item.Ports))
		for _, port := range item.Ports {
			ports = append(ports, manifestdomain.ManifestServicePort{Name: port.Name, ServicePort: port.ServicePort, TargetPort: port.TargetPort, Protocol: port.Protocol})
			knownPorts[port.ServicePort] = struct{}{}
		}
		servicesSnapshot = append(servicesSnapshot, manifestdomain.ManifestService{ID: item.ID, Name: item.Name, Ports: ports})
		servicePorts[item.Name] = knownPorts
	}
	routesSnapshot := make([]manifestdomain.ManifestRoute, 0, len(routes))
	for _, item := range routes {
		if _, ok := servicePorts[item.ServiceName]; !ok {
			return nil, fmt.Errorf("%w: service %s", ErrManifestRouteTargetInvalid, item.ServiceName)
		}
		if _, ok := servicePorts[item.ServiceName][item.ServicePort]; !ok {
			return nil, fmt.Errorf("%w: service %s port %d", ErrManifestRouteTargetInvalid, item.ServiceName, item.ServicePort)
		}
		routesSnapshot = append(routesSnapshot, manifestdomain.ManifestRoute{
			ID: item.ID, Name: item.Name, Host: item.Host, Path: item.Path, ServiceName: item.ServiceName, ServicePort: item.ServicePort,
		})
	}
	configData := appConfig.RenderedConfigMap
	if len(configData) == 0 && len(appConfig.Files) > 0 {
		configData = make(map[string]string, len(appConfig.Files))
	}
	files := make([]manifestdomain.ManifestFile, 0, len(appConfig.Files))
	for _, file := range appConfig.Files {
		files = append(files, manifestdomain.ManifestFile{Name: file.Name, Content: file.Content})
		if len(configData) > 0 {
			configData[file.Name] = file.Content
		}
	}
	imageRepository, err := resolveManifestImageRepository(image, imageRegistry)
	if err != nil {
		return nil, err
	}
	imageRef, annotations, err := resolveWorkloadImageRef(imageRepository, image.Tag, image.Digest)
	if err != nil {
		return nil, err
	}

	appConfigSnapshot := manifestdomain.ManifestAppConfig{
		ID:           appConfig.ID,
		Name:         appConfig.Name,
		MountPath:    appConfig.MountPath,
		Files:        files,
		Data:         configData,
		SourcePath:   appConfig.SourcePath,
		SourceCommit: appConfig.SourceCommit,
	}
	workloadSnapshot := manifestdomain.ManifestWorkloadConfig{
		ID:           workload.ID,
		Name:         workload.Name,
		Replicas:     workload.Replicas,
		Resources:    workload.Resources,
		Probes:       workload.Probes,
		Env:          toModelEnvVars(workload.Env),
		WorkloadType: workload.WorkloadType,
		Strategy:     workload.Strategy,
	}

	configMapName := configMapResourceName(appConfigSnapshot, applicationName)
	renderedObjects, err := renderManifestObjects(namespace, applicationName, req.ApplicationID.String(), req.EnvironmentID, configMapName, appConfigSnapshot, workloadSnapshot, servicesSnapshot, routesSnapshot, imageRef, annotations)
	if err != nil {
		return nil, err
	}
	return &manifestdomain.Manifest{
		ApplicationID:          req.ApplicationID,
		EnvironmentID:          req.EnvironmentID,
		ImageID:                req.ImageID,
		ImageRef:               imageRef,
		ServicesSnapshot:       servicesSnapshot,
		RoutesSnapshot:         routesSnapshot,
		AppConfigSnapshot:      appConfigSnapshot,
		WorkloadConfigSnapshot: workloadSnapshot,
		RenderedObjects:        renderedObjects,
		RenderedYAML:           joinRenderedYAML(renderedObjects),
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

func configMapResourceName(appConfig manifestdomain.ManifestAppConfig, applicationName string) string {
	base := strings.TrimSpace(appConfig.Name)
	if base == "" {
		base = strings.TrimSpace(applicationName)
	}
	base = sanitizeKubernetesName(base)
	if base == "" {
		base = "config"
	}
	suffix := uuid.NewString()
	if len(suffix) > 8 {
		suffix = suffix[:8]
	}
	name := sanitizeKubernetesName(base + "-" + suffix)
	if len(name) > 63 {
		name = strings.TrimRight(name[:63], "-")
	}
	if name == "" {
		return "config-" + suffix
	}
	return name
}

func sanitizeKubernetesName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var out []rune
	lastDash := false
	for _, r := range value {
		valid := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if valid {
			out = append(out, r)
			lastDash = false
			continue
		}
		if !lastDash {
			out = append(out, '-')
			lastDash = true
		}
	}
	return strings.Trim(string(out), "-")
}
