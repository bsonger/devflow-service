package http

import (
	"github.com/bsonger/devflow-service/internal/release"
	"github.com/bsonger/devflow-service/internal/platform/routercore"
	"github.com/bsonger/devflow-service/internal/release/service"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"net/http"
	"time"
)

type Module string

const (
	ModuleManifest Module = "manifest"
	ModuleImage    Module = "image"
	ModuleRelease  Module = "release"
	ModuleIntent   Module = "intent"
)

type Options struct {
	ServiceName   string
	EnableSwagger bool
	Modules       []Module
}

// NewRouter creates the main Gin router.
func NewRouter() *gin.Engine {
	return NewRouterWithOptions(Options{
		ServiceName:   "release-service",
		EnableSwagger: true,
		Modules: []Module{
			ModuleManifest,
			ModuleImage,
			ModuleRelease,
			ModuleIntent,
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
		r.GET("/api/v1/release/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	api := r.Group("/api/v1")

	registerModules(api, opts)
	return r
}

func serviceName(opts Options) string {
	if opts.ServiceName == "" {
		return "release-service"
	}
	return opts.ServiceName
}

func registerModules(api *gin.RouterGroup, opts Options) {
	release.NewModule().RegisterRoutes(api)
	NewReleaseHandler(service.ReleaseService).RegisterRoutes(api)
	RegisterReleaseWritebackRoutes(api)
}
