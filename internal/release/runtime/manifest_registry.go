package runtime

import (
	"fmt"
	"strings"

	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	model "github.com/bsonger/devflow-service/internal/release/domain"
)

// ManifestRegistryConfigFromConfig resolves the historical `manifest_registry` config block
// into the OCI publication target used by release deployment bundle publication.
//
// The external config key keeps its legacy name for compatibility, but this runtime path
// now feeds deploy-side bundle publication rather than build-side Manifest persistence.
func ManifestRegistryConfigFromConfig(source *model.ManifestRegistryRuntimeConfig, image *model.ImageRegistryRuntimeConfig) (manifestdomain.ManifestRegistryConfig, bool, error) {
	bundleRegistryCfg := manifestdomain.ManifestRegistryConfig{
		Registry:   firstNonEmpty(stringValue(source, func(v *model.ManifestRegistryRuntimeConfig) string { return v.Registry }), stringValue(image, func(v *model.ImageRegistryRuntimeConfig) string { return v.Registry })),
		Namespace:  firstNonEmpty(stringValue(source, func(v *model.ManifestRegistryRuntimeConfig) string { return v.Namespace }), stringValue(image, func(v *model.ImageRegistryRuntimeConfig) string { return v.Namespace })),
		Repository: stringValue(source, func(v *model.ManifestRegistryRuntimeConfig) string { return v.Repository }),
		Username:   firstNonEmpty(stringValue(source, func(v *model.ManifestRegistryRuntimeConfig) string { return v.Username }), stringValue(image, func(v *model.ImageRegistryRuntimeConfig) string { return v.Username })),
		Password:   firstNonEmpty(stringValue(source, func(v *model.ManifestRegistryRuntimeConfig) string { return v.Password }), stringValue(image, func(v *model.ImageRegistryRuntimeConfig) string { return v.Password })),
		PlainHTTP:  boolValue(source, func(v *model.ManifestRegistryRuntimeConfig) bool { return v.PlainHTTP }),
	}
	if bundleRegistryCfg.Repository == "" {
		bundleRegistryCfg.Repository = "manifests"
	}
	if bundleRegistryCfg.Registry == "" && bundleRegistryCfg.Namespace == "" {
		return manifestdomain.ManifestRegistryConfig{}, false, nil
	}
	if bundleRegistryCfg.Registry == "" {
		return manifestdomain.ManifestRegistryConfig{}, false, fmt.Errorf("manifest registry config missing registry")
	}
	if bundleRegistryCfg.Namespace == "" {
		return manifestdomain.ManifestRegistryConfig{}, false, fmt.Errorf("manifest registry config missing namespace")
	}
	return bundleRegistryCfg, true, nil
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
