package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	appconfigdownstream "github.com/bsonger/devflow-service/internal/appconfig/transport/downstream"
	imageservice "github.com/bsonger/devflow-service/internal/image/service"
	appservicedownstream "github.com/bsonger/devflow-service/internal/appservice/transport/downstream"
	imagedomain "github.com/bsonger/devflow-service/internal/image/domain"
	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	store "github.com/bsonger/devflow-service/internal/platform/db"
	"github.com/google/uuid"
	"sigs.k8s.io/yaml"
)

var (
	ErrManifestImageApplicationMismatch = errors.New("image does not belong to application")
	ErrManifestAppConfigMissing         = errors.New("effective app config is missing")
	ErrManifestWorkloadConfigMissing    = errors.New("effective workload config is missing")
	ErrManifestRouteTargetInvalid       = errors.New("route points to missing service or port")
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
		Get(context.Context, uuid.UUID) (*applicationProjection, error)
	}
}

func NewManifestService() *manifestService {
	return &manifestService{
		images: imageservice.ImageService,
		apps:   ApplicationService,
	}
}

func (s *manifestService) CreateManifest(ctx context.Context, req *manifestdomain.CreateManifestRequest) (*manifestdomain.Manifest, error) {
	runtimeCfg := CurrentRuntimeConfig()
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
	target, err := resolveDeployTarget(ctx, req.ApplicationID.String(), req.EnvironmentID)
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
		return nil, err
	}
	if err := s.insert(ctx, manifest); err != nil {
		return nil, err
	}
	return manifest, nil
}

func resolveManifestImageRepository(image *imagedomain.Image, cfg imagedomain.ImageRegistryConfig) (string, error) {
	if image == nil {
		return "", fmt.Errorf("image is required")
	}
	name := strings.Trim(strings.TrimSpace(image.Name), "/")
	if name == "" {
		return "", fmt.Errorf("image name is required")
	}
	repoAddress := strings.TrimSpace(image.RepoAddress)
	if looksLikeContainerRepository(repoAddress) {
		return strings.TrimRight(repoAddress, "/") + "/" + name, nil
	}
	repository := strings.TrimRight(cfg.Repository(), "/")
	if repository == "" {
		return "", fmt.Errorf("image repository is not deployable")
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
	query := `
		select id, application_id, environment_id, image_id, image_ref,
			artifact_repository, artifact_tag, artifact_ref, artifact_digest, artifact_media_type, artifact_pushed_at,
			services_snapshot, routes_snapshot, app_config_snapshot, workload_config_snapshot,
			rendered_objects, rendered_yaml, status, created_at, updated_at, deleted_at
		from manifests
	`
	clauses := make([]string, 0, 4)
	args := make([]any, 0, 4)
	if !filter.IncludeDeleted {
		clauses = append(clauses, "deleted_at is null")
	}
	if filter.ApplicationID != nil {
		args = append(args, *filter.ApplicationID)
		clauses = append(clauses, placeholderClause("application_id", len(args)))
	}
	if filter.EnvironmentID != nil {
		args = append(args, *filter.EnvironmentID)
		clauses = append(clauses, placeholderClause("environment_id", len(args)))
	}
	if filter.ImageID != nil {
		args = append(args, *filter.ImageID)
		clauses = append(clauses, placeholderClause("image_id", len(args)))
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
	out := make([]manifestdomain.Manifest, 0)
	for rows.Next() {
		item, err := scanManifest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

func (s *manifestService) GetResources(ctx context.Context, id uuid.UUID) (*manifestdomain.ManifestResourcesView, error) {
	item, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return buildManifestResourcesView(item)
}

func (s *manifestService) Get(ctx context.Context, id uuid.UUID) (*manifestdomain.Manifest, error) {
	return scanManifest(store.DB().QueryRowContext(ctx, `
		select id, application_id, environment_id, image_id, image_ref,
			artifact_repository, artifact_tag, artifact_ref, artifact_digest, artifact_media_type, artifact_pushed_at,
			services_snapshot, routes_snapshot, app_config_snapshot, workload_config_snapshot,
			rendered_objects, rendered_yaml, status, created_at, updated_at, deleted_at
		from manifests
		where id = $1 and deleted_at is null
	`, id))
}

func (s *manifestService) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := store.DB().ExecContext(ctx, `
		update manifests
		set deleted_at = $2, updated_at = $2
		where id = $1 and deleted_at is null
	`, id, time.Now())
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
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

func (s *manifestService) insert(ctx context.Context, m *manifestdomain.Manifest) error {
	servicesJSON, err := marshalJSON(m.ServicesSnapshot, "[]")
	if err != nil {
		return err
	}
	routesJSON, err := marshalJSON(m.RoutesSnapshot, "[]")
	if err != nil {
		return err
	}
	appConfigJSON, err := marshalJSON(m.AppConfigSnapshot, "{}")
	if err != nil {
		return err
	}
	workloadJSON, err := marshalJSON(m.WorkloadConfigSnapshot, "{}")
	if err != nil {
		return err
	}
	renderedJSON, err := marshalJSON(m.RenderedObjects, "[]")
	if err != nil {
		return err
	}
	_, err = store.DB().ExecContext(ctx, `
		insert into manifests (
			id, application_id, environment_id, image_id, image_ref,
			artifact_repository, artifact_tag, artifact_ref, artifact_digest, artifact_media_type, artifact_pushed_at,
			services_snapshot, routes_snapshot, app_config_snapshot, workload_config_snapshot,
			rendered_objects, rendered_yaml, status, created_at, updated_at, deleted_at
		) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)
	`, m.ID, m.ApplicationID, m.EnvironmentID, m.ImageID, m.ImageRef,
		m.ArtifactRepository, m.ArtifactTag, m.ArtifactRef, m.ArtifactDigest, m.ArtifactMediaType, nullableTimePtr(m.ArtifactPushedAt),
		servicesJSON, routesJSON, appConfigJSON, workloadJSON, renderedJSON, m.RenderedYAML,
		m.Status, m.CreatedAt, m.UpdatedAt, m.DeletedAt)
	return err
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
