package support

import (
	"strings"
	"sync"

	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	"github.com/bsonger/devflow-service/internal/platform/oci"
	model "github.com/bsonger/devflow-service/internal/release/domain"
)

const (
	defaultManifestBuildTektonNamespace       = "tekton-pipelines"
	defaultManifestBuildTektonPipeline        = "devflow-tekton-image-build-push-only"
	defaultManifestBuildTektonPVCGenerateName = "devflow-tekton-image-build-push-only"
)

type ManifestBuildTektonConfig struct {
	Namespace       string
	BuildPipeline   string
	PVCGenerateName string
}

func (cfg ManifestBuildTektonConfig) WithDefaults() ManifestBuildTektonConfig {
	resolved := ManifestBuildTektonConfig{
		Namespace:       strings.TrimSpace(cfg.Namespace),
		BuildPipeline:   strings.TrimSpace(cfg.BuildPipeline),
		PVCGenerateName: strings.TrimSpace(cfg.PVCGenerateName),
	}
	if resolved.Namespace == "" {
		resolved.Namespace = defaultManifestBuildTektonNamespace
	}
	if resolved.BuildPipeline == "" {
		resolved.BuildPipeline = defaultManifestBuildTektonPipeline
	}
	if resolved.PVCGenerateName == "" {
		resolved.PVCGenerateName = resolved.BuildPipeline
	}
	return resolved
}

func ManifestBuildTektonConfigFromModel(source *model.TektonConfig) ManifestBuildTektonConfig {
	cfg := ManifestBuildTektonConfig{}
	if source != nil {
		cfg.Namespace = source.Namespace
		cfg.BuildPipeline = source.BuildPipeline
		cfg.PVCGenerateName = source.PVCGenerateName
	}
	return cfg.WithDefaults()
}

type RuntimeConfig struct {
	ImageRegistry oci.ImageRegistryConfig
	// ManifestRegistry retains the historical field name because the external config and
	// downstream release/runtime code still speak in `manifest_registry` terms. In current
	// behavior, these values point at release deployment bundle publication, not build-side manifests.
	ManifestRegistry manifestdomain.ManifestRegistryConfig
	// ManifestRegistryEnabled gates whether release bundle publication should run at all.
	ManifestRegistryEnabled bool
	// ManifestPublisherMode selects how release bundle publication is performed while keeping
	// the historical `manifest_registry.mode` config key stable.
	ManifestPublisherMode string
	Tekton                ManifestBuildTektonConfig
	Downstream            model.DownstreamConfig
	ControlPlaneID        string
}

var (
	runtimeConfigMu sync.RWMutex
	runtimeConfig   RuntimeConfig
)

func ConfigureRuntimeConfig(cfg RuntimeConfig) {
	runtimeConfigMu.Lock()
	defer runtimeConfigMu.Unlock()
	runtimeConfig = cfg
}

func CurrentRuntimeConfig() RuntimeConfig {
	runtimeConfigMu.RLock()
	defer runtimeConfigMu.RUnlock()
	return runtimeConfig
}
