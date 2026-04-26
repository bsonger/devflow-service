package support

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	applicationdownstream "github.com/bsonger/devflow-service/internal/application/transport/downstream"
	clusterdownstream "github.com/bsonger/devflow-service/internal/cluster/transport/downstream"
	environmentdownstream "github.com/bsonger/devflow-service/internal/environment/transport/downstream"
	"github.com/bsonger/devflow-service/internal/platform/k8s"
	projectdownstream "github.com/bsonger/devflow-service/internal/project/transport/downstream"
	"github.com/bsonger/devflow-service/internal/release/transport/downstream"
	"github.com/bsonger/devflow-service/internal/shared/downstreamhttp"
)

var (
	ErrDeployTargetApplicationIDRequired        = errors.New("application id is required")
	ErrDeployTargetEnvironmentIDRequired        = errors.New("environment id is required")
	ErrDeployTargetBindingMissing               = errors.New("application environment binding is missing")
	ErrDeployTargetBindingLookupFailed          = errors.New("application environment binding lookup failed")
	ErrDeployTargetBindingMalformed             = errors.New("application environment binding payload is malformed")
	ErrDeployTargetApplicationLookupFailed      = errors.New("application metadata lookup failed")
	ErrDeployTargetApplicationMetadataMissing   = errors.New("application metadata is missing")
	ErrDeployTargetApplicationMetadataMalformed = errors.New("application metadata is malformed")
	ErrDeployTargetProjectLookupFailed          = errors.New("project metadata lookup failed")
	ErrDeployTargetProjectMetadataMissing       = errors.New("project metadata is missing")
	ErrDeployTargetProjectMetadataMalformed     = errors.New("project metadata is malformed")
	ErrDeployTargetEnvironmentLookupFailed      = errors.New("environment metadata lookup failed")
	ErrDeployTargetEnvironmentMetadataMissing   = errors.New("environment metadata is missing")
	ErrDeployTargetEnvironmentMetadataMalformed = errors.New("environment metadata is malformed")
	ErrDeployTargetClusterLookupFailed          = errors.New("cluster metadata lookup failed")
	ErrDeployTargetClusterMetadataMissing       = errors.New("cluster metadata is missing")
	ErrDeployTargetClusterMetadataMalformed     = errors.New("cluster metadata is malformed")
	ErrDeployTargetClusterNotReady              = errors.New("cluster onboarding is not ready")
	ErrDeployTargetClusterReadinessMalformed    = errors.New("cluster readiness metadata is malformed")
	ErrDeployTargetNamespaceInvalid             = errors.New("derived namespace is invalid")
	ErrDeployTargetClusterServerInvalid         = errors.New("cluster server is invalid")
)

type DeployTarget struct {
	Namespace         string
	DestinationServer string
	ProjectName       string
	EnvironmentName   string
	ClusterID         string
}

type deployTargetBindingReader interface {
	GetApplicationEnvironment(ctx context.Context, applicationId, environmentId string) (*downstream.ApplicationEnvironment, error)
}

type deployTargetApplicationReader interface {
	GetApplication(ctx context.Context, id string) (*applicationdownstream.Application, error)
}
type deployTargetProjectReader interface {
	GetProject(ctx context.Context, id string) (*projectdownstream.Project, error)
}
type deployTargetEnvironmentReader interface {
	GetEnvironment(ctx context.Context, id string) (*environmentdownstream.Environment, error)
}
type deployTargetClusterReader interface {
	GetCluster(ctx context.Context, id string) (*clusterdownstream.Cluster, error)
}
type deployTargetOwnerReader interface {
	deployTargetApplicationReader
	deployTargetProjectReader
	deployTargetEnvironmentReader
	deployTargetClusterReader
}

type deployTargetOwnerReaders struct {
	applicationReader deployTargetApplicationReader
	projectReader     deployTargetProjectReader
	environmentReader deployTargetEnvironmentReader
	clusterReader     deployTargetClusterReader
}

