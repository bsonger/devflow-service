package support

import (
	"context"
	"errors"
	"strings"
	"testing"

	applicationdownstream "github.com/bsonger/devflow-service/internal/application/transport/downstream"
	clusterdownstream "github.com/bsonger/devflow-service/internal/cluster/transport/downstream"
	environmentdownstream "github.com/bsonger/devflow-service/internal/environment/transport/downstream"
	"github.com/bsonger/devflow-service/internal/platform/k8s"
	projectdownstream "github.com/bsonger/devflow-service/internal/project/transport/downstream"
	"github.com/bsonger/devflow-service/internal/release/transport/downstream"
)

type fakeBindingReader struct {
	binding *downstream.ApplicationEnvironment
	err     error
}

func (f fakeBindingReader) GetApplicationEnvironment(context.Context, string, string) (*downstream.ApplicationEnvironment, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.binding, nil
}

type fakeOwnerReader struct {
	application     *applicationdownstream.Application
	applicationErr  error
	project         *projectdownstream.Project
	projectErr      error
	environment     *environmentdownstream.Environment
	environmentErr  error
	cluster         *clusterdownstream.Cluster
	clusterErr      error
}

func (f fakeOwnerReader) GetApplication(ctx context.Context, id string) (*applicationdownstream.Application, error) {
	if f.applicationErr != nil {
		return nil, f.applicationErr
	}
	return f.application, nil
}

func (f fakeOwnerReader) GetProject(ctx context.Context, id string) (*projectdownstream.Project, error) {
	if f.projectErr != nil {
		return nil, f.projectErr
	}
	return f.project, nil
}

func (f fakeOwnerReader) GetEnvironment(ctx context.Context, id string) (*environmentdownstream.Environment, error) {
	if f.environmentErr != nil {
		return nil, f.environmentErr
	}
	return f.environment, nil
}

func (f fakeOwnerReader) GetCluster(ctx context.Context, id string) (*clusterdownstream.Cluster, error) {
	if f.clusterErr != nil {
		return nil, f.clusterErr
	}
	return f.cluster, nil
}

func TestResolveDeployTargetRequiresApplicationID(t *testing.T) {
	_, err := ResolveDeployTarget(context.Background(), "", "staging")
	if !errors.Is(err, ErrDeployTargetApplicationIDRequired) {
		t.Fatalf("error = %v, want %v", err, ErrDeployTargetApplicationIDRequired)
	}
}

func TestResolveDeployTargetRequiresEnvironmentID(t *testing.T) {
	_, err := ResolveDeployTarget(context.Background(), "app-1", "")
	if !errors.Is(err, ErrDeployTargetEnvironmentIDRequired) {
		t.Fatalf("error = %v, want %v", err, ErrDeployTargetEnvironmentIDRequired)
	}
}

func TestResolveDeployTargetMissingBinding(t *testing.T) {
	resolver := &deployTargetResolver{bindingReader: &fakeBindingReader{err: errors.New("downstream request failed: 404")}}
	_, err := resolver.Resolve(context.Background(), "app-1", "staging")
	if !errors.Is(err, ErrDeployTargetBindingMissing) {
		t.Fatalf("error = %v, want %v", err, ErrDeployTargetBindingMissing)
	}
}

func TestResolveDeployTargetMalformedBinding(t *testing.T) {
	resolver := &deployTargetResolver{bindingReader: &fakeBindingReader{binding: &downstream.ApplicationEnvironment{}}}
	_, err := resolver.Resolve(context.Background(), "app-1", "staging")
	if !errors.Is(err, ErrDeployTargetBindingMalformed) {
		t.Fatalf("error = %v, want %v", err, ErrDeployTargetBindingMalformed)
	}
}

func TestResolveDeployTargetMissingApplicationMetadata(t *testing.T) {
	resolver := &deployTargetResolver{
		bindingReader: &fakeBindingReader{binding: &downstream.ApplicationEnvironment{ID: "ae-1", ApplicationID: "app-1"}},
		ownerReader:   &fakeOwnerReader{applicationErr: errors.New("downstream request failed: 404")},
	}
	_, err := resolver.Resolve(context.Background(), "app-1", "staging")
	if !errors.Is(err, ErrDeployTargetApplicationMetadataMissing) {
		t.Fatalf("error = %v, want %v", err, ErrDeployTargetApplicationMetadataMissing)
	}
}

func TestResolveDeployTargetMissingProjectMetadata(t *testing.T) {
	resolver := &deployTargetResolver{
		bindingReader: &fakeBindingReader{binding: &downstream.ApplicationEnvironment{ID: "ae-1", ApplicationID: "app-1"}},
		ownerReader:   &fakeOwnerReader{application: &applicationdownstream.Application{ID: "app-1", Name: "portal", ProjectID: "project-1"}, projectErr: errors.New("downstream request failed: 404")},
	}
	_, err := resolver.Resolve(context.Background(), "app-1", "staging")
	if !errors.Is(err, ErrDeployTargetProjectMetadataMissing) {
		t.Fatalf("error = %v, want %v", err, ErrDeployTargetProjectMetadataMissing)
	}
}

func TestResolveDeployTargetMissingEnvironmentMetadata(t *testing.T) {
	resolver := &deployTargetResolver{
		bindingReader: &fakeBindingReader{binding: &downstream.ApplicationEnvironment{ID: "ae-1", ApplicationID: "app-1"}},
		ownerReader: &fakeOwnerReader{
			application: &applicationdownstream.Application{ID: "app-1", Name: "portal", ProjectID: "project-1"},
			project:     &projectdownstream.Project{ID: "project-1", Name: "checkout"},
			environmentErr: errors.New("downstream request failed: 404"),
		},
	}
	_, err := resolver.Resolve(context.Background(), "app-1", "staging")
	if !errors.Is(err, ErrDeployTargetEnvironmentMetadataMissing) {
		t.Fatalf("error = %v, want %v", err, ErrDeployTargetEnvironmentMetadataMissing)
	}
}

