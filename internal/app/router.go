package app

import (
	"time"

	"github.com/bsonger/devflow-service/internal/appconfig"
	"github.com/bsonger/devflow-service/internal/application"
	"github.com/bsonger/devflow-service/internal/applicationenv"
	"github.com/bsonger/devflow-service/internal/approute"
	"github.com/bsonger/devflow-service/internal/appservice"
	"github.com/bsonger/devflow-service/internal/cluster"
	"github.com/bsonger/devflow-service/internal/environment"
	"github.com/bsonger/devflow-service/internal/platform/routercore"
	"github.com/bsonger/devflow-service/internal/project"
	"github.com/bsonger/devflow-service/internal/workloadconfig"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/swaggo/files"
	"github.com/swaggo/gin-swagger"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

type Module string

const (
	ModuleProject        Module = "project"
	ModuleApplication    Module = "application"
	ModuleApplicationEnv Module = "application-environment"
	ModuleCluster        Module = "cluster"
	ModuleEnvironment    Module = "environment"
	ModuleAppService     Module = "app-service"
	ModuleAppRoute       Module = "app-route"
	ModuleAppConfig      Module = "app-config"
	ModuleWorkloadConfig Module = "workload-config"
)

type Options struct {
	ServiceName   string
	EnableSwagger bool
	Modules       []Module
}

func NewRouter() *gin.Engine {
	return NewRouterWithOptions(Options{
		ServiceName:   "devflow",
		EnableSwagger: true,
		Modules: []Module{
			ModuleProject,
			ModuleApplication,
			ModuleApplicationEnv,
			ModuleCluster,
			ModuleEnvironment,
			ModuleAppConfig,
			ModuleWorkloadConfig,
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
		r.GET("/api/v1/app/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	api := r.Group("/api/v1")
	registerModules(api, opts)
	return r
}

func RegisterProjectRoutes(rg *gin.RouterGroup) {
	project.NewModule().RegisterRoutes(rg)
}

func RegisterApplicationCoreRoutes(rg *gin.RouterGroup) {
	application.NewModule().RegisterRoutes(rg)
}

func RegisterApplicationRoutes(rg *gin.RouterGroup) {
	RegisterApplicationCoreRoutes(rg)
}

func RegisterApplicationEnvRoutes(rg *gin.RouterGroup) {
	applicationenv.NewModule().RegisterRoutes(rg)
}

func RegisterClusterRoutes(rg *gin.RouterGroup) {
	cluster.NewModule().RegisterRoutes(rg)
}

func RegisterEnvironmentRoutes(rg *gin.RouterGroup) {
	environment.NewModule().RegisterRoutes(rg)
}

func RegisterAppServiceRoutes(rg *gin.RouterGroup) {
	appservice.NewModule().RegisterRoutes(rg)
}

func RegisterAppRouteRoutes(rg *gin.RouterGroup) {
	approute.NewModule().RegisterRoutes(rg)
}

func RegisterAppConfigRoutes(rg *gin.RouterGroup) {
	appconfig.NewModule().RegisterRoutes(rg)
}

func RegisterWorkloadConfigRoutes(rg *gin.RouterGroup) {
	workloadconfig.NewModule().RegisterRoutes(rg)
}

func serviceName(opts Options) string {
	if opts.ServiceName == "" {
		return "devflow"
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
		case ModuleProject:
			RegisterProjectRoutes(api)
		case ModuleApplication:
			RegisterApplicationRoutes(api)
		case ModuleApplicationEnv:
			RegisterApplicationEnvRoutes(api)
		case ModuleCluster:
			RegisterClusterRoutes(api)
		case ModuleEnvironment:
			RegisterEnvironmentRoutes(api)
		case ModuleAppService:
			RegisterAppServiceRoutes(api)
		case ModuleAppRoute:
			RegisterAppRouteRoutes(api)
		case ModuleAppConfig:
			RegisterAppConfigRoutes(api)
		case ModuleWorkloadConfig:
			RegisterWorkloadConfigRoutes(api)
		}
	}
}

func toStatusModules(modules []Module) []string {
	if len(modules) == 0 {
		modules = []Module{
			ModuleProject,
			ModuleApplication,
			ModuleApplicationEnv,
			ModuleCluster,
			ModuleEnvironment,
			ModuleAppConfig,
			ModuleWorkloadConfig,
		}
	}

	out := make([]string, 0, len(modules))
	for _, module := range modules {
		out = append(out, string(module))
	}
	return out
}
