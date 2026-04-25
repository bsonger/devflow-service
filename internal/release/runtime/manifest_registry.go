package runtime

import (
	"fmt"
	"strings"

	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	model "github.com/bsonger/devflow-service/internal/release/domain"
)

func ManifestRegistryConfigFromConfig(source *model.ManifestRegistryRuntimeConfig, image *model.ImageRegistryRuntimeConfig) (manifestdomain.ManifestRegistryConfig, bool, error) {
	cfg := manifestdomain.ManifestRegistryConfig{
		Registry:   firstNonEmpty(stringValue(source, func(v *model.ManifestRegistryRuntimeConfig) string { return v.Registry }), stringValue(image, func(v *model.ImageRegistryRuntimeConfig) string { return v.Registry })),
		Namespace:  firstNonEmpty(stringValue(source, func(v *model.ManifestRegistryRuntimeConfig) string { return v.Namespace }), stringValue(image, func(v *model.ImageRegistryRuntimeConfig) string { return v.Namespace })),
		Repository: stringValue(source, func(v *model.ManifestRegistryRuntimeConfig) string { return v.Repository }),
		Username:   firstNonEmpty(stringValue(source, func(v *model.ManifestRegistryRuntimeConfig) string { return v.Username }), stringValue(image, func(v *model.ImageRegistryRuntimeConfig) string { return v.Username })),
		Password:   firstNonEmpty(stringValue(source, func(v *model.ManifestRegistryRuntimeConfig) string { return v.Password }), stringValue(image, func(v *model.ImageRegistryRuntimeConfig) string { return v.Password })),
		PlainHTTP:  boolValue(source, func(v *model.ManifestRegistryRuntimeConfig) bool { return v.PlainHTTP }),
	}
	if cfg.Repository == "" {
		cfg.Repository = "manifests"
	}
	if cfg.Registry == "" && cfg.Namespace == "" {
		return manifestdomain.ManifestRegistryConfig{}, false, nil
	}
	if cfg.Registry == "" {
		return manifestdomain.ManifestRegistryConfig{}, false, fmt.Errorf("manifest registry config missing registry")
	}
	if cfg.Namespace == "" {
		return manifestdomain.ManifestRegistryConfig{}, false, fmt.Errorf("manifest registry config missing namespace")
	}
	return cfg, true, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func boolValue[T any](value *T, getter func(*T) bool) bool {
	if value == nil {
		return false
	}
	return getter(value)
}
