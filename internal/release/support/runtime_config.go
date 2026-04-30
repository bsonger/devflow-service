package support

import (
	"sync"

	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	"github.com/bsonger/devflow-service/internal/platform/oci"
	model "github.com/bsonger/devflow-service/internal/release/domain"
)

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
	Downstream            model.DownstreamConfig
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
