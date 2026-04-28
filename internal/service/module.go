package serviceapi

import (
	servicepkg "github.com/bsonger/devflow-service/internal/service/service"
	servicehttp "github.com/bsonger/devflow-service/internal/service/transport/http"
	"github.com/gin-gonic/gin"
)

type Module struct {
	handler *servicehttp.Handler
}

func NewModule() Module {
	return Module{
		handler: servicehttp.NewHandler(servicepkg.DefaultServiceService),
	}
}

func (m Module) RegisterRoutes(rg *gin.RouterGroup) {
	m.handler.RegisterRoutes(rg)
}