func (r deployTargetOwnerReaders) GetApplication(ctx context.Context, id string) (*applicationdownstream.Application, error) {
	return r.applicationReader.GetApplication(ctx, id)
}

func (r deployTargetOwnerReaders) GetProject(ctx context.Context, id string) (*projectdownstream.Project, error) {
	return r.projectReader.GetProject(ctx, id)
}

func (r deployTargetOwnerReaders) GetEnvironment(ctx context.Context, id string) (*environmentdownstream.Environment, error) {
	return r.environmentReader.GetEnvironment(ctx, id)
}

func (r deployTargetOwnerReaders) GetCluster(ctx context.Context, id string) (*clusterdownstream.Cluster, error) {
	return r.clusterReader.GetCluster(ctx, id)
}

type deployTargetResolver struct {
	bindingReader deployTargetBindingReader
	ownerReader   deployTargetOwnerReader
}

func newDeployTargetResolver(cfg RuntimeConfig) *deployTargetResolver {
	return &deployTargetResolver{
		bindingReader: downstream.NewOrchestratorManifestClient(strings.TrimSpace(cfg.Downstream.PlatformOrchestratorBaseURL)),
		ownerReader: deployTargetOwnerReaders{
			applicationReader: applicationdownstream.New(strings.TrimSpace(cfg.Downstream.AppServiceBaseURL)),
			projectReader:     projectdownstream.New(strings.TrimSpace(cfg.Downstream.AppServiceBaseURL)),
			environmentReader: environmentdownstream.New(strings.TrimSpace(cfg.Downstream.AppServiceBaseURL)),
			clusterReader:     clusterdownstream.New(strings.TrimSpace(cfg.Downstream.AppServiceBaseURL)),
		},
	}
}

func ResolveDeployTarget(ctx context.Context, applicationId, environmentId string) (*DeployTarget, error) {
	resolver := newDeployTargetResolver(CurrentRuntimeConfig())
	return resolver.Resolve(ctx, applicationId, environmentId)
}

func resolveDeployTarget(ctx context.Context, applicationId, environmentId string) (*DeployTarget, error) {
	return ResolveDeployTarget(ctx, applicationId, environmentId)
}

