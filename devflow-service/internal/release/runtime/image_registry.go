package runtime

import (
	"fmt"
	"strings"

	imagedomain "github.com/bsonger/devflow-service/internal/image/domain"
	model "github.com/bsonger/devflow-service/internal/release/domain"
)

func ImageRegistryConfigFromConfig(source *model.ImageRegistryRuntimeConfig) (imagedomain.ImageRegistryConfig, error) {
	cfg := imagedomain.ImageRegistryConfig{
		Registry:  stringValue(source, func(v *model.ImageRegistryRuntimeConfig) string { return v.Registry }),
		Namespace: stringValue(source, func(v *model.ImageRegistryRuntimeConfig) string { return v.Namespace }),
		Username:  stringValue(source, func(v *model.ImageRegistryRuntimeConfig) string { return v.Username }),
		Password:  stringValue(source, func(v *model.ImageRegistryRuntimeConfig) string { return v.Password }),
	}
	if cfg.Registry == "" {
		return imagedomain.ImageRegistryConfig{}, fmt.Errorf("image_registry.registry is required")
	}
	if cfg.Namespace == "" {
		return imagedomain.ImageRegistryConfig{}, fmt.Errorf("image_registry.namespace is required")
	}
	return cfg, nil
}

func stringValue[T any](value *T, getter func(*T) string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(getter(value))
}
