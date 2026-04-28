package application

import (
	applicationservice "github.com/bsonger/devflow-service/internal/application/service"
	apphttp "github.com/bsonger/devflow-service/internal/application/transport/http"
	"github.com/gin-gonic/gin"
)

type Module struct {
	handler *apphttp.Handler
}

func NewModule() Module {
	return Module{
		handler: apphttp.NewHandler(applicationservice.DefaultService),
	}
}

func (m Module) RegisterRoutes(rg *gin.RouterGroup) {
	m.handler.RegisterRoutes(rg)
}