func (r *deployTargetResolver) Resolve(ctx context.Context, applicationId, environmentId string) (*DeployTarget, error) {
	applicationId = strings.TrimSpace(applicationId)
	environmentId = strings.TrimSpace(environmentId)
	if applicationId == "" {
		return nil, ErrDeployTargetApplicationIDRequired
	}
	if environmentId == "" {
		return nil, ErrDeployTargetEnvironmentIDRequired
	}

	binding, err := r.bindingReader.GetApplicationEnvironment(ctx, applicationId, environmentId)
	if err != nil {
		if downstreamhttp.IsStatus(err, http.StatusNotFound) {
			return nil, fmt.Errorf("%w: application_id=%s environment_id=%s", ErrDeployTargetBindingMissing, applicationId, environmentId)
		}
		return nil, fmt.Errorf("%w: %v", ErrDeployTargetBindingLookupFailed, err)
	}
	if binding == nil || strings.TrimSpace(binding.ID) == "" || strings.TrimSpace(binding.ApplicationID) == "" {
		return nil, fmt.Errorf("%w: missing id or application_id", ErrDeployTargetBindingMalformed)
	}
	if strings.TrimSpace(binding.ApplicationID) != applicationId {
		return nil, fmt.Errorf("%w: binding application_id=%s expected=%s", ErrDeployTargetBindingMalformed, binding.ApplicationID, applicationId)
	}

	application, err := r.ownerReader.GetApplication(ctx, applicationId)
	if err != nil {
		if downstreamhttp.IsStatus(err, http.StatusNotFound) {
			return nil, fmt.Errorf("%w: application_id=%s", ErrDeployTargetApplicationMetadataMissing, applicationId)
		}
		return nil, fmt.Errorf("%w: %v", ErrDeployTargetApplicationLookupFailed, err)
	}
	if application == nil || strings.TrimSpace(application.ID) == "" || strings.TrimSpace(application.ProjectID) == "" {
		return nil, fmt.Errorf("%w: missing id or project_id", ErrDeployTargetApplicationMetadataMalformed)
	}

	project, err := r.ownerReader.GetProject(ctx, strings.TrimSpace(application.ProjectID))
	if err != nil {
		if downstreamhttp.IsStatus(err, http.StatusNotFound) {
			return nil, fmt.Errorf("%w: project_id=%s", ErrDeployTargetProjectMetadataMissing, application.ProjectID)
		}
		return nil, fmt.Errorf("%w: %v", ErrDeployTargetProjectLookupFailed, err)
	}
	if project == nil || strings.TrimSpace(project.ID) == "" || strings.TrimSpace(project.Name) == "" {
		return nil, fmt.Errorf("%w: missing id or name", ErrDeployTargetProjectMetadataMalformed)
	}

	environment, err := r.ownerReader.GetEnvironment(ctx, environmentId)
	if err != nil {
		if downstreamhttp.IsStatus(err, http.StatusNotFound) {
			return nil, fmt.Errorf("%w: environment_id=%s", ErrDeployTargetEnvironmentMetadataMissing, environmentId)
		}
		return nil, fmt.Errorf("%w: %v", ErrDeployTargetEnvironmentLookupFailed, err)
	}
	if environment == nil || strings.TrimSpace(environment.ID) == "" || strings.TrimSpace(environment.Name) == "" || strings.TrimSpace(environment.ClusterID) == "" {
		return nil, fmt.Errorf("%w: missing id/name/cluster_id", ErrDeployTargetEnvironmentMetadataMalformed)
	}

	cluster, err := r.ownerReader.GetCluster(ctx, strings.TrimSpace(environment.ClusterID))
	if err != nil {
		if downstreamhttp.IsStatus(err, http.StatusNotFound) {
			return nil, fmt.Errorf("%w: cluster_id=%s", ErrDeployTargetClusterMetadataMissing, environment.ClusterID)
		}
		return nil, fmt.Errorf("%w: %v", ErrDeployTargetClusterLookupFailed, err)
	}
	if cluster == nil || strings.TrimSpace(cluster.ID) == "" {
		return nil, fmt.Errorf("%w: missing id", ErrDeployTargetClusterMetadataMalformed)
	}

	if !cluster.OnboardingReady {
		if strings.TrimSpace(cluster.OnboardingCheckedAt) == "" {
			return nil, fmt.Errorf("%w: cluster_id=%s onboarding not yet checked", ErrDeployTargetClusterReadinessMalformed, cluster.ID)
		}
		reason := strings.TrimSpace(cluster.OnboardingError)
		if reason == "" {
			reason = "unknown onboarding failure"
		}
		return nil, fmt.Errorf("%w: cluster_id=%s reason=%s", ErrDeployTargetClusterNotReady, cluster.ID, reason)
	}

	namespace, err := k8s.DeriveNamespace(project.Name, environment.Name)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDeployTargetNamespaceInvalid, err)
	}
	server, err := normalizeClusterServer(cluster.Server)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDeployTargetClusterServerInvalid, err)
	}

	return &DeployTarget{
		Namespace:         namespace,
		DestinationServer: server,
		ProjectName:       strings.TrimSpace(project.Name),
		EnvironmentName:   strings.TrimSpace(environment.Name),
		ClusterID:         strings.TrimSpace(cluster.ID),
	}, nil
}

func normalizeClusterServer(server string) (string, error) {
	trimmed := strings.TrimSpace(server)
	if trimmed == "" {
		return "", errors.New("cluster server is empty")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("cluster server parse failed: %w", err)
	}
	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("cluster server scheme must be http or https")
	}
	host := strings.TrimSpace(parsed.Host)
	if host == "" {
		return "", fmt.Errorf("cluster server host is empty")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("cluster server must not include query or fragment")
	}
	if path := strings.TrimSpace(parsed.EscapedPath()); path != "" && path != "/" {
		return "", fmt.Errorf("cluster server path is not allowed")
	}
	return scheme + "://" + host, nil
}
