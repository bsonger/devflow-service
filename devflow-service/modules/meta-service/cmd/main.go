package main

import (
	"github.com/bsonger/devflow-service/modules/meta-service/pkg/infra/config"
	"github.com/bsonger/devflow-service/modules/meta-service/pkg/router"
	"github.com/bsonger/devflow-service/shared/bootstrap"
	"github.com/bsonger/devflow-service/shared/observability"
)

func main() {
	err := bootstrap.Run(bootstrap.Options[config.Config, router.Options, string]{
		Name: "meta-service",
		RouteOptions: router.Options{
			ServiceName:   "meta-service",
			EnableSwagger: true,
			Modules: []router.Module{
				router.ModuleProject,
				router.ModuleApplication,
				router.ModuleCluster,
				router.ModuleEnvironment,
			},
		},
		Load:        config.Load,
		InitRuntime: config.InitRuntime,
		NewRouter: func(opts router.Options) bootstrap.Runner {
			return router.NewRouterWithOptions(opts)
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
