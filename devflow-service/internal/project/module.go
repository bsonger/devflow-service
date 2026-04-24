package project

import (
	projectservice "github.com/bsonger/devflow-service/internal/project/service"
	projecthttp "github.com/bsonger/devflow-service/internal/project/transport/http"
	"github.com/gin-gonic/gin"
)

type Module struct {
	handler *projecthttp.Handler
}

func NewModule() Module {
	return Module{
		handler: projecthttp.NewHandler(projectservice.DefaultService),
	}
}

func (m Module) RegisterRoutes(rg *gin.RouterGroup) {
	m.handler.RegisterRoutes(rg)
}
