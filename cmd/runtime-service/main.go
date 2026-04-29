// @title DevFlow Runtime Service API
// @version 1.0
// @description DevFlow runtime-service HTTP API.
// @BasePath /

package main

import (
	platformconfig "github.com/bsonger/devflow-service/internal/platform/config"
	"github.com/bsonger/devflow-service/internal/platform/runtime/bootstrap"
	"github.com/bsonger/devflow-service/internal/platform/runtime/observability"
	runtimehttp "github.com/bsonger/devflow-service/internal/runtime/transport/http"
)

func main() {
	err := bootstrap.Run(bootstrap.Options[platformconfig.Config, runtimehttp.Options, string]{
		Name: "runtime-service",
		RouteOptions: runtimehttp.Options{
			ServiceName:   "runtime-service",
			EnableSwagger: true,
			Modules: []runtimehttp.Module{
				runtimehttp.ModuleRuntimeAPI,
				runtimehttp.ModuleRuntimeObservedWorkload,
				runtimehttp.ModuleRuntimeObservedPod,
			},
		},
		Load:        platformconfig.Load,
		InitRuntime: platformconfig.InitRuntime,
		NewRouter: func(opts runtimehttp.Options) bootstrap.Runner {
			return runtimehttp.NewRouterWithOptions(opts)
		},
		ResolveConfigPort: func(cfg *platformconfig.Config) int {
			if cfg != nil && cfg.Server != nil {
				return cfg.Server.Port
			}
			return 0
		},
		StartMetricsServer: observability.StartMetricsServer,
		StartPprofServer:   observability.StartPprofServer,
		PortEnv:            "RUNTIME_SERVICE_PORT",
		DefaultPort:        8084,
		MetricsPortEnv:     "RUNTIME_SERVICE_METRICS_PORT",
		PprofPortEnv:       "RUNTIME_SERVICE_PPROF_PORT",
	})
	if err != nil {
		panic(err)
	}
}
