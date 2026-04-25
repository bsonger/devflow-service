// @title DevFlow Network Service API
// @version 1.0
// @description DevFlow network-service HTTP API.
// @BasePath /

package main

import (
	networkhttp "github.com/bsonger/devflow-service/internal/networkservice/transport/http"
	platformconfig "github.com/bsonger/devflow-service/internal/platform/config"
	"github.com/bsonger/devflow-service/internal/platform/runtime/bootstrap"
	"github.com/bsonger/devflow-service/internal/platform/runtime/observability"
)

func main() {
	err := bootstrap.Run(bootstrap.Options[platformconfig.Config, networkhttp.Options, string]{
		Name: "network-service",
		RouteOptions: networkhttp.Options{
			ServiceName:   "network-service",
			EnableSwagger: true,
			Modules: []networkhttp.Module{
				networkhttp.ModuleAppService,
				networkhttp.ModuleAppRoute,
			},
		},
		Load:        platformconfig.Load,
		InitRuntime: platformconfig.InitRuntime,
		NewRouter: func(opts networkhttp.Options) bootstrap.Runner {
			return networkhttp.NewRouterWithOptions(opts)
		},
		ResolveConfigPort: func(cfg *platformconfig.Config) int {
			if cfg != nil && cfg.Server != nil {
				return cfg.Server.Port
			}
			return 0
		},
		StartMetricsServer: observability.StartMetricsServer,
		StartPprofServer:   observability.StartPprofServer,
		PortEnv:            "NETWORK_SERVICE_PORT",
		DefaultPort:        8086,
		MetricsPortEnv:     "NETWORK_SERVICE_METRICS_PORT",
		PprofPortEnv:       "NETWORK_SERVICE_PPROF_PORT",
	})
	if err != nil {
		panic(err)
	}
}
