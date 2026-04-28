package support

import (
	"sync"

	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	"github.com/bsonger/devflow-service/internal/platform/oci"
	model "github.com/bsonger/devflow-service/internal/release/domain"
)

type RuntimeConfig struct {
	ImageRegistry           oci.ImageRegistryConfig
	ManifestRegistry        manifestdomain.ManifestRegistryConfig
	ManifestRegistryEnabled bool
	ManifestPublisherMode   string
	Downstream              model.DownstreamConfig
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
