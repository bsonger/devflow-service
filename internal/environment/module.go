package environment

import (
	"github.com/bsonger/devflow-service/internal/environment/service"
	"github.com/bsonger/devflow-service/internal/environment/transport/http"
	"github.com/gin-gonic/gin"
)

type Module struct {
	handler *http.Handler
}

func NewModule() Module {
	return Module{
		handler: http.NewHandler(service.DefaultService),
	}
}

func (m Module) RegisterRoutes(rg *gin.RouterGroup) {
	m.handler.RegisterRoutes(rg)
}
