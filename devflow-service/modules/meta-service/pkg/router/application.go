package router

import (
	"github.com/bsonger/devflow-service/modules/meta-service/pkg/api"
	"github.com/gin-gonic/gin"
)

func registerApplicationGroup(rg *gin.RouterGroup) *gin.RouterGroup {
	app := rg.Group("/applications")

	app.GET("", api.ApplicationRouteApi.List)
	app.GET("/:id", api.ApplicationRouteApi.Get)
	app.POST("", api.ApplicationRouteApi.Create)
	app.PUT("/:id", api.ApplicationRouteApi.Update)
	app.DELETE("/:id", api.ApplicationRouteApi.Delete)
	app.PATCH("/:id/active_image", api.ApplicationRouteApi.UpdateActiveImage)

	return app
}

func RegisterApplicationCoreRoutes(rg *gin.RouterGroup) {
	registerApplicationGroup(rg)
}

func RegisterApplicationRoutes(rg *gin.RouterGroup) {
	RegisterApplicationCoreRoutes(rg)
}
