package router

import (
	"github.com/bsonger/devflow-service/modules/meta-service/pkg/api"
	"github.com/gin-gonic/gin"
)

func RegisterProjectRoutes(rg *gin.RouterGroup) {
	project := rg.Group("/projects")

	project.GET("", api.ProjectRouteApi.List)
	project.GET("/:id", api.ProjectRouteApi.Get)
	project.POST("", api.ProjectRouteApi.Create)
	project.PUT("/:id", api.ProjectRouteApi.Update)
	project.DELETE("/:id", api.ProjectRouteApi.Delete)
	project.GET("/:id/applications", api.ProjectRouteApi.ListApplications)
}
