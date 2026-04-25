package support

import (
	"sync"

	imagedomain "github.com/bsonger/devflow-service/internal/image/domain"
	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	model "github.com/bsonger/devflow-service/internal/release/domain"
)

type RuntimeConfig struct {
	ImageRegistry           imagedomain.ImageRegistryConfig
	ManifestRegistry        manifestdomain.ManifestRegistryConfig
	ManifestRegistryEnabled bool
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
