// @title DevFlow Config Service API
// @version 1.0
// @description DevFlow config-service HTTP API.
// @BasePath /

package main

import (
	"github.com/bsonger/devflow-service/internal/configservice/transport/http"
	"github.com/bsonger/devflow-service/internal/platform/config"
	"github.com/bsonger/devflow-service/internal/platform/runtime/bootstrap"
	"github.com/bsonger/devflow-service/internal/platform/runtime/observability"
)

func main() {
	err := bootstrap.Run(bootstrap.Options[config.Config, http.Options, string]{
		Name: "config-service",
		RouteOptions: http.Options{
			ServiceName:   "config-service",
			EnableSwagger: true,
			Modules: []http.Module{
				http.ModuleAppConfig,
				http.ModuleWorkloadConfig,
			},
		},
		Load:        config.Load,
		InitRuntime: config.InitRuntime,
		NewRouter: func(opts http.Options) bootstrap.Runner {
			return http.NewRouterWithOptions(opts)
		},
		ResolveConfigPort: func(cfg *config.Config) int {
			if cfg != nil && cfg.Server != nil {
				return cfg.Server.Port
			}
			return 0
		},
		StartMetricsServer: observability.StartMetricsServer,
		StartPprofServer:   observability.StartPprofServer,
		PortEnv:            "CONFIG_SERVICE_PORT",
		DefaultPort:        8082,
		MetricsPortEnv:     "CONFIG_SERVICE_METRICS_PORT",
		PprofPortEnv:       "CONFIG_SERVICE_PPROF_PORT",
	})
	if err != nil {
		panic(err)
	}
}
