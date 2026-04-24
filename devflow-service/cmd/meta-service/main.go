package main

import (
	serviceapp "github.com/bsonger/devflow-service/internal/app"
	platformconfig "github.com/bsonger/devflow-service/internal/platform/config"
	platformbootstrap "github.com/bsonger/devflow-service/internal/platform/runtime/bootstrap"
	platformobservability "github.com/bsonger/devflow-service/internal/platform/runtime/observability"
)

func main() {
	err := platformbootstrap.Run(platformbootstrap.Options[platformconfig.Config, serviceapp.Options, string]{
		Name: "meta-service",
		RouteOptions: serviceapp.Options{
			ServiceName:   "meta-service",
			EnableSwagger: true,
			Modules: []serviceapp.Module{
				serviceapp.ModuleProject,
				serviceapp.ModuleApplication,
				serviceapp.ModuleCluster,
				serviceapp.ModuleEnvironment,
			},
		},
		Load:        platformconfig.Load,
		InitRuntime: platformconfig.InitRuntime,
		NewRouter: func(opts serviceapp.Options) platformbootstrap.Runner {
			return serviceapp.NewRouterWithOptions(opts)
		},
		ResolveConfigPort: func(cfg *platformconfig.Config) int {
			if cfg != nil && cfg.Server != nil {
				return cfg.Server.Port
			}
			return 0
		},
		StartMetricsServer: platformobservability.StartMetricsServer,
		StartPprofServer:   platformobservability.StartPprofServer,
		PortEnv:            "META_SERVICE_PORT",
		DefaultPort:        8081,
		MetricsPortEnv:     "META_SERVICE_METRICS_PORT",
		PprofPortEnv:       "META_SERVICE_PPROF_PORT",
	})
	if err != nil {
		panic(err)
	}
}
