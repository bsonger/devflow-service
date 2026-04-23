package router

import (
	"github.com/bsonger/devflow-service/modules/meta-service/pkg/api"
	"github.com/gin-gonic/gin"
)

func RegisterClusterRoutes(rg *gin.RouterGroup) {
	cluster := rg.Group("/clusters")

	cluster.GET("", api.ClusterRouteApi.List)
	cluster.GET("/:id", api.ClusterRouteApi.Get)
	cluster.POST("", api.ClusterRouteApi.Create)
	cluster.PUT("/:id", api.ClusterRouteApi.Update)
	cluster.DELETE("/:id", api.ClusterRouteApi.Delete)
}
