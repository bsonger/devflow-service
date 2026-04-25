package http

import (
	"time"

	"github.com/bsonger/devflow-service/internal/approute"
	"github.com/bsonger/devflow-service/internal/appservice"
	"github.com/bsonger/devflow-service/internal/platform/routercore"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

type Module string

const (
	ModuleAppService Module = "app-service"
	ModuleAppRoute   Module = "app-route"
)

type Options struct {
	ServiceName   string
	EnableSwagger bool
	Modules       []Module
}

func NewRouter() *gin.Engine {
	return NewRouterWithOptions(Options{
		ServiceName:   "network-service",
		EnableSwagger: true,
		Modules: []Module{
			ModuleAppService,
			ModuleAppRoute,
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
		r.GET("/api/v1/network/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	api := r.Group("/api/v1")
	registerModules(api, opts)
	return r
}

func serviceName(opts Options) string {
	if opts.ServiceName == "" {
		return "network-service"
	}
	return opts.ServiceName
}

func registerModules(api *gin.RouterGroup, opts Options) {
	seen := make(map[Module]struct{}, len(opts.Modules))
	for _, module := range opts.Modules {
		if _, ok := seen[module]; ok {
			continue
		}
		seen[module] = struct{}{}

		switch module {
		case ModuleAppService:
			appservice.NewModule().RegisterRoutes(api)
		case ModuleAppRoute:
			approute.NewModule().RegisterRoutes(api)
		}
	}
}

func toStatusModules(modules []Module) []string {
	if len(modules) == 0 {
		modules = []Module{
			ModuleAppService,
			ModuleAppRoute,
		}
	}

	out := make([]string, 0, len(modules))
	for _, module := range modules {
		out = append(out, string(module))
	}
	return out
}
