// @title DevFlow Release Service API
// @version 1.0
// @description DevFlow release-service HTTP API.
// @BasePath /

package main

import (
	"github.com/bsonger/devflow-service/internal/platform/runtime/bootstrap"
	"github.com/bsonger/devflow-service/internal/platform/runtime/observability"
	"github.com/bsonger/devflow-service/internal/release/config"
	"github.com/bsonger/devflow-service/internal/release/runtime"
	releasehttp "github.com/bsonger/devflow-service/internal/release/transport/http"
)

func main() {
	err := bootstrap.Run(bootstrap.Options[config.Config, releasehttp.Options, runtime.ExecutionMode]{
		Name:        "release-service",
		Load:        config.Load,
		InitRuntime: config.InitRuntime,
		NewRouter: func(opts releasehttp.Options) bootstrap.Runner {
			return releasehttp.NewRouterWithOptions(opts)
		},
		SetExecutionMode: runtime.SetExecutionMode,
		ResolveConfigPort: func(cfg *config.Config) int {
			if cfg != nil && cfg.Server != nil {
				return cfg.Server.Port
			}
			return 0
		},
		StartMetricsServer: observability.StartMetricsServer,
		StartPprofServer:   observability.StartPprofServer,
		ExecutionMode:      runtime.ExecutionModeDirect,
		RouteOptions: releasehttp.Options{
			ServiceName:   "release-service",
			EnableSwagger: true,
			Modules: []releasehttp.Module{
				releasehttp.ModuleManifest,
				releasehttp.ModuleImage,
				releasehttp.ModuleRelease,
				releasehttp.ModuleIntent,
			},
		},
		PortEnv:        "RELEASE_SERVICE_PORT",
		DefaultPort:    8083,
		MetricsPortEnv: "RELEASE_SERVICE_METRICS_PORT",
		PprofPortEnv:   "RELEASE_SERVICE_PPROF_PORT",
	})
	if err != nil {
		panic(err)
	}
}
