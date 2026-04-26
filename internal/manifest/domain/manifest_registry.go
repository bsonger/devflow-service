package domain

import (
	"strings"

	"github.com/bsonger/devflow-service/internal/image/domain"
)

type ManifestRegistryConfig struct {
	Registry   string
	Namespace  string
	Repository string
	Username   string
	Password   string
	PlainHTTP  bool
}

func (c ManifestRegistryConfig) RepositoryPrefix() string {
	registry := domain.ImageRegistryConfig{
		Registry:  c.Registry,
		Namespace: c.Namespace,
	}.Repository()
	repository := normalizeImageSegment(c.Repository)
	if repository == "" {
		repository = "manifests"
	}
	if registry == "" {
		return repository
	}
	return registry + "/" + repository
}

func (c ManifestRegistryConfig) RepositoryFor(applicationName, environmentID string) string {
	prefix := c.RepositoryPrefix()
	application := normalizeImageSegment(applicationName)
	if application == "" {
		application = "application"
	}
	return prefix + "/" + application
}

func normalizeImageSegment(value string) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	trimmed = strings.ReplaceAll(trimmed, "/", "-")
	trimmed = strings.ReplaceAll(trimmed, "_", "-")
	trimmed = strings.ReplaceAll(trimmed, " ", "-")
	return strings.Trim(trimmed, "-")
}
