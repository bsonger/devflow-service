package http

import (
	"time"

	"github.com/bsonger/devflow-service/internal/platform/routercore"
	runtimeservice "github.com/bsonger/devflow-service/internal/runtime/service"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

type Module string

const (
	ModuleRuntimeAPI              Module = "runtime-api"
	ModuleRuntimeObservedWorkload Module = "runtime-observed-workload"
	ModuleRuntimeObservedPod      Module = "runtime-observed-pod"
)

type Options struct {
	ServiceName   string
	EnableSwagger bool
	ObserverToken string
	Modules       []Module
}

func NewRouter() *gin.Engine {
	return NewRouterWithOptions(Options{
		ServiceName:   "runtime-service",
		EnableSwagger: true,
		Modules: []Module{
			ModuleRuntimeAPI,
			ModuleRuntimeObservedWorkload,
			ModuleRuntimeObservedPod,
		},
	})
}

func NewRouterWithOptions(opts Options) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.Use(
		otelgin.Middleware(serviceName(opts), otelgin.WithFilter(routercore.OtelFilter)),
		routercore.LoggerMiddleware(),
		routercore.GinZapRecovery(),
		routercore.PyroscopeMiddleware(),
		routercore.GinMetricsMiddleware(),
		routercore.GinZapLogger(),
		cors.New(cors.Config{
			AllowOrigins:     []string{"*"},
			AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
			AllowHeaders:     []string{"*"},
			ExposeHeaders:    []string{"Content-Length"},
			AllowCredentials: true,
			MaxAge:           12 * time.Hour,
		}),
	)

	routercore.RegisterStatusRoutes(r, routercore.StatusOptions{
		ServiceName:   serviceName(opts),
		EnableSwagger: opts.EnableSwagger,
		Modules:       toStatusModules(opts.Modules),
	})

	if opts.EnableSwagger {
		r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
		r.GET("/api/v1/runtime/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	api := r.Group("/api/v1")
	registerModules(api, opts)
	return r
}

func serviceName(opts Options) string {
	if opts.ServiceName == "" {
		return "runtime-service"
	}
	return opts.ServiceName
}

func registerModules(api *gin.RouterGroup, opts Options) {
	handler := NewHandler(runtimeservice.DefaultService)
	handler.RegisterRoutes(api)
	internal := api.Group("")
	internal.Use(RequireObserverToken(resolveObserverToken(opts.ObserverToken)))
	handler.RegisterInternalRoutes(internal)
}

func toStatusModules(modules []Module) []string {
	if len(modules) == 0 {
		modules = []Module{
			ModuleRuntimeAPI,
			ModuleRuntimeObservedWorkload,
			ModuleRuntimeObservedPod,
		}
	}

	out := make([]string, 0, len(modules))
	for _, module := range modules {
		out = append(out, string(module))
	}
	return out
}
