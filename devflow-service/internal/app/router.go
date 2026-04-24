package app

import (
	"net/http"
	"time"

	applicationmodule "github.com/bsonger/devflow-service/internal/application"
	clustermodule "github.com/bsonger/devflow-service/internal/cluster"
	environmentmodule "github.com/bsonger/devflow-service/internal/environment"
	routercore "github.com/bsonger/devflow-service/internal/platform/routercore"
	projectmodule "github.com/bsonger/devflow-service/internal/project"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

type Module string

const (
	ModuleProject     Module = "project"
	ModuleApplication Module = "application"
	ModuleCluster     Module = "cluster"
	ModuleEnvironment Module = "environment"
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
	projectmodule.NewModule().RegisterRoutes(rg)
}

func RegisterApplicationCoreRoutes(rg *gin.RouterGroup) {
	applicationmodule.NewModule().RegisterRoutes(rg)
}

func RegisterApplicationRoutes(rg *gin.RouterGroup) {
	RegisterApplicationCoreRoutes(rg)
}

func RegisterClusterRoutes(rg *gin.RouterGroup) {
	clustermodule.NewModule().RegisterRoutes(rg)
}

func RegisterEnvironmentRoutes(rg *gin.RouterGroup) {
	environmentmodule.NewModule().RegisterRoutes(rg)
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
		}
	}
}
