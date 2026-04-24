// @title DevFlow Meta Service API
// @version 1.0
// @description DevFlow meta-service HTTP API.
// @BasePath /

package main

import (
	"github.com/bsonger/devflow-service/internal/app"
	"github.com/bsonger/devflow-service/internal/platform/config"
	"github.com/bsonger/devflow-service/internal/platform/runtime/bootstrap"
	"github.com/bsonger/devflow-service/internal/platform/runtime/observability"
)

func main() {
	err := bootstrap.Run(bootstrap.Options[config.Config, app.Options, string]{
		Name: "meta-service",
		RouteOptions: app.Options{
			ServiceName:   "meta-service",
			EnableSwagger: true,
			Modules: []app.Module{
				app.ModuleProject,
				app.ModuleApplication,
				app.ModuleCluster,
				app.ModuleEnvironment,
				app.ModuleNetwork,
				app.ModuleConfig,
			},
		},
		Load:        config.Load,
		InitRuntime: config.InitRuntime,
		NewRouter: func(opts app.Options) bootstrap.Runner {
			return app.NewRouterWithOptions(opts)
		},
		ResolveConfigPort: func(cfg *config.Config) int {
			if cfg != nil && cfg.Server != nil {
				return cfg.Server.Port
			}
			return 0
		},
		StartMetricsServer: observability.StartMetricsServer,
		StartPprofServer:   observability.StartPprofServer,
		PortEnv:            "META_SERVICE_PORT",
		DefaultPort:        8081,
		MetricsPortEnv:     "META_SERVICE_METRICS_PORT",
		PprofPortEnv:       "META_SERVICE_PPROF_PORT",
	})
	if err != nil {
		panic(err)
	}
}
