package domain

import "github.com/bsonger/devflow-service/internal/platform/oci"

type ManifestRegistryConfig struct {
	Registry   string
	Namespace  string
	Repository string
	Username   string
	Password   string
	PlainHTTP  bool
}

func (c ManifestRegistryConfig) RepositoryPrefix() string {
	registry := oci.ImageRegistryConfig{
		Registry:  c.Registry,
		Namespace: c.Namespace,
	}.Repository()
	repository := oci.NormalizeImageSegment(c.Repository)
	if repository == "" {
		repository = "manifests"
	}
	if registry == "" {
		return repository
	}
	return registry + "/" + repository
}

func (c ManifestRegistryConfig) RepositoryFor(applicationName, environmentId string) string {
	prefix := c.RepositoryPrefix()
	application := oci.NormalizeImageSegment(applicationName)
	if application == "" {
		application = "application"
	}
	return prefix + "/" + application
}