func TestResolveDeployTargetMissingClusterMetadata(t *testing.T) {
	resolver := &deployTargetResolver{
		bindingReader: &fakeBindingReader{binding: &downstream.ApplicationEnvironment{ID: "ae-1", ApplicationID: "app-1"}},
		ownerReader: &fakeOwnerReader{
			application: &applicationdownstream.Application{ID: "app-1", Name: "portal", ProjectID: "project-1"},
			project:     &projectdownstream.Project{ID: "project-1", Name: "checkout"},
			environment: &environmentdownstream.Environment{ID: "staging", Name: "staging", ClusterID: "cluster-1"},
			clusterErr:  errors.New("downstream request failed: 404"),
		},
	}
	_, err := resolver.Resolve(context.Background(), "app-1", "staging")
	if !errors.Is(err, ErrDeployTargetClusterMetadataMissing) {
		t.Fatalf("error = %v, want %v", err, ErrDeployTargetClusterMetadataMissing)
	}
}

func TestResolveDeployTargetClusterNotReady(t *testing.T) {
	resolver := &deployTargetResolver{
		bindingReader: &fakeBindingReader{binding: &downstream.ApplicationEnvironment{ID: "ae-1", ApplicationID: "app-1"}},
		ownerReader: &fakeOwnerReader{
			application: &applicationdownstream.Application{ID: "app-1", Name: "portal", ProjectID: "project-1"},
			project:     &projectdownstream.Project{ID: "project-1", Name: "checkout"},
			environment: &environmentdownstream.Environment{ID: "staging", Name: "staging", ClusterID: "cluster-1"},
			cluster:     &clusterdownstream.Cluster{ID: "cluster-1", Name: "cluster-1", OnboardingReady: false, OnboardingCheckedAt: "2024-01-01T00:00:00Z"},
		},
	}
	_, err := resolver.Resolve(context.Background(), "app-1", "staging")
	if !errors.Is(err, ErrDeployTargetClusterNotReady) {
		t.Fatalf("error = %v, want %v", err, ErrDeployTargetClusterNotReady)
	}
}

func TestResolveDeployTargetSuccess(t *testing.T) {
	resolver := &deployTargetResolver{
		bindingReader: &fakeBindingReader{binding: &downstream.ApplicationEnvironment{ID: "ae-1", ApplicationID: "app-1"}},
		ownerReader: &fakeOwnerReader{
			application: &applicationdownstream.Application{ID: "app-1", Name: "portal", ProjectID: "project-1"},
			project:     &projectdownstream.Project{ID: "project-1", Name: "checkout"},
			environment: &environmentdownstream.Environment{ID: "staging", Name: "staging", ClusterID: "cluster-1"},
			cluster:     &clusterdownstream.Cluster{ID: "cluster-1", Name: "cluster-1", OnboardingReady: true, Server: "https://k8s.example.com"},
		},
	}
	target, err := resolver.Resolve(context.Background(), "app-1", "staging")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if target.Namespace != "checkout-staging" {
		t.Fatalf("namespace = %q, want checkout-staging", target.Namespace)
	}
	if target.DestinationServer != "https://k8s.example.com" {
		t.Fatalf("destination server = %q, want https://k8s.example.com", target.DestinationServer)
	}
}

func TestDeriveNamespaceProductionSpecialCase(t *testing.T) {
	namespace, err := k8s.DeriveNamespace("Payments_API", "production")
	if err != nil {
		t.Fatalf("DeriveNamespace error = %v", err)
	}
	if namespace != "payments-api" {
		t.Fatalf("namespace = %q, want payments-api", namespace)
	}
}

func TestDeriveNamespaceStagingWithDash(t *testing.T) {
	namespace, err := k8s.DeriveNamespace("Checkout", "Staging")
	if err != nil {
		t.Fatalf("DeriveNamespace error = %v", err)
	}
	if namespace != "checkout-staging" {
		t.Fatalf("namespace = %q, want checkout-staging", namespace)
	}
}

func TestDeriveNamespaceHandlesSpecialCharacters(t *testing.T) {
	namespace, err := k8s.DeriveNamespace("My App_2.0", "dev-env")
	if err != nil {
		t.Fatalf("DeriveNamespace error = %v", err)
	}
	if namespace != "my-app-2-0-dev-env" {
		t.Fatalf("namespace = %q, want my-app-2-0-dev-env", namespace)
	}
}

func TestDeriveNamespaceRequiresNames(t *testing.T) {
	if _, err := k8s.DeriveNamespace("", "staging"); !errors.Is(err, k8s.ErrNamespaceProjectNameRequired) {
		t.Fatalf("error = %v, want %v", err, k8s.ErrNamespaceProjectNameRequired)
	}
	if _, err := k8s.DeriveNamespace("checkout", ""); !errors.Is(err, k8s.ErrNamespaceEnvironmentNameRequired) {
		t.Fatalf("error = %v, want %v", err, k8s.ErrNamespaceEnvironmentNameRequired)
	}
}

func TestDeriveNamespaceTooLong(t *testing.T) {
	longName := strings.Repeat("a", 40)
	_, err := k8s.DeriveNamespace(longName, longName)
	if !errors.Is(err, k8s.ErrNamespaceDerivedValueTooLong) {
		t.Fatalf("error = %v, want %v", err, k8s.ErrNamespaceDerivedValueTooLong)
	}
}
