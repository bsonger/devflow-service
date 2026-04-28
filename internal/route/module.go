package routeapi

import (
	routepkg "github.com/bsonger/devflow-service/internal/route/service"
	routehttp "github.com/bsonger/devflow-service/internal/route/transport/http"
	"github.com/gin-gonic/gin"
)

type Module struct {
	handler *routehttp.Handler
}

func NewModule() Module {
	return Module{
		handler: routehttp.NewHandler(routepkg.DefaultRouteService),
	}
}

func (m Module) RegisterRoutes(rg *gin.RouterGroup) {
	m.handler.RegisterRoutes(rg)
}
