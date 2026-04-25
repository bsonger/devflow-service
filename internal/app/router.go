package app

import (
	"net/http"
	"time"

	"github.com/bsonger/devflow-service/internal/application"
	"github.com/bsonger/devflow-service/internal/cluster"
	"github.com/bsonger/devflow-service/internal/appservice"
	"github.com/bsonger/devflow-service/internal/approute"
	"github.com/bsonger/devflow-service/internal/appconfig"
	"github.com/bsonger/devflow-service/internal/environment"
	"github.com/bsonger/devflow-service/internal/workloadconfig"
	"github.com/bsonger/devflow-service/internal/platform/routercore"
	"github.com/bsonger/devflow-service/internal/project"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/swaggo/files"
	"github.com/swaggo/gin-swagger"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

type Module string

const (
	ModuleProject     Module = "project"
	ModuleApplication Module = "application"
	ModuleCluster     Module = "cluster"
	ModuleEnvironment Module = "environment"
	ModuleAppService   Module = "app-service"
	ModuleAppRoute     Module = "app-route"
	ModuleAppConfig    Module = "app-config"
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
			ModuleCluster,
			ModuleEnvironment,
			ModuleAppService,
			ModuleAppRoute,
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

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"service": serviceName(opts),
			"status":  "ok",
		})
	})
	r.GET("/readyz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"service": serviceName(opts),
			"status":  "ready",
		})
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
