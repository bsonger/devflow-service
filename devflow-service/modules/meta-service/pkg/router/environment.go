package router

import (
	"github.com/bsonger/devflow-service/modules/meta-service/pkg/api"
	"github.com/gin-gonic/gin"
)

func RegisterEnvironmentRoutes(rg *gin.RouterGroup) {
	environment := rg.Group("/environments")

	environment.GET("", api.EnvironmentRouteApi.List)
	environment.GET("/:id", api.EnvironmentRouteApi.Get)
	environment.POST("", api.EnvironmentRouteApi.Create)
	environment.PUT("/:id", api.EnvironmentRouteApi.Update)
	environment.DELETE("/:id", api.EnvironmentRouteApi.Delete)
}
